package diffparse

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/sanix-darker/prev/internal/core"
	"github.com/sanix-darker/prev/internal/serena"
)

// EnrichedFileChange is a FileChange augmented with surrounding code context.
type EnrichedFileChange struct {
	FileChange
	Language       string
	FullNewContent string
	EnrichedHunks  []EnrichedHunk
	TokenEstimate  int
}

// EnrichedHunk is a Hunk with surrounding context lines.
type EnrichedHunk struct {
	Hunk
	ContextBefore []string
	ContextAfter  []string
	StartLine     int
	EndLine       int
}

var languageMap = map[string]string{
	".go":    "go",
	".py":    "python",
	".js":    "javascript",
	".ts":    "typescript",
	".tsx":   "tsx",
	".jsx":   "jsx",
	".rb":    "ruby",
	".rs":    "rust",
	".java":  "java",
	".c":     "c",
	".cpp":   "cpp",
	".h":     "c",
	".hpp":   "cpp",
	".cs":    "csharp",
	".php":   "php",
	".swift": "swift",
	".kt":    "kotlin",
	".scala": "scala",
	".sh":    "bash",
	".bash":  "bash",
	".zsh":   "zsh",
	".yaml":  "yaml",
	".yml":   "yaml",
	".json":  "json",
	".toml":  "toml",
	".xml":   "xml",
	".html":  "html",
	".css":   "css",
	".scss":  "scss",
	".sql":   "sql",
	".md":    "markdown",
	".r":     "r",
	".lua":   "lua",
	".zig":   "zig",
	".ex":    "elixir",
	".exs":   "elixir",
	".erl":   "erlang",
	".hs":    "haskell",
	".ml":    "ocaml",
	".vim":   "vim",
}

// DetectLanguage returns the language name based on file extension.
func DetectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	if lang, ok := languageMap[ext]; ok {
		return lang
	}

	// Check special filenames
	base := strings.ToLower(filepath.Base(filePath))
	switch base {
	case "dockerfile":
		return "dockerfile"
	case "makefile":
		return "makefile"
	case "cmakelists.txt":
		return "cmake"
	}

	return ""
}

// EnrichFileChanges takes parsed file changes and adds surrounding code context.
// serenaClient can be nil (disabled). contextLines defaults to 10 if <= 0.
// maxBatchTokens is the budget; if exceeded with Serena unavailable, contextLines is reduced.
func EnrichFileChanges(
	changes []FileChange,
	repoPath, baseBranch, targetBranch string,
	contextLines int,
	maxBatchTokens int,
	serenaClient *serena.Client,
) ([]EnrichedFileChange, error) {
	if contextLines <= 0 {
		contextLines = 10
	}
	if maxBatchTokens <= 0 {
		maxBatchTokens = 80000
	}

	enriched := make([]EnrichedFileChange, 0, len(changes))

	for _, fc := range changes {
		efc := EnrichedFileChange{
			FileChange: fc,
		}

		name := fc.NewName
		if name == "" {
			name = fc.OldName
		}
		efc.Language = DetectLanguage(name)

		if fc.IsBinary || fc.IsDeleted {
			efc.TokenEstimate = 100
			enriched = append(enriched, efc)
			continue
		}

		// Get full file content from target branch
		content, err := core.GetFileContent(repoPath, targetBranch, name)
		if err != nil {
			// Non-fatal: keep raw hunks so review context remains actionable.
			efc.EnrichedHunks = fallbackEnrichedHunks(fc.Hunks)
			formatted := FormatEnrichedForReview(efc)
			efc.TokenEstimate = len(formatted) / 4
			enriched = append(enriched, efc)
			continue
		}
		efc.FullNewContent = content

		var newLines []string
		if content != "" {
			newLines = strings.Split(content, "\n")
		}

		// Enrich hunks with context
		efc.EnrichedHunks = enrichHunks(fc.Hunks, newLines, contextLines)
		if len(efc.EnrichedHunks) == 0 && len(fc.Hunks) > 0 {
			efc.EnrichedHunks = fallbackEnrichedHunks(fc.Hunks)
		}

		// Estimate tokens from formatted output
		formatted := FormatEnrichedForReview(efc)
		efc.TokenEstimate = len(formatted) / 4

		enriched = append(enriched, efc)
	}

	// Check total token estimate
	totalTokens := 0
	for _, efc := range enriched {
		totalTokens += efc.TokenEstimate
	}

	// If over budget, try Serena or reduce context
	if totalTokens > maxBatchTokens {
		if serenaClient != nil {
			enriched = enrichWithSerena(enriched, serenaClient, repoPath)
		} else if contextLines > 3 {
			// Reduce context and re-enrich
			return EnrichFileChanges(changes, repoPath, baseBranch, targetBranch, 3, maxBatchTokens, nil)
		}
	}

	return enriched, nil
}

