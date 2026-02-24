package handlers

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sanix-darker/prev/internal/provider"
	"github.com/sanix-darker/prev/internal/review"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAIProvider implements provider.AIProvider for testing.
type mockAIProvider struct {
	response string
	calls    int
}

func (m *mockAIProvider) Info() provider.ProviderInfo {
	return provider.ProviderInfo{Name: "mock", DisplayName: "Mock"}
}

func (m *mockAIProvider) Complete(_ context.Context, _ provider.CompletionRequest) (*provider.CompletionResponse, error) {
	m.calls++
	return &provider.CompletionResponse{
		Content: m.response,
		Choices: []provider.Choice{{Content: m.response}},
	}, nil
}

func (m *mockAIProvider) CompleteStream(_ context.Context, _ provider.CompletionRequest) provider.StreamResult {
	ch := make(chan provider.StreamChunk, 1)
	errCh := make(chan error, 1)
	ch <- provider.StreamChunk{Content: m.response, Done: true}
	close(ch)
	close(errCh)
	return provider.StreamResult{Chunks: ch, Err: errCh}
}

func (m *mockAIProvider) Validate(_ context.Context) error { return nil }

func setupHandlerRepo(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "prev-handler-test-*")
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

	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "utils.go"), []byte("package main\n\nfunc util() {}\n"), 0644))
	run("add", ".")
	run("commit", "-m", "initial")

	run("checkout", "-b", "feature")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hi\") }\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "utils.go"), []byte("package main\n\nfunc util() { return }\n"), 0644))
	run("add", ".")
	run("commit", "-m", "update files")

	run("checkout", "main")

	return dir
}

func TestExtractBranchReview_Success(t *testing.T) {
	repoPath := setupHandlerRepo(t)

	mock := &mockAIProvider{
		response: "## Summary\nGood changes.\n\n**main.go:5** [MEDIUM]: Consider error handling.\n",
	}

	cfg := review.ReviewConfig{
		ContextLines:   3,
		MaxBatchTokens: 80000,
		Strictness:     "normal",
		SerenaMode:     "off",
	}

	result, err := ExtractBranchReview(mock, "feature", repoPath, "", cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have made 2 AI calls (walkthrough + review)
	assert.Equal(t, 2, mock.calls)
	assert.Equal(t, "feature", result.BranchName)
	assert.Greater(t, result.TotalFiles, 0)
}

func TestExtractBranchReview_NoDiff(t *testing.T) {
	repoPath := setupHandlerRepo(t)

	mock := &mockAIProvider{response: "no issues"}
	cfg := review.ReviewConfig{
		ContextLines:   3,
		MaxBatchTokens: 80000,
		Strictness:     "normal",
		SerenaMode:     "off",
	}

	// main vs main = no diff
	_, err := ExtractBranchReview(mock, "main", repoPath, "", cfg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no differences")
}

func TestFilterDiffByPath_Matches(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,3 +1,3 @@
-old main
+new main
diff --git a/utils.go b/utils.go
--- a/utils.go
+++ b/utils.go
@@ -1,3 +1,3 @@
-old utils
+new utils`

	filtered := filterDiffByPath(diff, "main.go")
	assert.Contains(t, filtered, "main.go")
	assert.NotContains(t, filtered, "utils.go")
}

func TestFilterDiffByPath_NoMatch(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,3 +1,3 @@
-old
+new`

	filtered := filterDiffByPath(diff, "nonexistent.go")
	assert.Empty(t, filtered)
}

func TestExtractBranchHandler_Success(t *testing.T) {
	repoPath := setupHandlerRepo(t)

	result, err := ExtractBranchHandler("feature", repoPath, "", nil)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.NotEmpty(t, result[0])
}

func TestExtractBranchHandler_NoDiff(t *testing.T) {
	repoPath := setupHandlerRepo(t)

	_, err := ExtractBranchHandler("main", repoPath, "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no differences")
}
