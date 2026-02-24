package review

import (
	"testing"

	"github.com/sanix-darker/prev/internal/diffparse"
	"github.com/stretchr/testify/assert"
)

func TestBuildWalkthroughPrompt(t *testing.T) {
	files := []CategorizedFile{
		{
			EnrichedFileChange: diffparse.EnrichedFileChange{
				FileChange: diffparse.FileChange{
					NewName: "main.go",
					Stats:   diffparse.DiffStats{Additions: 10, Deletions: 2},
				},
			},
			Category: "Modified",
			Group:    "core",
		},
	}

	prompt := BuildWalkthroughPrompt("feature", "main", files, "1 file changed, 10 insertions(+)", "normal")

	assert.Contains(t, prompt, "feature")
	assert.Contains(t, prompt, "main")
	assert.Contains(t, prompt, "| File |")
	assert.Contains(t, prompt, "main.go")
	assert.Contains(t, prompt, "NORMAL")
	assert.Contains(t, prompt, "Summary")
	assert.Contains(t, prompt, "Changes Table")
}

func TestBuildFileReviewPrompt(t *testing.T) {
	batch := FileBatch{
		Files: []CategorizedFile{
			{
				EnrichedFileChange: diffparse.EnrichedFileChange{
					FileChange: diffparse.FileChange{
						NewName: "handler.go",
						Stats:   diffparse.DiffStats{Additions: 5, Deletions: 1},
					},
					Language: "go",
				},
			},
		},
	}

	prompt := BuildFileReviewPrompt(batch, "This branch adds auth.", "feature", "strict")

	assert.Contains(t, prompt, "Walkthrough Context")
	assert.Contains(t, prompt, "This branch adds auth.")
	assert.Contains(t, prompt, "handler.go")
	assert.Contains(t, prompt, "STRICT")
	assert.Contains(t, prompt, "suggestion")
	assert.Contains(t, prompt, "SEVERITY")
}
