package review

import "github.com/sanix-darker/prev/internal/core"

// ReviewConfig holds configuration for the branch review pipeline.
type ReviewConfig struct {
	ContextLines   int    // default 10
	MaxBatchTokens int    // default 80000
	Strictness     string // "strict"/"normal"/"lenient"
	CommitByCommit bool
	SerenaMode     string // "auto"/"on"/"off"
	Debug          bool
}

// DefaultReviewConfig returns a ReviewConfig with sensible defaults.
func DefaultReviewConfig() ReviewConfig {
	return ReviewConfig{
		ContextLines:   10,
		MaxBatchTokens: 80000,
		Strictness:     "normal",
		SerenaMode:     "auto",
	}
}

// BranchReviewResult is the complete output of a branch review.
type BranchReviewResult struct {
	BranchName     string
	BaseBranch     string
	Walkthrough    WalkthroughResult
	FileReviews    []FileReviewResult
	TotalFiles     int
	TotalAdditions int
	TotalDeletions int
}

// WalkthroughResult holds the pass-1 walkthrough summary.
type WalkthroughResult struct {
	Summary      string
	ChangesTable string
	RawContent   string
}

// FileReviewResult holds the review output for a single file.
type FileReviewResult struct {
	FilePath string
	Comments []core.FileComment
	Summary  string
}