func fallbackEnrichedHunks(hunks []Hunk) []EnrichedHunk {
	if len(hunks) == 0 {
		return nil
	}
	out := make([]EnrichedHunk, 0, len(hunks))
	for _, h := range hunks {
		start := h.NewStart
		if start <= 0 {
			start = 1
		}
		end := h.NewStart + h.NewLines - 1
		if end < start {
			end = start
		}
		out = append(out, EnrichedHunk{
			Hunk:      h,
			StartLine: start,
			EndLine:   end,
		})
	}
	return out
}

// enrichHunks adds context lines to hunks and merges overlapping ones.
func enrichHunks(hunks []Hunk, newLines []string, contextLines int) []EnrichedHunk {
	if len(hunks) == 0 || len(newLines) == 0 {
		return nil
	}

	// Sort hunks by NewStart
	sorted := make([]Hunk, len(hunks))
	copy(sorted, hunks)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].NewStart < sorted[j].NewStart
	})

	// Build enriched hunks with context ranges
	type hunkRange struct {
		hunk     Hunk
		ctxStart int // 1-based
		ctxEnd   int // 1-based
	}

	var ranges []hunkRange
	for _, h := range sorted {
		hunkEnd := h.NewStart + h.NewLines - 1
		if hunkEnd < h.NewStart {
			hunkEnd = h.NewStart
		}

		ctxStart := h.NewStart - contextLines
		if ctxStart < 1 {
			ctxStart = 1
		}
		ctxEnd := hunkEnd + contextLines
		if ctxEnd > len(newLines) {
			ctxEnd = len(newLines)
		}

		ranges = append(ranges, hunkRange{hunk: h, ctxStart: ctxStart, ctxEnd: ctxEnd})
	}

	// Merge overlapping ranges
	merged := []hunkRange{ranges[0]}
	for i := 1; i < len(ranges); i++ {
		last := &merged[len(merged)-1]
		if ranges[i].ctxStart <= last.ctxEnd+1 {
			// Merge: extend the end and combine hunk lines
			if ranges[i].ctxEnd > last.ctxEnd {
				last.ctxEnd = ranges[i].ctxEnd
			}
			last.hunk.Lines = append(last.hunk.Lines, ranges[i].hunk.Lines...)
			last.hunk.NewLines += ranges[i].hunk.NewLines
			last.hunk.OldLines += ranges[i].hunk.OldLines
		} else {
			merged = append(merged, ranges[i])
		}
	}

	// Build enriched hunks
	var result []EnrichedHunk
	for _, r := range merged {
		eh := EnrichedHunk{
			Hunk:      r.hunk,
			StartLine: r.ctxStart,
			EndLine:   r.ctxEnd,
		}

		// Context before the hunk
		beforeEnd := r.hunk.NewStart - 1
		if beforeEnd >= r.ctxStart && r.ctxStart >= 1 {
			for i := r.ctxStart; i <= beforeEnd && i <= len(newLines); i++ {
				eh.ContextBefore = append(eh.ContextBefore, newLines[i-1])
			}
		}

		// Context after the hunk
		hunkEnd := r.hunk.NewStart + r.hunk.NewLines - 1
		if hunkEnd < r.hunk.NewStart {
			hunkEnd = r.hunk.NewStart
		}
		afterStart := hunkEnd + 1
		if afterStart <= r.ctxEnd && afterStart >= 1 {
			for i := afterStart; i <= r.ctxEnd && i <= len(newLines); i++ {
				eh.ContextAfter = append(eh.ContextAfter, newLines[i-1])
			}
		}

		result = append(result, eh)
	}

	return result
}

