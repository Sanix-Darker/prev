package diffparse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleDiff = `diff --git a/main.go b/main.go
index 1234567..abcdef0 100644
--- a/main.go
+++ b/main.go
@@ -1,5 +1,6 @@
 package main

 import "fmt"
+import "os"

 func main() {
-    fmt.Println("hello")
+    fmt.Println(os.Args)
 }
`

func TestParseGitDiff(t *testing.T) {
	changes, err := ParseGitDiff(sampleDiff)
	require.NoError(t, err)
	require.Len(t, changes, 1)

	fc := changes[0]
	assert.Equal(t, "main.go", fc.OldName)
	assert.Equal(t, "main.go", fc.NewName)
	assert.False(t, fc.IsNew)
	assert.False(t, fc.IsDeleted)
	assert.False(t, fc.IsRenamed)
	assert.False(t, fc.IsBinary)
	assert.Equal(t, 2, fc.Stats.Additions)
	assert.Equal(t, 1, fc.Stats.Deletions)
}

const newFileDiff = `diff --git a/new_file.go b/new_file.go
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/new_file.go
@@ -0,0 +1,3 @@
+package main
+
+func newFunc() {}
`

func TestParseGitDiff_NewFile(t *testing.T) {
	changes, err := ParseGitDiff(newFileDiff)
	require.NoError(t, err)
	require.Len(t, changes, 1)

	fc := changes[0]
	assert.True(t, fc.IsNew)
	assert.Equal(t, "new_file.go", fc.NewName)
	assert.Equal(t, 3, fc.Stats.Additions)
}

const deletedFileDiff = `diff --git a/old_file.go b/old_file.go
deleted file mode 100644
index 1234567..0000000
--- a/old_file.go
+++ /dev/null
@@ -1,3 +0,0 @@
-package main
-
-func oldFunc() {}
`

func TestParseGitDiff_DeletedFile(t *testing.T) {
	changes, err := ParseGitDiff(deletedFileDiff)
	require.NoError(t, err)
	require.Len(t, changes, 1)

	fc := changes[0]
	assert.True(t, fc.IsDeleted)
	assert.Equal(t, "old_file.go", fc.OldName)
	assert.Equal(t, 3, fc.Stats.Deletions)
}

const renameDiff = `diff --git a/old_name.go b/new_name.go
similarity index 100%
rename from old_name.go
rename to new_name.go
`

func TestParseGitDiff_Rename(t *testing.T) {
	changes, err := ParseGitDiff(renameDiff)
	require.NoError(t, err)
	require.Len(t, changes, 1)

	fc := changes[0]
	assert.True(t, fc.IsRenamed)
	assert.Equal(t, "old_name.go", fc.OldName)
	assert.Equal(t, "new_name.go", fc.NewName)
}

func TestLineNumberMapping(t *testing.T) {
	changes, err := ParseGitDiff(sampleDiff)
	require.NoError(t, err)
	require.Len(t, changes, 1)

	fc := changes[0]
	require.NotEmpty(t, fc.Hunks)

	// Check that added lines have NewLineNo set
	for _, h := range fc.Hunks {
		for _, l := range h.Lines {
			if l.Type == LineAdded {
				assert.Greater(t, l.NewLineNo, 0, "added lines should have NewLineNo")
			}
			if l.Type == LineDeleted {
				assert.Greater(t, l.OldLineNo, 0, "deleted lines should have OldLineNo")
			}
		}
	}
}

func TestParseGitLabDiffs(t *testing.T) {
	diffs := []GitLabDiff{
		{
			OldPath: "main.go",
			NewPath: "main.go",
			Diff:    "@@ -1,3 +1,4 @@\n package main\n \n+import \"os\"\n func main() {}\n",
		},
	}

	changes, err := ParseGitLabDiffs(diffs)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, "main.go", changes[0].NewName)
}

func TestParseGitLabDiffs_MarksPDFAsBinary(t *testing.T) {
	diffs := []GitLabDiff{
		{
			OldPath: "docs/spec.pdf",
			NewPath: "docs/spec.pdf",
			Diff:    "@@ -0,0 +0,0 @@\n",
		},
	}
	changes, err := ParseGitLabDiffs(diffs)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.True(t, changes[0].IsBinary)
}

func TestFormatForReview(t *testing.T) {
	changes := []FileChange{
		{
			NewName: "main.go",
			Stats:   DiffStats{Additions: 2, Deletions: 1},
			Hunks: []Hunk{
				{
					OldStart: 1, OldLines: 3, NewStart: 1, NewLines: 4,
					Lines: []DiffLine{
						{Type: LineContext, Content: "package main", OldLineNo: 1, NewLineNo: 1},
						{Type: LineAdded, Content: "import \"os\"", NewLineNo: 2},
						{Type: LineDeleted, Content: "import \"fmt\"", OldLineNo: 2},
					},
				},
			},
		},
	}

	formatted := FormatForReview(changes)
	assert.Contains(t, formatted, "main.go")
	assert.Contains(t, formatted, "+import \"os\"")
	assert.Contains(t, formatted, "-import \"fmt\"")
}

func TestFormatForReview_SkipsBinaryFiles(t *testing.T) {
	changes := []FileChange{
		{
			NewName:  "docs/spec.pdf",
			IsBinary: true,
		},
		{
			NewName: "main.go",
			Stats:   DiffStats{Additions: 1, Deletions: 0},
			Hunks: []Hunk{
				{
					OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 2,
					Lines: []DiffLine{
						{Type: LineAdded, Content: "fmt.Println(\"ok\")", NewLineNo: 2},
					},
				},
			},
		},
	}
	formatted := FormatForReview(changes)
	assert.NotContains(t, formatted, "spec.pdf")
	assert.Contains(t, formatted, "main.go")
}

func TestFilterTextChanges(t *testing.T) {
	changes := []FileChange{
		{NewName: "docs/spec.pdf", IsBinary: true},
		{NewName: "main.go", IsBinary: false},
	}
	got := FilterTextChanges(changes)
	require.Len(t, got, 1)
	assert.Equal(t, "main.go", got[0].NewName)
}
