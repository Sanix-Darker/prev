package handlers

import (
	"fmt"
	"strings"

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

type MRExtractOptions struct {
	DiffSource string // auto|git|raw|api
	RepoPath   string
}

// ExtractMRHandler fetches MR details and diffs, then builds a review prompt.
func ExtractMRHandler(
	provider vcs.VCSProvider,
	projectID string,
	mrIID int64,
	strictness string,
) (*MRReview, error) {
	return ExtractMRHandlerWithOptions(provider, projectID, mrIID, strictness, MRExtractOptions{})
}

func ExtractMRHandlerWithOptions(
	provider vcs.VCSProvider,
	projectID string,
	mrIID int64,
	strictness string,
	opts MRExtractOptions,
) (*MRReview, error) {
	// Fetch MR details
	mr, err := provider.FetchMR(projectID, mrIID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch MR: %w", err)
	}

	changes, err := extractMRChanges(provider, projectID, mrIID, mr, opts)
	if err != nil {
		return nil, err
	}
	changes = diffparse.FilterTextChanges(changes)
	if !hasAnyTextHunks(changes) {
		return nil, fmt.Errorf("no reviewable modified hunks found in MR context (diff source=%s)", normalizeDiffSource(opts.DiffSource))
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

func extractMRChanges(
	provider vcs.VCSProvider,
	projectID string,
	mrIID int64,
	mr *vcs.MergeRequest,
	opts MRExtractOptions,
) ([]diffparse.FileChange, error) {
	source := normalizeDiffSource(opts.DiffSource)

	if source == "git" || source == "auto" {
		if strings.TrimSpace(opts.RepoPath) != "" &&
			strings.TrimSpace(mr.DiffRefs.BaseSHA) != "" &&
			strings.TrimSpace(mr.DiffRefs.HeadSHA) != "" {
			raw, err := core.GetGitDiffForRefs(opts.RepoPath, mr.DiffRefs.BaseSHA, mr.DiffRefs.HeadSHA)
			if err == nil && strings.TrimSpace(raw) != "" {
				changes, perr := diffparse.ParseGitDiff(raw)
				if perr == nil {
					return changes, nil
				}
			}
			if source == "git" {
				return nil, fmt.Errorf("failed to build MR changes from local git refs %s...%s", mr.DiffRefs.BaseSHA, mr.DiffRefs.HeadSHA)
			}
		}
	}

	if source == "raw" || source == "auto" {
		raw, err := provider.FetchMRRawDiff(projectID, mrIID)
		if err == nil && strings.TrimSpace(raw) != "" {
			changes, perr := diffparse.ParseGitDiff(raw)
			if perr == nil {
				return changes, nil
			}
		}
		if source == "raw" {
			return nil, fmt.Errorf("failed to build MR changes from raw_diffs endpoint")
		}
	}

	// Legacy API fallback
	mrDiffs, err := provider.FetchMRDiffs(projectID, mrIID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch MR diffs: %w", err)
	}
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
	return changes, nil
}

func normalizeDiffSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "git", "raw", "api":
		return strings.ToLower(strings.TrimSpace(source))
	default:
		return "auto"
	}
}

func hasAnyTextHunks(changes []diffparse.FileChange) bool {
	for _, c := range changes {
		if c.IsBinary {
			continue
		}
		for _, h := range c.Hunks {
			if len(h.Lines) > 0 {
				return true
			}
		}
	}
	return false
}