// enrichWithSerena replaces raw context with Serena's symbol-level context.
func enrichWithSerena(enriched []EnrichedFileChange, client *serena.Client, repoPath string) []EnrichedFileChange {
	for i := range enriched {
		efc := &enriched[i]
		if efc.IsBinary || efc.IsDeleted {
			continue
		}

		name := efc.NewName
		if name == "" {
			name = efc.OldName
		}

		for j := range efc.EnrichedHunks {
			eh := &efc.EnrichedHunks[j]
			symbol, err := client.FindEnclosingSymbol(
				filepath.Join(repoPath, name),
				eh.Hunk.NewStart,
			)
			if err != nil || symbol == nil {
				continue
			}

			// Replace context with symbol content
			symbolLines := strings.Split(symbol.Content, "\n")
			eh.ContextBefore = symbolLines
			eh.ContextAfter = nil
			eh.StartLine = symbol.StartLine
			eh.EndLine = symbol.EndLine
		}

		// Recalculate token estimate
		formatted := FormatEnrichedForReview(*efc)
		efc.TokenEstimate = len(formatted) / 4
	}

	return enriched
}

// FormatEnrichedForReview formats an enriched file change for AI review.
func FormatEnrichedForReview(efc EnrichedFileChange) string {
	var sb strings.Builder

	name := efc.NewName
	if name == "" {
		name = efc.OldName
	}

	label := "Modified"
	if efc.IsNew {
		label = "New"
	} else if efc.IsDeleted {
		label = "Deleted"
	} else if efc.IsRenamed {
		label = fmt.Sprintf("Renamed from %s", efc.OldName)
	}

	langTag := ""
	if efc.Language != "" {
		langTag = fmt.Sprintf(" [%s]", efc.Language)
	}

	sb.WriteString(fmt.Sprintf("## File: %s (%s) [+%d/-%d]%s\n\n",
		name, label, efc.Stats.Additions, efc.Stats.Deletions, langTag))

	if efc.IsBinary {
		sb.WriteString("Binary file\n\n")
		return sb.String()
	}

	for _, eh := range efc.EnrichedHunks {
		sb.WriteString(fmt.Sprintf("### Lines %d-%d:\n", eh.StartLine, eh.EndLine))
		sb.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n",
			eh.Hunk.OldStart, eh.Hunk.OldLines, eh.Hunk.NewStart, eh.Hunk.NewLines))

		langFence := efc.Language
		if langFence == "" {
			langFence = "diff"
		}
		sb.WriteString(fmt.Sprintf("```%s\n", langFence))

		// Context before
		for i, line := range eh.ContextBefore {
			ctxLine := eh.StartLine + i
			if ctxLine >= eh.Hunk.NewStart {
				break
			}
			sb.WriteString(fmt.Sprintf("  %s | %s\n", fmtLineNo(ctxLine), line))
		}

		// Diff lines
		for _, dl := range eh.Hunk.Lines {
			switch dl.Type {
			case LineAdded:
				sb.WriteString(fmt.Sprintf("+ %s | %s\n", fmtLineNo(dl.NewLineNo), dl.Content))
			case LineDeleted:
				sb.WriteString(fmt.Sprintf("- %s | %s\n", fmtLineNo(dl.OldLineNo), dl.Content))
			default:
				sb.WriteString(fmt.Sprintf("  %s | %s\n", fmtLineNo(dl.NewLineNo), dl.Content))
			}
		}

		// Context after
		hunkEnd := eh.Hunk.NewStart + eh.Hunk.NewLines - 1
		for i, line := range eh.ContextAfter {
			sb.WriteString(fmt.Sprintf("  %s | %s\n", fmtLineNo(hunkEnd+1+i), line))
		}

		sb.WriteString("```\n\n")
	}

	return sb.String()
}

func fmtLineNo(line int) string {
	if line <= 0 {
		return "?"
	}
	return strconv.Itoa(line)
}
