package diffparse

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRepo creates a temporary git repo with a file for testing.
func setupTestRepo(t *testing.T) (repoPath, baseBranch, targetBranch string) {
	t.Helper()

	dir, err := os.MkdirTemp("", "prev-context-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })

	// Init repo
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v failed: %s", args, string(out))
	}

	run("init", "-b", "main")

	// Create initial file on main
	content := `package main

import "fmt"

func hello() {
	fmt.Println("hello")
}

func world() {
	fmt.Println("world")
}

func extra() {
	fmt.Println("extra")
}

func more() {
	fmt.Println("more")
}

func last() {
	fmt.Println("last")
}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte(content), 0644))
	run("add", "main.go")
	run("commit", "-m", "initial")

	// Create feature branch with changes
	run("checkout", "-b", "feature")

	newContent := `package main

import "fmt"

func hello() {
	fmt.Println("hello world!")
}

func world() {
	fmt.Println("world")
}

func extra() {
	fmt.Println("extra updated")
}

func more() {
	fmt.Println("more")
}

func last() {
	fmt.Println("last")
}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte(newContent), 0644))
	run("add", "main.go")
	run("commit", "-m", "update hello and extra")

	return dir, "main", "feature"
}

func TestEnrichFileChanges(t *testing.T) {
	repoPath, baseBranch, targetBranch := setupTestRepo(t)

	// Get the diff
	changes := []FileChange{
		{
			OldName: "main.go",
			NewName: "main.go",
			Hunks: []Hunk{
				{
					NewStart: 6,
					NewLines: 1,
					OldStart: 6,
					OldLines: 1,
					Lines: []DiffLine{
						{Type: LineDeleted, Content: `	fmt.Println("hello")`, OldLineNo: 6},
						{Type: LineAdded, Content: `	fmt.Println("hello world!")`, NewLineNo: 6},
					},
				},
			},
			Stats: DiffStats{Additions: 1, Deletions: 1},
		},
	}

	enriched, err := EnrichFileChanges(changes, repoPath, baseBranch, targetBranch, 3, 80000, nil)
	require.NoError(t, err)
	require.Len(t, enriched, 1)

	efc := enriched[0]
	assert.Equal(t, "go", efc.Language)
	assert.NotEmpty(t, efc.FullNewContent)
	assert.Greater(t, efc.TokenEstimate, 0)
	assert.NotEmpty(t, efc.EnrichedHunks)

	// Context lines should be present
	eh := efc.EnrichedHunks[0]
	assert.NotEmpty(t, eh.ContextBefore)
	assert.NotEmpty(t, eh.ContextAfter)
	assert.GreaterOrEqual(t, eh.StartLine, 1)
}

func TestHunkMerging(t *testing.T) {
	// Two hunks close together should merge
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "line content"
	}

	hunks := []Hunk{
		{NewStart: 5, NewLines: 2, OldStart: 5, OldLines: 2,
			Lines: []DiffLine{{Type: LineAdded, Content: "a", NewLineNo: 5}}},
		{NewStart: 10, NewLines: 2, OldStart: 10, OldLines: 2,
			Lines: []DiffLine{{Type: LineAdded, Content: "b", NewLineNo: 10}}},
	}

	// With contextLines=5, hunks at 5 and 10 should merge (5+5=10 overlaps with 10-5=5)
	result := enrichHunks(hunks, lines, 5)
	assert.Len(t, result, 1, "close hunks should merge")
}

func TestEnrich_NewFile(t *testing.T) {
	changes := []FileChange{
		{
			NewName: "new_file.go",
			IsNew:   true,
			Hunks: []Hunk{
				{NewStart: 1, NewLines: 3, Lines: []DiffLine{
					{Type: LineAdded, Content: "package main", NewLineNo: 1},
					{Type: LineAdded, Content: "", NewLineNo: 2},
					{Type: LineAdded, Content: "func main() {}", NewLineNo: 3},
				}},
			},
			Stats: DiffStats{Additions: 3},
		},
	}

	// For new files, GetFileContent will fail (no repo), but we handle gracefully
	enriched, err := EnrichFileChanges(changes, "/nonexistent", "main", "feature", 3, 80000, nil)
	require.NoError(t, err)
	require.Len(t, enriched, 1)
	assert.Equal(t, "go", enriched[0].Language)
	assert.True(t, enriched[0].IsNew)
}

func TestEnrich_DeletedFile(t *testing.T) {
	changes := []FileChange{
		{
			OldName:   "old_file.py",
			IsDeleted: true,
			Stats:     DiffStats{Deletions: 10},
		},
	}

	enriched, err := EnrichFileChanges(changes, "/nonexistent", "main", "feature", 3, 80000, nil)
	require.NoError(t, err)
	require.Len(t, enriched, 1)
	assert.True(t, enriched[0].IsDeleted)
	assert.Equal(t, "python", enriched[0].Language)
}

