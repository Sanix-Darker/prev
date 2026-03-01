package cmd

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sanix-darker/prev/internal/core"
	"github.com/sanix-darker/prev/internal/diffparse"
	"github.com/sanix-darker/prev/internal/vcs"
)

const (
	defaultReviewMemoryFile = ".prev/review-memory.md"
	reviewMemoryVersion     = 1
)

var reviewMemoryJSONFence = regexp.MustCompile("(?s)```prev-memory-json\\s*(\\{.*?\\})\\s*```")

type reviewMemory struct {
	Version   int                 `json:"version"`
	UpdatedAt string              `json:"updated_at"`
	Entries   []reviewMemoryEntry `json:"entries"`
}

type reviewMemoryEntry struct {
	ID        string `json:"id"`
	RuleID    string `json:"rule_id"`
	Status    string `json:"status"` // open|fixed
	Severity  string `json:"severity"`
	FilePath  string `json:"file_path"`
	Line      int    `json:"line"`
	Message   string `json:"message"`
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
	Hits      int    `json:"hits"`
	Fixes     int    `json:"fixes"`
	LastMR    string `json:"last_mr"`
}

func loadReviewMemory(repoPath, configuredPath string) (reviewMemory, string, error) {
	path := resolveReviewMemoryPath(repoPath, configuredPath)
	if strings.TrimSpace(path) == "" {
		return reviewMemory{Version: reviewMemoryVersion}, "", nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return reviewMemory{Version: reviewMemoryVersion}, path, nil
		}
		return reviewMemory{}, path, err
	}
	mem, err := parseReviewMemoryMarkdown(raw)
	if err != nil {
		return reviewMemory{}, path, err
	}
	normalizeReviewMemory(&mem)
	return mem, path, nil
}

func resolveReviewMemoryPath(repoPath, configuredPath string) string {
	path := strings.TrimSpace(configuredPath)
	if path == "" {
		path = defaultReviewMemoryFile
	}
	if filepath.IsAbs(path) {
		return path
	}
	root := strings.TrimSpace(repoPath)
	if root == "" {
		root = "."
	}
	return filepath.Join(root, path)
}

func parseReviewMemoryMarkdown(raw []byte) (reviewMemory, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return reviewMemory{Version: reviewMemoryVersion}, nil
	}

	var payload string
	if m := reviewMemoryJSONFence.FindStringSubmatch(text); len(m) == 2 {
		payload = m[1]
	} else {
		// Backward compatibility: allow plain JSON file contents.
		payload = text
	}

	var mem reviewMemory
	if err := json.Unmarshal([]byte(payload), &mem); err != nil {
		return reviewMemory{}, fmt.Errorf("invalid review memory payload: %w", err)
	}
	if mem.Version == 0 {
		mem.Version = reviewMemoryVersion
	}
	if mem.Entries == nil {
		mem.Entries = []reviewMemoryEntry{}
	}
	return mem, nil
}

func saveReviewMemory(path string, mem reviewMemory) error {
	normalizeReviewMemory(&mem)
	mem.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	raw, err := json.MarshalIndent(mem, "", "  ")
	if err != nil {
		return err
	}
	content := renderReviewMemoryMarkdown(mem, string(raw))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func renderReviewMemoryMarkdown(mem reviewMemory, payload string) string {
	openCount, fixedCount := reviewMemoryCounts(mem)
	var sb strings.Builder
	sb.WriteString("# prev Review Memory\n\n")
	sb.WriteString("<!-- prev:memory:v1 -->\n\n")
	sb.WriteString("Persistent reviewer memory across merge requests.\n\n")
	sb.WriteString("## Snapshot\n\n")
	sb.WriteString(fmt.Sprintf("- Updated: `%s`\n", strings.TrimSpace(mem.UpdatedAt)))
	sb.WriteString(fmt.Sprintf("- Entries: `%d`\n", len(mem.Entries)))
	sb.WriteString(fmt.Sprintf("- Open: `%d`\n", openCount))
	sb.WriteString(fmt.Sprintf("- Fixed: `%d`\n\n", fixedCount))

	writeMemoryTable := func(title string, filter func(reviewMemoryEntry) bool) {
		sb.WriteString("## " + title + "\n\n")
		sb.WriteString("| File | Line | Severity | Hits | Message |\n")
		sb.WriteString("|---|---:|---|---:|---|\n")
		written := 0
		for _, e := range mem.Entries {
			if !filter(e) {
				continue
			}
			sb.WriteString(fmt.Sprintf("| `%s` | %d | %s | %d | %s |\n",
				escapeTableCell(e.FilePath), e.Line, strings.ToUpper(e.Severity), e.Hits, escapeTableCell(e.Message)))
			written++
			if written >= 30 {
				break
			}
		}
		if written == 0 {
			sb.WriteString("| _none_ |  |  |  |  |\n")
		}
		sb.WriteString("\n")
	}

	writeMemoryTable("Open Findings", func(e reviewMemoryEntry) bool { return e.Status == "open" })
	writeMemoryTable("Fixed Findings", func(e reviewMemoryEntry) bool { return e.Status == "fixed" })

	sb.WriteString("## Machine Data\n\n")
	sb.WriteString("```prev-memory-json\n")
	sb.WriteString(payload)
	sb.WriteString("\n```\n")
	return sb.String()
}

func escapeTableCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func normalizeReviewMemory(mem *reviewMemory) {
	if mem.Version == 0 {
		mem.Version = reviewMemoryVersion
	}
	if mem.Entries == nil {
		mem.Entries = []reviewMemoryEntry{}
	}
	for i := range mem.Entries {
		mem.Entries[i].Status = normalizeMemoryStatus(mem.Entries[i].Status)
		mem.Entries[i].Severity = strings.ToUpper(strings.TrimSpace(mem.Entries[i].Severity))
		mem.Entries[i].FilePath = strings.TrimSpace(mem.Entries[i].FilePath)
		mem.Entries[i].Message = strings.TrimSpace(mem.Entries[i].Message)
		if mem.Entries[i].ID == "" {
			mem.Entries[i].ID = memoryEntryID(mem.Entries[i].FilePath, mem.Entries[i].Line, mem.Entries[i].Message)
		}
		if mem.Entries[i].RuleID == "" {
			mem.Entries[i].RuleID = memoryRuleID(mem.Entries[i].Message)
		}
	}
	sort.SliceStable(mem.Entries, func(i, j int) bool {
		ri := severityRank(mem.Entries[i].Severity)
		rj := severityRank(mem.Entries[j].Severity)
		if ri != rj {
			return ri > rj
		}
		if mem.Entries[i].Status != mem.Entries[j].Status {
			return mem.Entries[i].Status == "open"
		}
		return mem.Entries[i].LastSeen > mem.Entries[j].LastSeen
	})
}

func reviewMemoryCounts(mem reviewMemory) (openCount, fixedCount int) {
	for _, e := range mem.Entries {
		switch e.Status {
		case "open":
			openCount++
		case "fixed":
			fixedCount++
		}
	}
	return openCount, fixedCount
}

func normalizeMemoryStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "fixed":
		return "fixed"
	default:
		return "open"
	}
}

