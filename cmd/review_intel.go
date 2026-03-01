package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/sanix-darker/prev/internal/diffparse"
)

type symbolImpact struct {
	Symbol      string
	References  int
	ChangedHits int
	Files       map[string]int
}

var (
	reWordIdent = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
	reCallIdent = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	goDeclRe    = regexp.MustCompile(`^\s*func\s+(?:\([^)]+\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	phpDeclRe   = regexp.MustCompile(`^\s*function\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
)

func normalizeNativeImpact(enabled bool, maxSymbols int) (bool, int) {
	if !enabled {
		return false, 0
	}
	if maxSymbols <= 0 {
		maxSymbols = 12
	}
	if maxSymbols > 40 {
		maxSymbols = 40
	}
	return true, maxSymbols
}

func appendNativeImpactGuidelines(
	guidelines string,
	changes []diffparse.FileChange,
	repoPath string,
	enabled bool,
	maxSymbols int,
) string {
	enabled, maxSymbols = normalizeNativeImpact(enabled, maxSymbols)
	if !enabled {
		return guidelines
	}
	report := buildNativeImpactReport(changes, repoPath, maxSymbols)
	if strings.TrimSpace(report) == "" {
		return guidelines
	}
	if strings.TrimSpace(guidelines) == "" {
		return report
	}
	return guidelines + "\n" + report
}

func buildNativeImpactReport(changes []diffparse.FileChange, repoPath string, maxSymbols int) string {
	symbols := extractChangedSymbols(changes, maxSymbols)
	risks := detectNativeConcurrencySignals(changes)
	if len(symbols) == 0 && len(risks) == 0 {
		return ""
	}

	changedPaths := changedPathSet(changes)
	impact := map[string]symbolImpact{}
	if repoPath != "" && len(symbols) > 0 {
		impact = scanSymbolImpact(repoPath, symbols, changedPaths)
	}

	lines := []string{"Native impact precheck (deterministic):"}
	if len(symbols) > 0 {
		lines = append(lines, "Changed-symbol impact map:")
		for _, s := range symbols {
			if im, ok := impact[s]; ok {
				top := topImpactFiles(im.Files, 3)
				lines = append(lines, fmt.Sprintf("- `%s`: refs=%d changed_refs=%d top_files=%s", s, im.References, im.ChangedHits, top))
				continue
			}
			lines = append(lines, fmt.Sprintf("- `%s`: refs=unknown (repo scan unavailable)", s))
		}
	}
	if len(risks) > 0 {
		lines = append(lines, "Concurrency/race-risk signals from changed hunks:")
		for _, r := range risks {
			lines = append(lines, "- "+r)
		}
		lines = append(lines, "Treat these as hypotheses; confirm with precise code evidence before reporting.")
	}
	lines = append(lines, "Prioritize high fan-out symbols and unresolved concurrency signals.")
	return strings.Join(lines, "\n")
}

func extractChangedSymbols(changes []diffparse.FileChange, maxSymbols int) []string {
	seen := map[string]struct{}{}
	add := func(sym string) {
		sym = strings.TrimSpace(sym)
		if sym == "" || len(sym) < 3 {
			return
		}
		if isNoiseSymbol(sym) {
			return
		}
		seen[sym] = struct{}{}
	}

	for _, c := range changes {
		for _, h := range c.Hunks {
			for _, l := range h.Lines {
				if l.Type != diffparse.LineAdded {
					continue
				}
				content := strings.TrimSpace(l.Content)
				if content == "" {
					continue
				}
				if m := goDeclRe.FindStringSubmatch(content); len(m) == 2 {
					add(m[1])
				}
				if m := phpDeclRe.FindStringSubmatch(content); len(m) == 2 {
					add(m[1])
				}
				for _, m := range reCallIdent.FindAllStringSubmatch(content, -1) {
					if len(m) == 2 {
						add(m[1])
					}
				}
			}
		}
	}

	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	if len(out) > maxSymbols {
		out = out[:maxSymbols]
	}
	return out
}

func isNoiseSymbol(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "if", "for", "switch", "select", "return", "append", "make", "new",
		"len", "cap", "copy", "close", "panic", "recover", "error", "string",
		"int", "bool", "map", "chan", "func", "echo", "json_encode", "json_decode":
		return true
	default:
		return false
	}
}

func changedPathSet(changes []diffparse.FileChange) map[string]struct{} {
	out := make(map[string]struct{}, len(changes))
	for _, c := range changes {
		if n := strings.TrimSpace(c.NewName); n != "" {
			out[n] = struct{}{}
		}
		if o := strings.TrimSpace(c.OldName); o != "" {
			out[o] = struct{}{}
		}
	}
	return out
}

