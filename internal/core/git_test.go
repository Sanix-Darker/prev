package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupGitRepo creates a temporary git repo with a main branch and a feature branch.
func setupGitRepo(t *testing.T) (repoPath string) {
	t.Helper()

	dir, err := os.MkdirTemp("", "prev-git-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v failed: %s", args, string(out))
	}

	run("init", "-b", "main")

	// Create initial file on main
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.go"), []byte("package main\n\nfunc hello() {}\n"), 0644))
	run("add", "hello.go")
	run("commit", "-m", "initial commit")

	// Create feature branch with changes
	run("checkout", "-b", "feature")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.go"), []byte("package main\n\nfunc hello() { println(\"hi\") }\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "new_file.go"), []byte("package main\n\nfunc newFunc() {}\n"), 0644))
	run("add", "hello.go", "new_file.go")
	run("commit", "-m", "add greeting and new file")
	run("commit", "--allow-empty", "-m", "second feature commit")

	// Go back to main for a clean state
	run("checkout", "main")

	return dir
}

func TestGetFileContent_ExistingFile(t *testing.T) {
	repoPath := setupGitRepo(t)

	content, err := GetFileContent(repoPath, "feature", "hello.go")
	require.NoError(t, err)
	assert.Contains(t, content, "println")
}

func TestGetFileContent_MissingFile(t *testing.T) {
	repoPath := setupGitRepo(t)

	content, err := GetFileContent(repoPath, "feature", "nonexistent.go")
	assert.NoError(t, err)
	assert.Equal(t, "", content)
}

func TestGetFileContent_AtDifferentRefs(t *testing.T) {
	repoPath := setupGitRepo(t)

	mainContent, err := GetFileContent(repoPath, "main", "hello.go")
	require.NoError(t, err)

	featureContent, err := GetFileContent(repoPath, "feature", "hello.go")
	require.NoError(t, err)

	assert.NotEqual(t, mainContent, featureContent, "content should differ between main and feature")
	assert.NotContains(t, mainContent, "println")
	assert.Contains(t, featureContent, "println")
}

func TestGetCommitList_BetweenBranches(t *testing.T) {
	repoPath := setupGitRepo(t)

	commits, err := GetCommitList(repoPath, "main", "feature")
	require.NoError(t, err)
	assert.Len(t, commits, 2)

	// Verify commits have hash and subject
	for _, c := range commits {
		assert.NotEmpty(t, c.Hash)
		assert.NotEmpty(t, c.Subject)
	}
}

func TestGetCommitList_EmptyRange(t *testing.T) {
	repoPath := setupGitRepo(t)

	commits, err := GetCommitList(repoPath, "main", "main")
	require.NoError(t, err)
	assert.Empty(t, commits)
}

func TestGetDiffStat_BetweenBranches(t *testing.T) {
	repoPath := setupGitRepo(t)

	stat, err := GetDiffStat(repoPath, "main", "feature")
	require.NoError(t, err)
	assert.NotEmpty(t, stat)
	assert.Contains(t, stat, "hello.go")
}

func TestGetBaseBranch_Default(t *testing.T) {
	repoPath := setupGitRepo(t)

	// No origin/HEAD configured, should return "main"
	branch := GetBaseBranch(repoPath)
	assert.Equal(t, "main", branch)
}

func TestGetGitDiffForBranch(t *testing.T) {
	repoPath := setupGitRepo(t)

	diff, err := GetGitDiffForBranch(repoPath, "main", "feature")
	require.NoError(t, err)
	assert.NotEmpty(t, diff)
	assert.Contains(t, diff, "hello.go")
}

func TestGetGitDiffForRefs(t *testing.T) {
	repoPath := setupGitRepo(t)

	diff, err := GetGitDiffForRefs(repoPath, "main", "feature")
	require.NoError(t, err)
	assert.NotEmpty(t, diff)
	assert.Contains(t, diff, "hello.go")
}

func TestGetGitDiffForBranch_SameBranch(t *testing.T) {
	repoPath := setupGitRepo(t)

	diff, err := GetGitDiffForBranch(repoPath, "main", "main")
	require.NoError(t, err)
	assert.Empty(t, diff)
}

func TestGetCommitMessage(t *testing.T) {
	repoPath := setupGitRepo(t)

	msg, err := GetCommitMessage(repoPath, "feature")
	require.NoError(t, err)
	assert.Contains(t, msg, "second feature commit")
}