func memoryRuleID(message string) string {
	norm := normalizeMemoryMessage(message)
	sum := sha1.Sum([]byte(norm))
	return fmt.Sprintf("%x", sum[:8])
}

func memoryEntryID(filePath string, line int, message string) string {
	key := strings.ToLower(strings.TrimSpace(filePath)) + "|" + strconv.Itoa(line) + "|" + normalizeMemoryMessage(message)
	sum := sha1.Sum([]byte(key))
	return fmt.Sprintf("%x", sum[:10])
}

func normalizeMemoryMessage(message string) string {
	msg := strings.ToLower(strings.TrimSpace(message))
	fields := strings.Fields(msg)
	return strings.Join(fields, " ")
}

func updateReviewMemoryFromDiscussions(mem *reviewMemory, discussions []vcs.MRDiscussion, mrRef string, now time.Time) bool {
	type noteState struct {
		Severity string
		Status   string
		FilePath string
		Line     int
		Message  string
	}
	byID := map[string]noteState{}
	for _, d := range discussions {
		for _, n := range d.Notes {
			if n.FilePath == "" || n.Line <= 0 {
				continue
			}
			sev, msg, ok := severityAndMessage(n.Body)
			if !ok {
				continue
			}
			status := ""
			if n.Resolved {
				status = "fixed"
			} else if n.Resolvable {
				status = "open"
			}
			if status == "" {
				continue
			}
			id := memoryEntryID(n.FilePath, n.Line, msg)
			if curr, exists := byID[id]; exists {
				// unresolved beats resolved for same key.
				if curr.Status == "open" {
					continue
				}
				if status == "open" {
					byID[id] = noteState{
						Severity: sev,
						Status:   status,
						FilePath: n.FilePath,
						Line:     n.Line,
						Message:  msg,
					}
				}
				continue
			}
			byID[id] = noteState{
				Severity: sev,
				Status:   status,
				FilePath: n.FilePath,
				Line:     n.Line,
				Message:  msg,
			}
		}
	}

	changed := false
	for id, st := range byID {
		if st.Status == "open" {
			if upsertReviewMemory(mem, id, st.FilePath, st.Line, st.Severity, st.Message, "open", mrRef, now) {
				changed = true
			}
		} else {
			if upsertReviewMemory(mem, id, st.FilePath, st.Line, st.Severity, st.Message, "fixed", mrRef, now) {
				changed = true
			}
		}
	}
	return changed
}

func updateReviewMemoryFromFindings(mem *reviewMemory, findings []core.FileComment, mrRef string, now time.Time) bool {
	changed := false
	for _, f := range findings {
		filePath := strings.TrimSpace(strings.TrimPrefix(f.FilePath, "./"))
		if filePath == "" || f.Line <= 0 || strings.TrimSpace(f.Message) == "" {
			continue
		}
		id := memoryEntryID(filePath, f.Line, f.Message)
		if upsertReviewMemory(mem, id, filePath, f.Line, f.Severity, f.Message, "open", mrRef, now) {
			changed = true
		}
	}
	return changed
}

