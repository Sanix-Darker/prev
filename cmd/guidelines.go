package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sanix-darker/prev/internal/core"
	"github.com/sanix-darker/prev/internal/guidelines"
)

func mergeGuidelines(parts ...string) string {
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, "\n\n")
}

func guidelineRootFromRepoPath(repoPath string) string {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" || repoPath == "." {
		if wd, err := os.Getwd(); err == nil {
			return wd
		}
		return "."
	}
	if abs, err := filepath.Abs(repoPath); err == nil {
		return abs
	}
	return repoPath
}

func guidelineRootForMR() string {
	if ciDir := strings.TrimSpace(os.Getenv("CI_PROJECT_DIR")); ciDir != "" {
		return ciDir
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

func guidelineRootForDiffInput(input string) string {
	parts := strings.Split(strings.ReplaceAll(input, " ", ""), ",")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		if wd, err := os.Getwd(); err == nil {
			return wd
		}
		return "."
	}
	first := parts[0]
	if abs, err := filepath.Abs(first); err == nil {
		if info, err := os.Stat(abs); err == nil {
			if info.IsDir() {
				return abs
			}
			return filepath.Dir(abs)
		}
		return filepath.Dir(abs)
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

func repoGuidelineSection(root string) string {
	return guidelines.BuildPromptSection(root)
}

func commitMessageContextBlock(repoPath, commitHash string) string {
	msg, err := core.GetCommitMessage(repoPath, commitHash)
	if err != nil || strings.TrimSpace(msg) == "" {
		return ""
	}
	return fmt.Sprintf("## Change Intent Context\nCommit message:\n```text\n%s\n```", strings.TrimSpace(msg))
}

func branchCommitContextBlock(repoPath, baseBranch, targetBranch string) string {
	commits, err := core.GetCommitList(repoPath, baseBranch, targetBranch)
	if err != nil || len(commits) == 0 {
		return ""
	}
	const maxCommits = 12
	if len(commits) > maxCommits {
		commits = commits[:maxCommits]
	}
	var b strings.Builder
	b.WriteString("## Change Intent Context\nBranch commit subjects (most recent first):\n")
	for _, c := range commits {
		b.WriteString("- ")
		b.WriteString(c.Subject)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}
