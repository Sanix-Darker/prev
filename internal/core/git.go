package core

import (
	"fmt"
	"os/exec"
	"strings"
)

// GetGitDiffForBranch returns the diff between baseBranch and targetBranch.
func GetGitDiffForBranch(repoPath, baseBranch, targetBranch string) (string, error) {
	diffRange := fmt.Sprintf("%s...%s", baseBranch, targetBranch)
	args := []string{"diff", diffRange}
	return runGitDiff(repoPath, args)
}

// GetGitDiffForCommit returns the diff for a single commit.
func GetGitDiffForCommit(repoPath, commitHash string) (string, error) {
	args := []string{"show", "--format=", commitHash}
	return runGitDiff(repoPath, args)
}

func runGitDiff(repoPath string, args []string) (string, error) {
	fullArgs := append([]string{"-C", repoPath}, args...)
	cmd := exec.Command("git", fullArgs...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git command failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git command failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CommitInfo holds a commit hash and subject line.
type CommitInfo struct {
	Hash    string
	Subject string
}

// GetFileContent returns the content of a file at a specific git ref.
// Returns "" and nil for missing files (new/deleted files).
func GetFileContent(repoPath, ref, filePath string) (string, error) {
	refPath := fmt.Sprintf("%s:%s", ref, filePath)
	cmd := exec.Command("git", "-C", repoPath, "show", refPath)
	out, err := cmd.Output()
	if err != nil {
		// Missing file is not an error (new or deleted file)
		return "", nil
	}
	return string(out), nil
}

// GetCommitList returns the list of commits between baseBranch and targetBranch.
func GetCommitList(repoPath, baseBranch, targetBranch string) ([]CommitInfo, error) {
	commitRange := fmt.Sprintf("%s..%s", baseBranch, targetBranch)
	cmd := exec.Command("git", "-C", repoPath, "log", "--format=%H|%s", commitRange)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git log failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	var commits []CommitInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 {
			commits = append(commits, CommitInfo{Hash: parts[0], Subject: parts[1]})
		}
	}
	return commits, nil
}

// GetDiffStat returns the diff stat summary between baseBranch and targetBranch.
func GetDiffStat(repoPath, baseBranch, targetBranch string) (string, error) {
	diffRange := fmt.Sprintf("%s...%s", baseBranch, targetBranch)
	cmd := exec.Command("git", "-C", repoPath, "diff", "--stat", diffRange)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git diff --stat failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git diff --stat failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GetDiffStatForCommit returns the diff stat summary for a single commit.
func GetDiffStatForCommit(repoPath, commitHash string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "show", "--stat", "--format=", commitHash)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git show --stat failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git show --stat failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GetBaseBranch tries to determine the base branch of the repo (main or master).
func GetBaseBranch(repoPath string) string {
	cmd := exec.Command("git", "-C", repoPath, "symbolic-ref", "refs/remotes/origin/HEAD", "--short")
	out, err := cmd.Output()
	if err != nil {
		return "main"
	}
	branch := strings.TrimSpace(string(out))
	// e.g. "origin/main" -> "main"
	parts := strings.SplitN(branch, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return branch
}
