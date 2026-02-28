package diffparse

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// FileChange represents a parsed file diff.
type FileChange struct {
	OldName   string
	NewName   string
	IsNew     bool
	IsDeleted bool
	IsRenamed bool
	IsBinary  bool
	Hunks     []Hunk
	Stats     DiffStats
}

// Hunk represents a diff hunk.
type Hunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	Lines    []DiffLine
}

// DiffLine represents a single line in a diff.
type DiffLine struct {
	Type      LineType
	Content   string
	OldLineNo int
	NewLineNo int
}

// LineType classifies a diff line.
type LineType int

const (
	LineContext LineType = iota
	LineAdded
	LineDeleted
)

// DiffStats holds addition/deletion counts.
type DiffStats struct {
	Additions int
	Deletions int
}

// ParseGitDiff parses raw unified diff output into structured FileChanges.
func ParseGitDiff(raw string) ([]FileChange, error) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	var changes []FileChange
	var current *FileChange
	var currentHunk *Hunk
	var oldLine, newLine int

	flushHunk := func() {
		if current != nil && currentHunk != nil {
			current.Hunks = append(current.Hunks, *currentHunk)
			currentHunk = nil
		}
	}

	flushFile := func() {
		flushHunk()
		if current == nil {
			return
		}
		finalizeFileChange(current)
		changes = append(changes, *current)
		current = nil
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			flushFile()
			oldName, newName := parseDiffGitHeader(line)
			current = &FileChange{
				OldName: oldName,
				NewName: newName,
			}
			continue
		}

		if current == nil {
			continue
		}

		if strings.HasPrefix(line, "new file mode ") {
			current.IsNew = true
			continue
		}
		if strings.HasPrefix(line, "deleted file mode ") {
			current.IsDeleted = true
			continue
		}
		if strings.HasPrefix(line, "rename from ") {
			current.IsRenamed = true
			current.OldName = cleanPath(strings.TrimPrefix(line, "rename from "))
			continue
		}
		if strings.HasPrefix(line, "rename to ") {
			current.IsRenamed = true
			current.NewName = cleanPath(strings.TrimPrefix(line, "rename to "))
			continue
		}
		if strings.HasPrefix(line, "Binary files ") || strings.Contains(line, "GIT binary patch") {
			current.IsBinary = true
			continue
		}
		if strings.HasPrefix(line, "--- ") {
			path := parsePathMarker(strings.TrimPrefix(line, "--- "))
			if path == "/dev/null" {
				current.IsNew = true
				current.OldName = ""
			} else {
				current.OldName = cleanPath(path)
			}
			continue
		}
		if strings.HasPrefix(line, "+++ ") {
			path := parsePathMarker(strings.TrimPrefix(line, "+++ "))
			if path == "/dev/null" {
				current.IsDeleted = true
				current.NewName = ""
			} else {
				current.NewName = cleanPath(path)
			}
			continue
		}

		if strings.HasPrefix(line, "@@ ") {
			flushHunk()
			h, ok := parseHunkHeader(line)
			if !ok {
				continue
			}
			currentHunk = &h
			oldLine = h.OldStart
			newLine = h.NewStart
			continue
		}

		if currentHunk == nil {
			continue
		}
		appendHunkLine(current, currentHunk, line, &oldLine, &newLine)
	}

	flushFile()

	if len(changes) == 0 && strings.TrimSpace(raw) != "" {
		return nil, fmt.Errorf("failed to parse diff: no file diffs found")
	}
	return changes, nil
}

// ParseGitLabDiffs converts GitLab MR diff responses into FileChanges.
func ParseGitLabDiffs(diffs []GitLabDiff) ([]FileChange, error) {
	var changes []FileChange
	for _, d := range diffs {
		fc := FileChange{
			OldName:   d.OldPath,
			NewName:   d.NewPath,
			IsNew:     d.NewFile,
			IsDeleted: d.DeletedFile,
			IsRenamed: d.RenamedFile,
		}
		if strings.Contains(d.Diff, "Binary files") || strings.Contains(d.Diff, "GIT binary patch") {
			fc.IsBinary = true
		}
		if !fc.IsBinary {
			fc.IsBinary = isBinaryReviewPath(changePath(fc))
		}

		if d.Diff != "" {
			parseRawHunksInto(&fc, d.Diff)
		}

		finalizeFileChange(&fc)
		changes = append(changes, fc)
	}

	return changes, nil
}

func parseRawHunksInto(fc *FileChange, raw string) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	var currentHunk *Hunk
	var oldLine, newLine int
	flushHunk := func() {
		if currentHunk != nil {
			fc.Hunks = append(fc.Hunks, *currentHunk)
			currentHunk = nil
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "@@ ") {
			flushHunk()
			h, ok := parseHunkHeader(line)
			if !ok {
				continue
			}
			currentHunk = &h
			oldLine = h.OldStart
			newLine = h.NewStart
			continue
		}
		if currentHunk == nil {
			continue
		}
		appendHunkLine(fc, currentHunk, line, &oldLine, &newLine)
	}

	flushHunk()
}

