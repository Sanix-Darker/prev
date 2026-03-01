package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sanix-darker/prev/internal/core"
	"github.com/sanix-darker/prev/internal/diffparse"
	"github.com/sanix-darker/prev/internal/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewMemory_SaveAndLoadMarkdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".prev", "review-memory.md")
	mem := reviewMemory{
		Version: reviewMemoryVersion,
		Entries: []reviewMemoryEntry{
			{
				ID:        "id1",
				RuleID:    "rule1",
				Status:    "open",
				Severity:  "HIGH",
				FilePath:  "public/index.php",
				Line:      31,
				Message:   "json_decode expects a string input.",
				FirstSeen: "2026-03-01T00:00:00Z",
				LastSeen:  "2026-03-01T00:00:00Z",
				Hits:      2,
				LastMR:    "grp/proj!2",
			},
		},
	}

	require.NoError(t, saveReviewMemory(path, mem))
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "```prev-memory-json")
	assert.Contains(t, string(raw), "| `public/index.php` | 31 | HIGH |")

	loaded, resolvedPath, err := loadReviewMemory(dir, ".prev/review-memory.md")
	require.NoError(t, err)
	assert.Equal(t, path, resolvedPath)
	require.Len(t, loaded.Entries, 1)
	assert.Equal(t, "public/index.php", loaded.Entries[0].FilePath)
	assert.Equal(t, "open", loaded.Entries[0].Status)
}

func TestUpdateReviewMemoryFromDiscussions_OpenBeatsFixed(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	mem := reviewMemory{Version: reviewMemoryVersion}
	discussions := []vcs.MRDiscussion{
		{
			ID: "d1",
			Notes: []vcs.MRDiscussionNote{
				{
					FilePath:   "public/index.php",
					Line:       31,
					Body:       "[HIGH] json_decode expects JSON string input.",
					Resolvable: true,
					Resolved:   true,
				},
				{
					FilePath:   "public/index.php",
					Line:       31,
					Body:       "[HIGH] json_decode expects JSON string input.",
					Resolvable: true,
					Resolved:   false,
				},
			},
		},
	}

	changed := updateReviewMemoryFromDiscussions(&mem, discussions, "grp/proj!3", now)
	assert.True(t, changed)
	require.Len(t, mem.Entries, 1)
	assert.Equal(t, "open", mem.Entries[0].Status)
	assert.Equal(t, 1, mem.Entries[0].Hits)
	assert.Equal(t, "grp/proj!3", mem.Entries[0].LastMR)
}

func TestUpdateReviewMemoryFromFindings(t *testing.T) {
	now := time.Date(2026, 3, 1, 13, 0, 0, 0, time.UTC)
	mem := reviewMemory{Version: reviewMemoryVersion}
	findings := []core.FileComment{
		{
			FilePath: "public/index.php",
			Line:     31,
			Severity: "HIGH",
			Message:  "json_decode expects JSON string input.",
		},
		{
			FilePath: "README.md",
			Line:     0, // ignored
			Severity: "LOW",
			Message:  "typo",
		},
	}

	changed := updateReviewMemoryFromFindings(&mem, findings, "grp/proj!5", now)
	assert.True(t, changed)
	require.Len(t, mem.Entries, 1)
	assert.Equal(t, "public/index.php", mem.Entries[0].FilePath)
	assert.Equal(t, "open", mem.Entries[0].Status)
}

func TestAppendReviewMemoryGuidelines_UsesRelevantChangedFiles(t *testing.T) {
	mem := reviewMemory{
		Version: reviewMemoryVersion,
		Entries: []reviewMemoryEntry{
			{
				ID:       "a",
				Status:   "open",
				Severity: "HIGH",
				FilePath: "public/index.php",
				Line:     31,
				Message:  "json_decode expects JSON string input.",
				Hits:     3,
				Fixes:    0,
				LastSeen: "2026-03-01T12:00:00Z",
			},
			{
				ID:       "b",
				Status:   "fixed",
				Severity: "MEDIUM",
				FilePath: "other/file.go",
				Line:     10,
				Message:  "old issue",
				Hits:     1,
				Fixes:    1,
				LastSeen: "2026-03-01T11:00:00Z",
			},
		},
	}
	changes := []diffparse.FileChange{
		{NewName: "public/index.php"},
	}

	out := appendReviewMemoryGuidelines("Base", mem, changes, 10)
	assert.Contains(t, out, "Historical reviewer memory")
	assert.Contains(t, out, "OPEN `public/index.php:31` [HIGH]")
	assert.NotContains(t, out, "other/file.go")
	assert.True(t, strings.HasPrefix(out, "Base"))
}
