package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type scriptedProvider struct {
	requests  []CompletionRequest
	responses []CompletionResponse
}

func (s *scriptedProvider) Info() ProviderInfo {
	return ProviderInfo{Name: "scripted"}
}

func (s *scriptedProvider) Complete(_ context.Context, req CompletionRequest) (*CompletionResponse, error) {
	s.requests = append(s.requests, req)
	idx := len(s.requests) - 1
	resp := CompletionResponse{Content: "ok", Choices: []Choice{{Content: "ok"}}}
	if idx < len(s.responses) {
		resp = s.responses[idx]
	}
	if len(resp.Choices) == 0 && resp.Content != "" {
		resp.Choices = []Choice{{Content: resp.Content}}
	}
	return &resp, nil
}

func (s *scriptedProvider) CompleteStream(_ context.Context, _ CompletionRequest) StreamResult {
	chunks := make(chan StreamChunk)
	errs := make(chan error, 1)
	close(chunks)
	close(errs)
	return StreamResult{Chunks: chunks, Err: errs}
}

func (s *scriptedProvider) Validate(_ context.Context) error { return nil }

func TestConversation_PreservesHistoryAcrossCalls(t *testing.T) {
	p := &scriptedProvider{responses: []CompletionResponse{
		{ID: "resp-1", Content: "walkthrough", Choices: []Choice{{Content: "walkthrough"}}},
		{ID: "resp-2", Content: "review", Choices: []Choice{{Content: "review"}}},
	}}
	conv := NewConversation(p, ConversationOptions{SystemPrompt: "review system"})

	resp, err := conv.Complete(context.Background(), "first prompt")
	require.NoError(t, err)
	assert.Equal(t, "resp-1", resp.ID)

	resp, err = conv.Complete(context.Background(), "second prompt")
	require.NoError(t, err)
	assert.Equal(t, "resp-2", resp.ID)

	require.Len(t, p.requests, 2)
	assert.Len(t, p.requests[0].Messages, 2)
	assert.Equal(t, RoleSystem, p.requests[0].Messages[0].Role)
	assert.Equal(t, RoleUser, p.requests[0].Messages[1].Role)

	second := p.requests[1].Messages
	require.Len(t, second, 4)
	assert.Equal(t, "first prompt", second[1].Content)
	assert.Equal(t, RoleAssistant, second[2].Role)
	assert.Equal(t, "walkthrough", second[2].Content)
	assert.Equal(t, "second prompt", second[3].Content)
	assert.Equal(t, "resp-2", conv.LastResponseID())
}

func TestConversation_CloneForksHistoryWithoutMutatingParent(t *testing.T) {
	p := &scriptedProvider{responses: []CompletionResponse{
		{Content: "seed reply", Choices: []Choice{{Content: "seed reply"}}},
		{Content: "fork reply", Choices: []Choice{{Content: "fork reply"}}},
	}}
	conv := NewConversation(p, ConversationOptions{Messages: []Message{{Role: RoleAssistant, Content: "seed context"}}})
	_, err := conv.Complete(context.Background(), "seed prompt")
	require.NoError(t, err)

	fork := conv.Clone()
	_, err = fork.Complete(context.Background(), "fork prompt")
	require.NoError(t, err)

	assert.Len(t, conv.Messages(), 3)
	assert.Len(t, fork.Messages(), 5)
	assert.Equal(t, "fork prompt", fork.Messages()[3].Content)
}

func TestSimpleComplete_UsesConversationFriendlyMessageShape(t *testing.T) {
	p := &scriptedProvider{responses: []CompletionResponse{{ID: "resp-1", Content: "done", Choices: []Choice{{Content: "done"}}}}}
	id, choices, err := SimpleComplete(p, "system", "assistant", "question")
	require.NoError(t, err)
	assert.Equal(t, "resp-1", id)
	assert.Equal(t, []string{"done"}, choices)
	require.Len(t, p.requests, 1)
	assert.Len(t, p.requests[0].Messages, 3)
}
