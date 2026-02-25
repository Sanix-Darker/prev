package diffparse

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/go-diff/diff"
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
	fileDiffs, err := diff.ParseMultiFileDiff([]byte(raw))
	if err != nil {
		return nil, fmt.Errorf("failed to parse diff: %w", err)
	}

	var changes []FileChange
	for _, fd := range fileDiffs {
		fc := FileChange{
			OldName: cleanPath(fd.OrigName),
			NewName: cleanPath(fd.NewName),
		}

		// Detect file type
		if fd.OrigName == "/dev/null" {
			fc.IsNew = true
			fc.OldName = ""
		}
		if fd.NewName == "/dev/null" {
			fc.IsDeleted = true
			fc.NewName = ""
		}
		if fc.OldName != "" && fc.NewName != "" && fc.OldName != fc.NewName {
			fc.IsRenamed = true
		}

		// Check for binary
		if fd.Extended != nil {
			for _, ext := range fd.Extended {
				if strings.Contains(ext, "Binary files") || strings.Contains(ext, "GIT binary patch") {
					fc.IsBinary = true
					break
				}
			}
		}
		if !fc.IsBinary {
			fc.IsBinary = isBinaryReviewPath(changePath(fc))
		}

		// Parse hunks
		for _, h := range fd.Hunks {
			hunk := Hunk{
				OldStart: int(h.OrigStartLine),
				OldLines: int(h.OrigLines),
				NewStart: int(h.NewStartLine),
				NewLines: int(h.NewLines),
			}

			oldLine := int(h.OrigStartLine)
			newLine := int(h.NewStartLine)

			for _, line := range strings.Split(string(h.Body), "\n") {
				if len(line) == 0 {
					continue
				}

				dl := DiffLine{}
				switch line[0] {
				case '+':
					dl.Type = LineAdded
					dl.Content = line[1:]
					dl.NewLineNo = newLine
					dl.OldLineNo = 0
					newLine++
					fc.Stats.Additions++
				case '-':
					dl.Type = LineDeleted
					dl.Content = line[1:]
					dl.OldLineNo = oldLine
					dl.NewLineNo = 0
					oldLine++
					fc.Stats.Deletions++
				default:
					dl.Type = LineContext
					if len(line) > 0 && line[0] == ' ' {
						dl.Content = line[1:]
					} else {
						dl.Content = line
					}
					dl.OldLineNo = oldLine
					dl.NewLineNo = newLine
					oldLine++
					newLine++
				}
				hunk.Lines = append(hunk.Lines, dl)
			}

			fc.Hunks = append(fc.Hunks, hunk)
		}

		changes = append(changes, fc)
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
			// Create a pseudo unified diff header for the parser
			header := fmt.Sprintf("--- a/%s\n+++ b/%s\n", d.OldPath, d.NewPath)
			parsed, err := diff.ParseFileDiff([]byte(header + d.Diff))
			if err == nil && parsed != nil {
				for _, h := range parsed.Hunks {
					hunk := Hunk{
						OldStart: int(h.OrigStartLine),
						OldLines: int(h.OrigLines),
						NewStart: int(h.NewStartLine),
						NewLines: int(h.NewLines),
					}

					oldLine := int(h.OrigStartLine)
					newLine := int(h.NewStartLine)

					for _, line := range strings.Split(string(h.Body), "\n") {
						if len(line) == 0 {
							continue
						}
						dl := DiffLine{}
						switch line[0] {
						case '+':
							dl.Type = LineAdded
							dl.Content = line[1:]
							dl.NewLineNo = newLine
							newLine++
							fc.Stats.Additions++
						case '-':
							dl.Type = LineDeleted
							dl.Content = line[1:]
							dl.OldLineNo = oldLine
							oldLine++
							fc.Stats.Deletions++
						default:
							dl.Type = LineContext
							if len(line) > 0 && line[0] == ' ' {
								dl.Content = line[1:]
							} else {
								dl.Content = line
							}
							dl.OldLineNo = oldLine
							dl.NewLineNo = newLine
							oldLine++
							newLine++
						}
						hunk.Lines = append(hunk.Lines, dl)
					}
					fc.Hunks = append(fc.Hunks, hunk)
				}
			}
		}

		changes = append(changes, fc)
	}

	return changes, nil
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
