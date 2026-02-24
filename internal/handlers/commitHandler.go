package handlers

import (
	"fmt"
	"strings"

	"github.com/sanix-darker/prev/internal/config"
	"github.com/sanix-darker/prev/internal/core"
)

// ExtractCommitHandler gets the diff for a specific commit.
func ExtractCommitHandler(
	conf config.Config,
	commitHash string,
	repoPath string,
	gitPath string,
	help func() error,
) ([]string, error) {
	if repoPath == "" || repoPath == "." {
		repoPath = "."
	}

	diff, err := core.GetGitDiffForCommit(repoPath, commitHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit diff: %w", err)
	}

	if diff == "" {
		return nil, fmt.Errorf("no differences found for commit %s", commitHash)
	}

	// If gitPath filter is set, filter the diff
	if gitPath != "" && gitPath != "." {
		diff = filterCommitDiffByPath(diff, gitPath)
	}

	return []string{diff}, nil
}

// filterCommitDiffByPath filters a unified diff to only include files matching the given path prefix.
func filterCommitDiffByPath(diff string, pathFilter string) string {
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
