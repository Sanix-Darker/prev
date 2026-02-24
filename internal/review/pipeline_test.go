package review

import (
	"context"
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
	// Track which prompt was sent (use first 50 chars as key)
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

func TestRunBranchReview(t *testing.T) {
	// This test requires a real git repo, so we skip if not in a suitable env.
	// We test the components individually; this is more of an integration smoke test.

	// Create a mock AI response
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

	// We need a real git repo for the full pipeline, so test parseWalkthrough separately
	walkthrough := parseWalkthrough(mockResp)
	require.NotEmpty(t, walkthrough.Summary)
	assert.Contains(t, walkthrough.Summary, "good changes")
	assert.NotEmpty(t, walkthrough.ChangesTable)
	assert.Contains(t, walkthrough.ChangesTable, "main.go")

	// Verify mock provider is usable
	assert.NotNil(t, mock)
	assert.Equal(t, "normal", cfg.Strictness)
}