func appendHunkLine(fc *FileChange, h *Hunk, line string, oldLine, newLine *int) {
	if line == "" || line == `\ No newline at end of file` {
		return
	}

	dl := DiffLine{}
	switch line[0] {
	case '+':
		dl.Type = LineAdded
		dl.Content = line[1:]
		dl.NewLineNo = *newLine
		*newLine++
		fc.Stats.Additions++
	case '-':
		dl.Type = LineDeleted
		dl.Content = line[1:]
		dl.OldLineNo = *oldLine
		*oldLine++
		fc.Stats.Deletions++
	default:
		dl.Type = LineContext
		if line[0] == ' ' {
			dl.Content = line[1:]
		} else {
			dl.Content = line
		}
		dl.OldLineNo = *oldLine
		dl.NewLineNo = *newLine
		*oldLine++
		*newLine++
	}

	h.Lines = append(h.Lines, dl)
}

func parseDiffGitHeader(line string) (string, string) {
	parts := strings.Fields(line)
	if len(parts) < 4 {
		return "", ""
	}
	return cleanPath(strings.Trim(parts[2], `"`)), cleanPath(strings.Trim(parts[3], `"`))
}

func parsePathMarker(raw string) string {
	s := strings.TrimSpace(raw)
	if idx := strings.IndexByte(s, '\t'); idx >= 0 {
		s = s[:idx]
	}
	if strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		s = strings.Trim(s, `"`)
	} else if idx := strings.IndexByte(s, ' '); idx >= 0 {
		s = s[:idx]
	}
	return s
}

func parseHunkHeader(line string) (Hunk, bool) {
	if !strings.HasPrefix(line, "@@ -") {
		return Hunk{}, false
	}
	rest := strings.TrimPrefix(line, "@@ -")
	idx := strings.Index(rest, " @@")
	if idx < 0 {
		return Hunk{}, false
	}
	rangePart := rest[:idx]
	parts := strings.Split(rangePart, " +")
	if len(parts) != 2 {
		return Hunk{}, false
	}

	oldStart, oldLines, ok := parseRange(parts[0])
	if !ok {
		return Hunk{}, false
	}
	newStart, newLines, ok := parseRange(parts[1])
	if !ok {
		return Hunk{}, false
	}

	return Hunk{
		OldStart: oldStart,
		OldLines: oldLines,
		NewStart: newStart,
		NewLines: newLines,
	}, true
}

func parseRange(s string) (int, int, bool) {
	start := 0
	lines := 1
	parts := strings.SplitN(strings.TrimSpace(s), ",", 2)

	v, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	start = v

	if len(parts) == 2 {
		v, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, false
		}
		lines = v
	}
	return start, lines, true
}

func finalizeFileChange(fc *FileChange) {
	if fc.IsNew {
		fc.OldName = ""
	}
	if fc.IsDeleted {
		fc.NewName = ""
	}
	if !fc.IsRenamed && fc.OldName != "" && fc.NewName != "" && fc.OldName != fc.NewName {
		fc.IsRenamed = true
	}
	if !fc.IsBinary {
		fc.IsBinary = isBinaryReviewPath(changePath(*fc))
	}
}

// GitLabDiff mirrors the structure used by the GitLab API.
type GitLabDiff struct {
	OldPath     string
	NewPath     string
	Diff        string
	NewFile     bool
	RenamedFile bool
	DeletedFile bool
}

// FormatForReview formats file changes into a string suitable for AI review.
func FormatForReview(changes []FileChange) string {
	var sb strings.Builder

	for _, fc := range changes {
		if fc.IsBinary {
			continue
		}

		name := fc.NewName
		if name == "" {
			name = fc.OldName
		}

		label := "Modified"
		if fc.IsNew {
			label = "New"
		} else if fc.IsDeleted {
			label = "Deleted"
		} else if fc.IsRenamed {
			label = fmt.Sprintf("Renamed from %s", fc.OldName)
		}

		sb.WriteString(fmt.Sprintf("### File: %s (%s) [+%d/-%d]\n",
			name, label, fc.Stats.Additions, fc.Stats.Deletions))

		for _, h := range fc.Hunks {
			sb.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n",
				h.OldStart, h.OldLines, h.NewStart, h.NewLines))
			for _, l := range h.Lines {
				switch l.Type {
				case LineAdded:
					sb.WriteString(fmt.Sprintf("+%s\n", l.Content))
				case LineDeleted:
					sb.WriteString(fmt.Sprintf("-%s\n", l.Content))
				default:
					sb.WriteString(fmt.Sprintf(" %s\n", l.Content))
				}
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// FilterTextChanges returns only reviewable text-file changes.
func FilterTextChanges(changes []FileChange) []FileChange {
	out := make([]FileChange, 0, len(changes))
	for _, fc := range changes {
		if fc.IsBinary {
			continue
		}
		out = append(out, fc)
	}
	return out
}

func changePath(fc FileChange) string {
	if strings.TrimSpace(fc.NewName) != "" {
		return fc.NewName
	}
	return fc.OldName
}

func isBinaryReviewPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(path)))
	switch ext {
	case ".pdf", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".ico", ".tiff", ".heic",
		".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar",
		".jar", ".war", ".so", ".dll", ".dylib", ".a", ".o", ".obj", ".exe", ".bin", ".class",
		".woff", ".woff2", ".ttf", ".otf", ".eot",
		".mp3", ".mp4", ".mov", ".wav", ".avi", ".mkv", ".flac":
		return true
	default:
		return false
	}
}

func cleanPath(p string) string {
	p = strings.TrimPrefix(p, "a/")
	p = strings.TrimPrefix(p, "b/")
	return p
}
