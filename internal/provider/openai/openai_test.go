package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sanix-darker/prev/internal/config"
	"github.com/sanix-darker/prev/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func mockOpenAIServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
			return
		}

		if r.URL.Path == "/chat/completions" {
			resp := apiResponse{
				ID:    "chatcmpl-test",
				Model: "gpt-4o",
				Choices: []apiChoice{
					{
						Index:        0,
						Message:      apiMessage{Role: "assistant", Content: "Test response"},
						FinishReason: "stop",
					},
				},
				Usage: apiUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
}

func TestOpenAIComplete(t *testing.T) {
	server := mockOpenAIServer(t)
	defer server.Close()

	v := config.NewStore()
	v.Set("api_key", "test-key")
	v.Set("base_url", server.URL)
	v.Set("model", "gpt-4o")
	v.Set("max_tokens", 100)
	v.Set("timeout", "10s")

	p, err := NewProvider(v)
	require.NoError(t, err)

	resp, err := p.Complete(context.Background(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "Hello"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "Test response", resp.Content)
	assert.Equal(t, "chatcmpl-test", resp.ID)
	assert.Equal(t, 15, resp.Usage.TotalTokens)
}

func TestOpenAIComplete_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Invalid API key",
				"type":    "authentication_error",
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

func TestOpenAIComplete_EmptyAPIKey(t *testing.T) {
	v := config.NewStore()
	v.Set("base_url", "http://localhost:1234")

	p, err := NewProvider(v)
	require.NoError(t, err)

	err = p.Validate(context.Background())
	assert.Error(t, err)
	assert.ErrorIs(t, err, provider.ErrAuthentication)
}

func TestOpenAIInfo(t *testing.T) {
	v := config.NewStore()
	v.Set("api_key", "test")
	p, err := NewProvider(v)
	require.NoError(t, err)

	info := p.Info()
	assert.Equal(t, "openai", info.Name)
	assert.True(t, info.SupportsStreaming)
}

func TestOpenAIComplete_GPT5UsesMaxCompletionTokens(t *testing.T) {
	var got map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		resp := apiResponse{
			ID: "chatcmpl-test", Model: "gpt-5.2-chat-latest",
			Choices: []apiChoice{{Index: 0, Message: apiMessage{Role: "assistant", Content: "ok"}, FinishReason: "stop"}},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	v := config.NewStore()
	v.Set("api_key", "test-key")
	v.Set("base_url", server.URL)
	v.Set("model", "gpt-5.2-chat-latest")
	v.Set("max_tokens", 123)

	p, err := NewProvider(v)
	require.NoError(t, err)
	_, err = p.Complete(context.Background(), provider.CompletionRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "Hello"}},
	})
	require.NoError(t, err)

	_, hasMaxTokens := got["max_tokens"]
	_, hasMaxCompletion := got["max_completion_tokens"]
	assert.False(t, hasMaxTokens)
	assert.True(t, hasMaxCompletion)
	assert.EqualValues(t, 123, got["max_completion_tokens"])
}

func TestOpenAICompleteStream_UsesProviderClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"},\"finish_reason\":\"\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	v := config.NewStore()
	v.Set("api_key", "test-key")
	v.Set("base_url", server.URL)
	v.Set("timeout", "10s")

	ai, err := NewProvider(v)
	require.NoError(t, err)
	p := ai.(*Provider)

	var calls atomic.Int32
	baseRT := server.Client().Transport
	p.client = &http.Client{
		Timeout: 2 * time.Second,
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			calls.Add(1)
			return baseRT.RoundTrip(req)
		}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result := p.CompleteStream(ctx, provider.CompletionRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hello"}},
		Stream:   true,
	})

	var content strings.Builder
	for chunk := range result.Chunks {
		content.WriteString(chunk.Content)
	}
	err = <-result.Err
	require.NoError(t, err)
	assert.GreaterOrEqual(t, calls.Load(), int32(1))
	assert.Equal(t, "hello", content.String())
}
