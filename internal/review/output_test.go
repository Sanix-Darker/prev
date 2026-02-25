package review

import (
	"strings"
	"testing"

	"github.com/sanix-darker/prev/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestFormatBranchReview_FullResult(t *testing.T) {
	result := &BranchReviewResult{
		BranchName: "feature",
		BaseBranch: "main",
		Walkthrough: WalkthroughResult{
			Summary:      "This branch adds authentication.",
			ChangesTable: "| File | Type | Summary |\n|------|------|---------|\n| auth.go | New | JWT auth |",
		},
		FileReviews: []FileReviewResult{
			{
				FilePath: "auth.go",
				Comments: []core.FileComment{
					{FilePath: "auth.go", Line: 10, Severity: "HIGH", Message: "Hardcoded secret"},
					{FilePath: "auth.go", Line: 25, Severity: "MEDIUM", Message: "Missing error check"},
				},
			},
			{
				FilePath: "main.go",
				Summary:  "No significant issues found.",
			},
		},
		TotalFiles:     2,
		TotalAdditions: 100,
		TotalDeletions: 5,
	}

	output := FormatBranchReview(result)

	// Check all sections present
	assert.Contains(t, output, "# Branch Review: feature")
	assert.Contains(t, output, "## Walkthrough")
	assert.Contains(t, output, "This branch adds authentication.")
	assert.Contains(t, output, "## Changes")
	assert.Contains(t, output, "## Detailed Review")
	assert.Contains(t, output, "## Statistics")

	// Check severity counts
	assert.Contains(t, output, "Issues: 2 (1 HIGH, 1 MEDIUM)")
	assert.Contains(t, output, "Changes: +100/-5")
	assert.Contains(t, output, "Files reviewed: 2")
}

func TestFormatBranchReview_NoIssues(t *testing.T) {
	result := &BranchReviewResult{
		BranchName: "clean-branch",
		BaseBranch: "main",
		Walkthrough: WalkthroughResult{
			Summary: "Clean changes.",
		},
		FileReviews: []FileReviewResult{
			{FilePath: "clean.go"},
		},
		TotalFiles:     1,
		TotalAdditions: 10,
		TotalDeletions: 2,
	}

	output := FormatBranchReview(result)

	assert.Contains(t, output, "Issues: 0")
	assert.Contains(t, output, "No significant issues found.")
}

func TestFormatBranchReview_WithSuggestion(t *testing.T) {
	result := &BranchReviewResult{
		BranchName: "feature",
		BaseBranch: "main",
		FileReviews: []FileReviewResult{
			{
				FilePath: "handler.go",
				Comments: []core.FileComment{
					{
						FilePath:   "handler.go",
						Line:       42,
						Severity:   "MEDIUM",
						Message:    "Use errors.New",
						Suggestion: "return errors.New(\"invalid input\")",
					},
				},
			},
		},
		TotalFiles: 1,
	}

	output := FormatBranchReview(result)

	assert.Contains(t, output, "```suggestion")
	assert.Contains(t, output, "return errors.New(\"invalid input\")")
}

func TestFormatBranchReview_SeverityCounting(t *testing.T) {
	result := &BranchReviewResult{
		BranchName: "feature",
		BaseBranch: "main",
		FileReviews: []FileReviewResult{
			{
				FilePath: "file.go",
				Comments: []core.FileComment{
					{FilePath: "file.go", Line: 1, Severity: "CRITICAL", Message: "SQL injection"},
					{FilePath: "file.go", Line: 2, Severity: "CRITICAL", Message: "Command injection"},
					{FilePath: "file.go", Line: 3, Severity: "HIGH", Message: "Missing auth"},
					{FilePath: "file.go", Line: 4, Severity: "MEDIUM", Message: "Unused var"},
					{FilePath: "file.go", Line: 5, Severity: "LOW", Message: "Style nit"},
				},
			},
		},
		TotalFiles: 1,
	}

	output := FormatBranchReview(result)

	assert.Contains(t, output, "Issues: 5")
	// Verify ordering: CRITICAL, HIGH, MEDIUM, LOW
	critIdx := strings.Index(output, "2 CRITICAL")
	highIdx := strings.Index(output, "1 HIGH")
	medIdx := strings.Index(output, "1 MEDIUM")
	lowIdx := strings.Index(output, "1 LOW")

	assert.Greater(t, critIdx, -1)
	assert.Greater(t, highIdx, critIdx)
	assert.Greater(t, medIdx, highIdx)
	assert.Greater(t, lowIdx, medIdx)
}

func TestFormatMultiCommitReview(t *testing.T) {
	result := &MultiCommitReviewResult{
		BranchName: "feature",
		BaseBranch: "main",
		Commits: []CommitReviewResult{
			{
				CommitHash:    "abc123",
				CommitSubject: "add feature",
				Walkthrough: WalkthroughResult{
					Summary: "Adds a new feature.",
				},
				FileReviews:    []FileReviewResult{{FilePath: "main.go"}},
				TotalFiles:     1,
				TotalAdditions: 10,
				TotalDeletions: 2,
			},
		},
	}

	output := FormatMultiCommitReview(result)

	assert.Contains(t, output, "Branch Review (Per-Commit)")
	assert.Contains(t, output, "Commit abc123 - add feature")
	assert.Contains(t, output, "Walkthrough")
	assert.Contains(t, output, "Statistics")
}
