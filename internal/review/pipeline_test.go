package review

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/sanix-darker/prev/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider implements provider.AIProvider for testing.
type mockProvider struct {
	calls    []string
	response string
}

func (m *mockProvider) Info() provider.ProviderInfo {
	return provider.ProviderInfo{
		Name:             "mock",
		DisplayName:      "Mock",
		SupportsStreaming: false,
	}
}

func (m *mockProvider) Complete(_ context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
	prompt := req.Messages[len(req.Messages)-1].Content
	if len(prompt) > 50 {
		prompt = prompt[:50]
	}
	m.calls = append(m.calls, prompt)

	return &provider.CompletionResponse{
		Content: m.response,
		Choices: []provider.Choice{{Content: m.response}},
	}, nil
}

func (m *mockProvider) CompleteStream(_ context.Context, _ provider.CompletionRequest) provider.StreamResult {
	ch := make(chan provider.StreamChunk, 1)
	errCh := make(chan error, 1)
	ch <- provider.StreamChunk{Content: m.response, Done: true}
	close(ch)
	close(errCh)
	return provider.StreamResult{Chunks: ch, Err: errCh}
}

func (m *mockProvider) Validate(_ context.Context) error {
	return nil
}

// sequentialMockProvider returns different responses for successive calls.
type sequentialMockProvider struct {
	mu        sync.Mutex
	responses []string
	callIdx   int
	calls     []string
}

func (s *sequentialMockProvider) Info() provider.ProviderInfo {
	return provider.ProviderInfo{Name: "sequential-mock", DisplayName: "Sequential Mock"}
}

func (s *sequentialMockProvider) Complete(_ context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prompt := req.Messages[len(req.Messages)-1].Content
	if len(prompt) > 50 {
		prompt = prompt[:50]
	}
	s.calls = append(s.calls, prompt)

	resp := s.responses[0] // default to first
	if s.callIdx < len(s.responses) {
		resp = s.responses[s.callIdx]
	}
	s.callIdx++

	return &provider.CompletionResponse{
		Content: resp,
		Choices: []provider.Choice{{Content: resp}},
	}, nil
}

func (s *sequentialMockProvider) CompleteStream(_ context.Context, _ provider.CompletionRequest) provider.StreamResult {
	ch := make(chan provider.StreamChunk, 1)
	errCh := make(chan error, 1)
	close(ch)
	close(errCh)
	return provider.StreamResult{Chunks: ch, Err: errCh}
}

func (s *sequentialMockProvider) Validate(_ context.Context) error {
	return nil
}

// setupPipelineRepo creates a temp git repo for pipeline tests.
func setupPipelineRepo(t *testing.T) (repoPath string) {
	t.Helper()

	dir, err := os.MkdirTemp("", "prev-pipeline-test-*")
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

	content := "package main\n\nimport \"fmt\"\n\nfunc hello() {\n\tfmt.Println(\"hello\")\n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte(content), 0644))
	run("add", "main.go")
	run("commit", "-m", "initial")

	run("checkout", "-b", "feature")
	newContent := "package main\n\nimport \"fmt\"\n\nfunc hello() {\n\tfmt.Println(\"hello world!\")\n}\n\nfunc greet(name string) {\n\tfmt.Printf(\"Hi %s\\n\", name)\n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte(newContent), 0644))
	run("add", "main.go")
	run("commit", "-m", "update hello and add greet")

	run("checkout", "main")

	return dir
}