func TestEnrichFileChanges_TokenBudgetExceeded(t *testing.T) {
	repoPath, baseBranch, targetBranch := setupTestRepo(t)

	changes := []FileChange{
		{
			OldName: "main.go",
			NewName: "main.go",
			Hunks: []Hunk{
				{
					NewStart: 6,
					NewLines: 1,
					OldStart: 6,
					OldLines: 1,
					Lines: []DiffLine{
						{Type: LineDeleted, Content: `	fmt.Println("hello")`, OldLineNo: 6},
						{Type: LineAdded, Content: `	fmt.Println("hello world!")`, NewLineNo: 6},
					},
				},
			},
			Stats: DiffStats{Additions: 1, Deletions: 1},
		},
	}

	// Use a very small token budget to trigger the contextLines=3 fallback
	enriched, err := EnrichFileChanges(changes, repoPath, baseBranch, targetBranch, 10, 1, nil)
	require.NoError(t, err)
	require.Len(t, enriched, 1)

	// Should still have enriched hunks, just with reduced context (3 lines)
	assert.NotEmpty(t, enriched[0].EnrichedHunks)
}

func TestEnrichHunks_EmptyInput(t *testing.T) {
	// nil hunks
	result := enrichHunks(nil, []string{"line1"}, 3)
	assert.Nil(t, result)

	// nil lines
	result = enrichHunks([]Hunk{{NewStart: 1, NewLines: 1}}, nil, 3)
	assert.Nil(t, result)

	// empty hunks
	result = enrichHunks([]Hunk{}, []string{"line1"}, 3)
	assert.Nil(t, result)

	// empty lines
	result = enrichHunks([]Hunk{{NewStart: 1, NewLines: 1}}, []string{}, 3)
	assert.Nil(t, result)
}

func TestEnrichHunks_HunkAtFileStart(t *testing.T) {
	lines := []string{"line1", "line2", "line3", "line4", "line5"}
	hunks := []Hunk{
		{NewStart: 1, NewLines: 1, OldStart: 1, OldLines: 1,
			Lines: []DiffLine{{Type: LineAdded, Content: "changed", NewLineNo: 1}}},
	}

	result := enrichHunks(hunks, lines, 3)
	require.Len(t, result, 1)
	// No context before since hunk starts at line 1
	assert.Empty(t, result[0].ContextBefore)
	assert.Equal(t, 1, result[0].StartLine)
}

func TestEnrichHunks_HunkAtFileEnd(t *testing.T) {
	lines := []string{"line1", "line2", "line3", "line4", "line5"}
	hunks := []Hunk{
		{NewStart: 5, NewLines: 1, OldStart: 5, OldLines: 1,
			Lines: []DiffLine{{Type: LineAdded, Content: "changed", NewLineNo: 5}}},
	}

	result := enrichHunks(hunks, lines, 3)
	require.Len(t, result, 1)
	// No context after since hunk is at last line
	assert.Empty(t, result[0].ContextAfter)
	assert.Equal(t, len(lines), result[0].EndLine)
}

func TestEnrichFileChanges_BinaryFile(t *testing.T) {
	changes := []FileChange{
		{
			NewName:  "image.png",
			IsBinary: true,
			Stats:    DiffStats{},
		},
	}

	enriched, err := EnrichFileChanges(changes, "/nonexistent", "main", "feature", 3, 80000, nil)
	require.NoError(t, err)
	require.Len(t, enriched, 1)
	assert.True(t, enriched[0].IsBinary)
	assert.Equal(t, 100, enriched[0].TokenEstimate)
	assert.Empty(t, enriched[0].EnrichedHunks)
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"main.go", "go"},
		{"script.py", "python"},
		{"app.js", "javascript"},
		{"index.ts", "typescript"},
		{"component.tsx", "tsx"},
		{"Dockerfile", "dockerfile"},
		{"Makefile", "makefile"},
		{"style.css", "css"},
		{"data.json", "json"},
		{"config.yaml", "yaml"},
		{"query.sql", "sql"},
		{"lib.rs", "rust"},
		{"Main.java", "java"},
		{"unknown.xyz", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, DetectLanguage(tt.path))
		})
	}
}

func TestFormatEnrichedForReview(t *testing.T) {
	efc := EnrichedFileChange{
		FileChange: FileChange{
			NewName: "main.go",
			Stats:   DiffStats{Additions: 1, Deletions: 1},
		},
		Language: "go",
		EnrichedHunks: []EnrichedHunk{
			{
				Hunk: Hunk{
					NewStart: 5,
					NewLines: 2,
					Lines: []DiffLine{
						{Type: LineDeleted, Content: `old line`, OldLineNo: 5},
						{Type: LineAdded, Content: `new line`, NewLineNo: 5},
					},
				},
				ContextBefore: []string{"// context before"},
				ContextAfter:  []string{"// context after"},
				StartLine:     4,
				EndLine:        8,
			},
		},
	}

	output := FormatEnrichedForReview(efc)

	assert.Contains(t, output, "## File: main.go")
	assert.Contains(t, output, "(Modified)")
	assert.Contains(t, output, "[+1/-1]")
	assert.Contains(t, output, "[go]")
	assert.Contains(t, output, "### Lines 4-8:")
	assert.Contains(t, output, "// context before")
	assert.Contains(t, output, "+ new line")
	assert.Contains(t, output, "- old line")
	assert.Contains(t, output, "// context after")
	assert.Contains(t, output, "```go")
}