func upsertReviewMemory(
	mem *reviewMemory,
	id, filePath string,
	line int,
	severity, message, status, mrRef string,
	now time.Time,
) bool {
	normalizeReviewMemory(mem)
	status = normalizeMemoryStatus(status)
	when := now.UTC().Format(time.RFC3339)
	if id == "" {
		id = memoryEntryID(filePath, line, message)
	}

	for i := range mem.Entries {
		if mem.Entries[i].ID != id {
			continue
		}
		before := mem.Entries[i]
		mem.Entries[i].Status = status
		if severityRank(severity) > severityRank(mem.Entries[i].Severity) {
			mem.Entries[i].Severity = strings.ToUpper(strings.TrimSpace(severity))
		}
		if mem.Entries[i].Severity == "" {
			mem.Entries[i].Severity = strings.ToUpper(strings.TrimSpace(severity))
		}
		if strings.TrimSpace(message) != "" {
			mem.Entries[i].Message = strings.TrimSpace(message)
		}
		mem.Entries[i].FilePath = strings.TrimSpace(filePath)
		mem.Entries[i].Line = line
		mem.Entries[i].LastSeen = when
		mem.Entries[i].LastMR = mrRef
		mem.Entries[i].RuleID = memoryRuleID(mem.Entries[i].Message)
		if status == "open" {
			mem.Entries[i].Hits++
		}
		if status == "fixed" && before.Status != "fixed" {
			mem.Entries[i].Fixes++
		}
		return before != mem.Entries[i]
	}

	entry := reviewMemoryEntry{
		ID:        id,
		RuleID:    memoryRuleID(message),
		Status:    status,
		Severity:  strings.ToUpper(strings.TrimSpace(severity)),
		FilePath:  strings.TrimSpace(filePath),
		Line:      line,
		Message:   strings.TrimSpace(message),
		FirstSeen: when,
		LastSeen:  when,
		Hits:      0,
		Fixes:     0,
		LastMR:    mrRef,
	}
	if entry.Severity == "" {
		entry.Severity = "MEDIUM"
	}
	if status == "open" {
		entry.Hits = 1
	}
	if status == "fixed" {
		entry.Fixes = 1
	}
	mem.Entries = append(mem.Entries, entry)
	return true
}

func appendReviewMemoryGuidelines(guidelines string, mem reviewMemory, changes []diffparse.FileChange, maxItems int) string {
	if maxItems <= 0 {
		maxItems = 10
	}
	normalizeReviewMemory(&mem)
	if len(mem.Entries) == 0 {
		return guidelines
	}
	changedPaths := map[string]struct{}{}
	for _, c := range changes {
		path := strings.TrimSpace(c.NewName)
		if path == "" {
			path = strings.TrimSpace(c.OldName)
		}
		if path == "" {
			continue
		}
		changedPaths[strings.ToLower(path)] = struct{}{}
	}

	relevant := make([]reviewMemoryEntry, 0, maxItems)
	for _, e := range mem.Entries {
		if _, ok := changedPaths[strings.ToLower(e.FilePath)]; !ok {
			continue
		}
		relevant = append(relevant, e)
		if len(relevant) >= maxItems {
			break
		}
	}
	if len(relevant) == 0 {
		for _, e := range mem.Entries {
			if e.Status != "open" {
				continue
			}
			relevant = append(relevant, e)
			if len(relevant) >= minInt(3, maxItems) {
				break
			}
		}
	}
	if len(relevant) == 0 {
		return guidelines
	}

	lines := []string{
		"Historical reviewer memory from prior MRs (use this for consistency and regression checks):",
	}
	for i, e := range relevant {
		if i >= maxItems {
			break
		}
		lines = append(lines, fmt.Sprintf("- %s `%s:%d` [%s] %s (hits=%d fixes=%d)",
			strings.ToUpper(e.Status), e.FilePath, e.Line, strings.ToUpper(e.Severity),
			strings.TrimSpace(e.Message), e.Hits, e.Fixes))
	}
	lines = append(lines,
		"- Do not repeat fixed findings unless the issue reappears in the current diff.",
		"- Prioritize recurring open findings when they are still present.",
	)

	block := strings.Join(lines, "\n")
	if strings.TrimSpace(guidelines) == "" {
		return block
	}
	return guidelines + "\n" + block
}

func trimReviewMemory(mem *reviewMemory, maxEntries int) {
	if maxEntries <= 0 || len(mem.Entries) <= maxEntries {
		return
	}
	sort.SliceStable(mem.Entries, func(i, j int) bool {
		if mem.Entries[i].Status != mem.Entries[j].Status {
			return mem.Entries[i].Status == "open"
		}
		ri := severityRank(mem.Entries[i].Severity)
		rj := severityRank(mem.Entries[j].Severity)
		if ri != rj {
			return ri > rj
		}
		if mem.Entries[i].LastSeen != mem.Entries[j].LastSeen {
			return mem.Entries[i].LastSeen > mem.Entries[j].LastSeen
		}
		return mem.Entries[i].Hits > mem.Entries[j].Hits
	})
	mem.Entries = mem.Entries[:maxEntries]
}
