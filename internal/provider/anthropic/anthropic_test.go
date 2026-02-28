package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanix-darker/prev/internal/config"
	"github.com/sanix-darker/prev/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockAnthropicServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Anthropic-specific headers
		assert.Equal(t, anthropicVersion, r.Header.Get("anthropic-version"))
		assert.NotEmpty(t, r.Header.Get("x-api-key"))

		resp := apiResponse{
			ID:         "msg-test",
			Type:       "message",
			Role:       "assistant",
			Model:      "claude-sonnet-4-20250514",
			StopReason: "end_turn",
			Content: []apiContentBlock{
				{Type: "text", Text: "Test Claude response"},
			},
			Usage: apiUsage{InputTokens: 10, OutputTokens: 20},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestClaudeComplete(t *testing.T) {
	server := mockAnthropicServer(t)
	defer server.Close()

	v := config.NewStore()
	v.Set("api_key", "test-key")
	v.Set("base_url", server.URL)
	v.Set("model", "claude-sonnet-4-20250514")
	v.Set("max_tokens", 100)
	v.Set("timeout", "10s")

	p, err := NewProvider(v)
	require.NoError(t, err)

	resp, err := p.Complete(context.Background(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: "You are a helpful assistant"},
			{Role: provider.RoleUser, Content: "Hello"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "Test Claude response", resp.Content)
	assert.Equal(t, "msg-test", resp.ID)
	assert.Equal(t, 30, resp.Usage.TotalTokens)
}

func TestClaudeComplete_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type": "error",
			"error": map[string]interface{}{
				"type":    "authentication_error",
				"message": "invalid x-api-key",
			},
		})
	}))
	defer server.Close()

	v := config.NewStore()
	v.Set("api_key", "bad-key")
	v.Set("base_url", server.URL)

	p, err := NewProvider(v)
	require.NoError(t, err)

	_, err = p.Complete(context.Background(), provider.CompletionRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "Hello"}},
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, provider.ErrAuthentication)
}

func TestClaudeComplete_Streaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		events := []string{
			"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg-test\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"claude-sonnet-4-20250514\",\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\n",
			"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n",
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello \"}}\n\n",
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"world!\"}}\n\n",
			"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"input_tokens\":10,\"output_tokens\":5}}\n\n",
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
		}

		flusher, ok := w.(http.Flusher)
		for _, event := range events {
			w.Write([]byte(event))
			if ok {
				flusher.Flush()
			}
		}
	}))
	defer server.Close()

	v := config.NewStore()
	v.Set("api_key", "test-key")
	v.Set("base_url", server.URL)
	v.Set("timeout", "10s")

	p, err := NewProvider(v)
	require.NoError(t, err)

	result := p.CompleteStream(context.Background(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "Hello"},
		},
		Stream: true,
	})

	var content string
	for chunk := range result.Chunks {
		content += chunk.Content
	}
	err = <-result.Err
	assert.NoError(t, err)
	assert.Equal(t, "Hello world!", content)
}

func TestClaudeInfo(t *testing.T) {
	v := config.NewStore()
	v.Set("api_key", "test")
	p, err := NewProvider(v)
	require.NoError(t, err)

	info := p.Info()
	assert.Equal(t, "anthropic", info.Name)
	assert.True(t, info.SupportsStreaming)
}

func TestClaudeValidate_NoAPIKey(t *testing.T) {
	v := config.NewStore()
	p, err := NewProvider(v)
	require.NoError(t, err)

	err = p.Validate(context.Background())
	assert.Error(t, err)
	assert.ErrorIs(t, err, provider.ErrAuthentication)
}