func scanSymbolImpact(repoPath string, symbols []string, changedPaths map[string]struct{}) map[string]symbolImpact {
	const maxFiles = 900
	imp := make(map[string]symbolImpact, len(symbols))
	for _, s := range symbols {
		imp[s] = symbolImpact{
			Symbol: s,
			Files:  map[string]int{},
		}
	}

	scanned := 0
	_ = filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			switch name {
			case ".git", "node_modules", "vendor", ".prev", "dist", "build", ".idea", ".vscode":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		if scanned >= maxFiles {
			return filepath.SkipDir
		}
		rel, rerr := filepath.Rel(repoPath, path)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !isImpactTextFile(rel) {
			return nil
		}
		scanned++
		counts := scanSymbolCountsInFile(path, symbols)
		if len(counts) == 0 {
			return nil
		}
		for sym, cnt := range counts {
			entry := imp[sym]
			entry.References += cnt
			entry.Files[rel] += cnt
			if _, ok := changedPaths[rel]; ok {
				entry.ChangedHits += cnt
			}
			imp[sym] = entry
		}
		return nil
	})
	return imp
}

func isImpactTextFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".php", ".py", ".js", ".ts", ".tsx", ".jsx", ".java", ".rb", ".rs", ".cs", ".c", ".cc", ".cpp", ".h", ".hpp", ".yaml", ".yml", ".json":
		return true
	default:
		return false
	}
}

func scanSymbolCountsInFile(path string, symbols []string) map[string]int {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	counts := map[string]int{}
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		idents := reWordIdent.FindAllString(line, -1)
		if len(idents) == 0 {
			continue
		}
		local := map[string]int{}
		for _, id := range idents {
			local[id]++
		}
		for _, s := range symbols {
			if c := local[s]; c > 0 {
				counts[s] += c
			}
		}
	}
	if len(counts) == 0 {
		return nil
	}
	return counts
}

func topImpactFiles(files map[string]int, limit int) string {
	type kv struct {
		k string
		v int
	}
	arr := make([]kv, 0, len(files))
	for k, v := range files {
		arr = append(arr, kv{k: k, v: v})
	}
	sort.Slice(arr, func(i, j int) bool {
		if arr[i].v != arr[j].v {
			return arr[i].v > arr[j].v
		}
		return arr[i].k < arr[j].k
	})
	if len(arr) > limit {
		arr = arr[:limit]
	}
	parts := make([]string, 0, len(arr))
	for _, it := range arr {
		parts = append(parts, fmt.Sprintf("%s(%d)", it.k, it.v))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

func detectNativeConcurrencySignals(changes []diffparse.FileChange) []string {
	seen := map[string]struct{}{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		seen[s] = struct{}{}
	}

	for _, c := range changes {
		isGo := strings.HasSuffix(strings.ToLower(c.NewName), ".go") || strings.HasSuffix(strings.ToLower(c.OldName), ".go")
		if !isGo {
			continue
		}
		for _, h := range c.Hunks {
			hunkText := ""
			lockAdds := 0
			unlockAdds := 0
			for _, l := range h.Lines {
				if l.Type != diffparse.LineAdded {
					continue
				}
				line := strings.TrimSpace(l.Content)
				hunkText += "\n" + line
				if strings.Contains(line, "go ") || strings.Contains(line, "go\t") {
					add(fmt.Sprintf("%s:%d introduces goroutine execution; verify shared-state synchronization and context cancellation.", changeFileName(c), l.NewLineNo))
				}
				if strings.Contains(line, ".Lock()") {
					lockAdds++
				}
				if strings.Contains(line, ".Unlock()") {
					unlockAdds++
				}
				if strings.Contains(line, "<-") {
					add(fmt.Sprintf("%s:%d changes channel flow; verify send/receive pairing and cancellation paths.", changeFileName(c), l.NewLineNo))
				}
				if strings.Contains(line, "map[") && strings.Contains(line, "=") && !strings.Contains(line, ":=") {
					add(fmt.Sprintf("%s:%d may mutate map state; verify synchronization when shared across goroutines.", changeFileName(c), l.NewLineNo))
				}
			}
			if lockAdds > unlockAdds {
				add(fmt.Sprintf("%s hunk +%d..%d adds %d lock(s) but %d unlock(s); verify lock/unlock symmetry.", changeFileName(c), h.NewStart, h.NewStart+h.NewLines-1, lockAdds, unlockAdds))
			}
			if strings.Contains(hunkText, "sync.WaitGroup") && !strings.Contains(hunkText, ".Done()") {
				add(fmt.Sprintf("%s hunk +%d..%d touches WaitGroup without .Done() in added lines; verify completion signaling.", changeFileName(c), h.NewStart, h.NewStart+h.NewLines-1))
			}
		}
	}

	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

func changeFileName(c diffparse.FileChange) string {
	if strings.TrimSpace(c.NewName) != "" {
		return c.NewName
	}
	return c.OldName
}
