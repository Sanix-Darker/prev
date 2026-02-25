package handlers

import (
	"fmt"
	"strings"

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
	if gitPath != "" && gitPath != "." {
		cfg.PathFilter = gitPath
	}

	return review.RunBranchReview(aiProvider, repoPath, branchName, baseBranch, cfg, onProgress)
}

// ExtractBranchReviewPerCommit runs the review pipeline per commit.
func ExtractBranchReviewPerCommit(
	aiProvider provider.AIProvider,
	branchName string,
	repoPath string,
	gitPath string,
	cfg review.ReviewConfig,
	onProgress review.ProgressCallback,
) (*review.MultiCommitReviewResult, error) {
	if repoPath == "" || repoPath == "." {
		repoPath = "."
	}

	baseBranch := core.GetBaseBranch(repoPath)
	if gitPath != "" && gitPath != "." {
		cfg.PathFilter = gitPath
	}

	commits, err := core.GetCommitList(repoPath, baseBranch, branchName)
	if err != nil {
		return nil, fmt.Errorf("failed to list commits: %w", err)
	}
	if len(commits) == 0 {
		return nil, fmt.Errorf("no commits found between %s and %s", baseBranch, branchName)
	}

	// Oldest first for review readability.
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}

	var results []review.CommitReviewResult
	for i, c := range commits {
		if onProgress != nil {
			onProgress("Reviewing commit", i+1, len(commits))
		}
		res, err := review.RunCommitReview(aiProvider, repoPath, c.Hash, c.Subject, cfg, onProgress)
		if err != nil {
			return nil, err
		}
		results = append(results, *res)
	}

	return &review.MultiCommitReviewResult{
		BranchName: branchName,
		BaseBranch: baseBranch,
		Commits:    results,
	}, nil
}

// filterDiffByPath filters a unified diff to only include files matching the given path prefix.
func filterDiffByPath(diff string, pathFilter string) string {
	var result []string
	var include bool

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git") {
			include = strings.Contains(line, pathFilter)
		}
		if include {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}
