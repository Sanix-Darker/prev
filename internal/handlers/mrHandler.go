package handlers

import (
	"fmt"

	"github.com/sanix-darker/prev/internal/core"
	"github.com/sanix-darker/prev/internal/diffparse"
	"github.com/sanix-darker/prev/internal/vcs"
)

// MRReview holds the complete review result for a merge request.
type MRReview struct {
	MR      *vcs.MergeRequest
	Changes []diffparse.FileChange
	Prompt  string
}

// ExtractMRHandler fetches MR details and diffs, then builds a review prompt.
func ExtractMRHandler(
	provider vcs.VCSProvider,
	projectID string,
	mrIID int64,
	strictness string,
) (*MRReview, error) {
	// Fetch MR details
	mr, err := provider.FetchMR(projectID, mrIID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch MR: %w", err)
	}

	// Fetch MR diffs
	mrDiffs, err := provider.FetchMRDiffs(projectID, mrIID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch MR diffs: %w", err)
	}

	// Convert VCS diffs to diffparse format
	var glDiffs []diffparse.GitLabDiff
	for _, d := range mrDiffs {
		glDiffs = append(glDiffs, diffparse.GitLabDiff{
			OldPath:     d.OldPath,
			NewPath:     d.NewPath,
			Diff:        d.Diff,
			NewFile:     d.NewFile,
			RenamedFile: d.RenamedFile,
			DeletedFile: d.DeletedFile,
		})
	}

	changes, err := diffparse.ParseGitLabDiffs(glDiffs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse diffs: %w", err)
	}

	// Format diffs for AI review
	formattedDiffs := diffparse.FormatForReview(changes)

	// Build the review prompt
	prompt := core.BuildMRReviewPrompt(
		mr.Title,
		mr.Description,
		mr.SourceBranch,
		mr.TargetBranch,
		formattedDiffs,
		strictness,
	)

	return &MRReview{
		MR:      mr,
		Changes: changes,
		Prompt:  prompt,
	}, nil
}