func TestRunBranchReview_FullPipeline(t *testing.T) {
	repoPath := setupPipelineRepo(t)

	walkthroughResp := `## Summary
This branch adds a greeting feature and updates the hello function.

## Changes
| File | Type | Summary |
|------|------|---------|
| main.go | Modified | Updated hello output and added greet function |
`
	reviewResp := `## Review

**main.go:6** [MEDIUM]: Consider using fmt.Fprintf for testability.
`

	mock := &sequentialMockProvider{
		responses: []string{walkthroughResp, reviewResp},
	}

	cfg := ReviewConfig{
		ContextLines:   3,
		MaxBatchTokens: 80000,
		Strictness:     "normal",
		SerenaMode:     "off",
	}

	result, err := RunBranchReview(mock, repoPath, "feature", "main", cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify 2 AI calls were made (walkthrough + review)
	assert.Len(t, mock.calls, 2)

	// Verify result structure
	assert.Equal(t, "feature", result.BranchName)
	assert.Equal(t, "main", result.BaseBranch)
	assert.NotEmpty(t, result.Walkthrough.Summary)
	assert.Greater(t, result.TotalFiles, 0)
}

func TestRunBranchReview_EmptyDiff(t *testing.T) {
	repoPath := setupPipelineRepo(t)

	mock := &mockProvider{response: "no issues"}
	cfg := DefaultReviewConfig()
	cfg.SerenaMode = "off"

	// Same branch as base and target
	_, err := RunBranchReview(mock, repoPath, "main", "main", cfg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no differences")
}

func TestRunBranchReview_ProgressCallbacks(t *testing.T) {
	repoPath := setupPipelineRepo(t)

	mock := &sequentialMockProvider{
		responses: []string{"## Summary\nGood changes.\n", "**main.go**: No issues.\n"},
	}

	cfg := ReviewConfig{
		ContextLines:   3,
		MaxBatchTokens: 80000,
		Strictness:     "normal",
		SerenaMode:     "off",
	}

	var stages []string
	onProgress := func(stage string, current, total int) {
		stages = append(stages, stage)
	}

	result, err := RunBranchReview(mock, repoPath, "feature", "main", cfg, onProgress)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have reported progress for multiple stages
	assert.Contains(t, stages, "Getting diff")
	assert.Contains(t, stages, "Parsing diff")
	assert.Contains(t, stages, "Enriching context")
	assert.Contains(t, stages, "AI walkthrough")
	assert.Contains(t, stages, "Reviewing files")
}

func TestParseWalkthrough_NoTable(t *testing.T) {
	content := "This branch makes good improvements to the codebase.\nNo major issues found."
	result := parseWalkthrough(content)

	assert.Contains(t, result.Summary, "good improvements")
	assert.Empty(t, result.ChangesTable)
	assert.Equal(t, content, result.RawContent)
}

func TestParseWalkthrough_EmptyInput(t *testing.T) {
	result := parseWalkthrough("")

	assert.Empty(t, result.Summary)
	assert.Empty(t, result.ChangesTable)
	assert.Equal(t, "", result.RawContent)
}

func TestParseWalkthrough_WithTable(t *testing.T) {
	content := `## Summary
Good changes overall.

## Changes
| File | Type | Summary |
|------|------|---------|
| main.go | Modified | Updated hello function |
| utils.go | New | Added utility helpers |

## Notes
Some additional notes.`

	result := parseWalkthrough(content)

	assert.Contains(t, result.Summary, "Good changes overall")
	assert.Contains(t, result.ChangesTable, "main.go")
	assert.Contains(t, result.ChangesTable, "utils.go")
}

func TestRunBranchReview(t *testing.T) {
	// Smoke test for mock provider + parseWalkthrough
	mockResp := `## Summary
This branch makes good changes.

## Changes
| File | Type | Summary |
|------|------|---------|
| main.go | Modified | Updated hello function |

## Review
**main.go:5** [MEDIUM]: Consider adding error handling.
`

	mock := &mockProvider{response: mockResp}

	cfg := ReviewConfig{
		ContextLines:   3,
		MaxBatchTokens: 80000,
		Strictness:     "normal",
		SerenaMode:     "off",
	}

	walkthrough := parseWalkthrough(mockResp)
	require.NotEmpty(t, walkthrough.Summary)
	assert.Contains(t, walkthrough.Summary, "good changes")
	assert.NotEmpty(t, walkthrough.ChangesTable)
	assert.Contains(t, walkthrough.ChangesTable, "main.go")

	assert.NotNil(t, mock)
	assert.Equal(t, "normal", cfg.Strictness)
}
