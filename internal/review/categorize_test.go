package review

import (
	"testing"

	"github.com/sanix-darker/prev/internal/diffparse"
	"github.com/stretchr/testify/assert"
)

func TestCategorizeChanges(t *testing.T) {
	changes := []diffparse.EnrichedFileChange{
		{FileChange: diffparse.FileChange{NewName: "new.go", IsNew: true}},
		{FileChange: diffparse.FileChange{OldName: "old.go", NewName: "old.go"}},
		{FileChange: diffparse.FileChange{OldName: "del.go", IsDeleted: true}},
		{FileChange: diffparse.FileChange{OldName: "a.go", NewName: "b.go", IsRenamed: true}},
		{FileChange: diffparse.FileChange{NewName: "img.png", IsBinary: true}},
	}

	result := CategorizeChanges(changes)
	assert.Len(t, result, 5)
	assert.Equal(t, "New", result[0].Category)
	assert.Equal(t, "Modified", result[1].Category)
	assert.Equal(t, "Deleted", result[2].Category)
	assert.Equal(t, "Renamed", result[3].Category)
	assert.Equal(t, "Binary", result[4].Category)
}

func TestGroupDetection(t *testing.T) {
	tests := []struct {
		name  string
		efc   diffparse.EnrichedFileChange
		group string
	}{
		{"test file", diffparse.EnrichedFileChange{FileChange: diffparse.FileChange{NewName: "internal/foo/bar_test.go"}}, "tests"},
		{"cmd file", diffparse.EnrichedFileChange{FileChange: diffparse.FileChange{NewName: "cmd/branch.go"}}, "commands"},
		{"internal file", diffparse.EnrichedFileChange{FileChange: diffparse.FileChange{NewName: "internal/core/git.go"}}, "core"},
		{"docs file", diffparse.EnrichedFileChange{FileChange: diffparse.FileChange{NewName: "docs/README.md"}}, "docs"},
		{"go.mod", diffparse.EnrichedFileChange{FileChange: diffparse.FileChange{NewName: "go.mod"}}, "dependencies"},
		{"Dockerfile", diffparse.EnrichedFileChange{FileChange: diffparse.FileChange{NewName: "Dockerfile"}}, "ci/config"},
		{"github actions", diffparse.EnrichedFileChange{FileChange: diffparse.FileChange{NewName: ".github/workflows/ci.yml"}}, "ci/config"},
		{"root md", diffparse.EnrichedFileChange{FileChange: diffparse.FileChange{NewName: "CHANGELOG.md"}}, "docs"},
		{"other file", diffparse.EnrichedFileChange{FileChange: diffparse.FileChange{NewName: "main.go"}}, "other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CategorizeChanges([]diffparse.EnrichedFileChange{tt.efc})
			assert.Equal(t, tt.group, result[0].Group)
		})
	}
}
