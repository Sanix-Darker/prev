package guidelines

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxFiles        = 12
	maxBytesPerFile = 1500
	maxBytesTotal   = 5000
)

var explicitCandidates = []string{
	"AGENTS.md",
	"CLAUDE.md",
	".claude/CLAUDE.md",
	".claude/agents.md",
	".github/copilot-instructions.md",
	".copilot-instructions.md",
}

// BuildPromptSection discovers repository-local guideline files and formats
// them for prompt injection. Empty string means no guideline files were found.
func BuildPromptSection(repoRoot string) string {
	root := strings.TrimSpace(repoRoot)
	if root == "" {
		return ""
	}

	paths := discoverGuidelinePaths(root)
	if len(paths) == 0 {
		return ""
	}

	var (
		total        int
		usedFiles    int
		sb           strings.Builder
		truncatedAny bool
	)

	sb.WriteString("## Repository Guidelines\n")
	sb.WriteString("Apply these repository rules when reviewing. If a rule conflicts with correctness or security, call out the conflict and prioritize correctness/security.\n\n")

	for _, p := range paths {
		if usedFiles >= maxFiles || total >= maxBytesTotal {
			break
		}

		b, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			continue
		}

		content := strings.TrimSpace(string(b))
		if content == "" {
			continue
		}

		if len(content) > maxBytesPerFile {
			content = strings.TrimSpace(content[:maxBytesPerFile]) + "\n...[truncated]"
			truncatedAny = true
		}

		remaining := maxBytesTotal - total
		if len(content) > remaining {
			content = strings.TrimSpace(content[:remaining]) + "\n...[truncated]"
			truncatedAny = true
		}
		if strings.TrimSpace(content) == "" {
			break
		}

		sb.WriteString(fmt.Sprintf("### %s\n", p))
		sb.WriteString("```markdown\n")
		sb.WriteString(content)
		sb.WriteString("\n```\n\n")

		total += len(content)
		usedFiles++
	}

	if usedFiles == 0 {
		return ""
	}
	if truncatedAny {
		sb.WriteString("Note: guideline content was truncated to fit prompt budget.\n")
	}

	return strings.TrimSpace(sb.String())
}

func discoverGuidelinePaths(root string) []string {
	seen := map[string]struct{}{}
	var out []string

	addIfFile := func(rel string) {
		if _, ok := seen[rel]; ok {
			return
		}
		info, err := os.Stat(filepath.Join(root, rel))
		if err != nil || info.IsDir() {
			return
		}
		seen[rel] = struct{}{}
		out = append(out, rel)
	}

	for _, rel := range explicitCandidates {
		addIfFile(rel)
	}

	for _, rel := range listMarkdownFiles(root, ".claude") {
		addIfFile(rel)
	}

	for _, rel := range listMarkdownFiles(root, ".github/instructions") {
		addIfFile(rel)
	}

	sort.Strings(out)
	return out
}

func listMarkdownFiles(root, relDir string) []string {
	dir := filepath.Join(root, relDir)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil
	}

	var files []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".md") {
			if rel, err := filepath.Rel(root, path); err == nil {
				files = append(files, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	sort.Strings(files)
	return files
}
