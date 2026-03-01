package handlers

import (
	"fmt"

	"github.com/sanix-darker/prev/internal/core"
	"github.com/sanix-darker/prev/internal/provider"
	"github.com/sanix-darker/prev/internal/review"
)

// ExtractBranchHandler gets the diff for a branch compared to the base branch.
func ExtractBranchHandler(
	branchName string,
	repoPath string,
	gitPath string,
	help func() error,
) ([]string, error) {
	if repoPath == "" || repoPath == "." {
		repoPath = "."
	}

	baseBranch := core.GetBaseBranch(repoPath)

	diff, err := core.GetGitDiffForBranch(repoPath, baseBranch, branchName)
	if err != nil {
		return nil, fmt.Errorf("failed to get branch diff: %w", err)
	}

	if diff == "" {
		return nil, fmt.Errorf("no differences found between %s and %s", baseBranch, branchName)
	}

	// If gitPath filter is set, filter the diff
	if gitPath != "" && gitPath != "." {
		diff = filterDiffByPath(diff, gitPath)
	}

	return []string{diff}, nil
}

// ExtractBranchReview runs the new two-pass review pipeline.
func ExtractBranchReview(
	aiProvider provider.AIProvider,
	branchName string,
	repoPath string,
	gitPath string,
	cfg review.ReviewConfig,
	onProgress review.ProgressCallback,
) (*review.BranchReviewResult, error) {
	if repoPath == "" || repoPath == "." {
		repoPath = "."
	}

	baseBranch := core.GetBaseBranch(repoPath)

	return review.RunBranchReview(aiProvider, repoPath, branchName, baseBranch, cfg, onProgress)
}

// filterDiffByPath filters a unified diff to only include files matching the given path prefix.
func filterDiffByPath(diff string, pathFilter string) string {
	return filterUnifiedDiffByPath(diff, pathFilter)
}
