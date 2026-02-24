package provider_test

import (
	"context"
	"testing"

	"github.com/sanix-darker/prev/internal/provider"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider is a test double that satisfies AIProvider.
type mockProvider struct {
	name string
}

func (m *mockProvider) Info() provider.ProviderInfo {
	return provider.ProviderInfo{
		Name:             m.name,
		DisplayName:      "Mock " + m.name,
		SupportsStreaming: true,
	}
}

func (m *mockProvider) Complete(ctx context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
	return &provider.CompletionResponse{
		ID:      "mock-id",
		Content: "mock response from " + m.name,
		Choices: []provider.Choice{
			{Index: 0, Content: "mock response from " + m.name, FinishReason: "stop"},
		},
	}, nil
}

func (m *mockProvider) CompleteStream(ctx context.Context, req provider.CompletionRequest) provider.StreamResult {
	chunks := make(chan provider.StreamChunk, 2)
	errCh := make(chan error, 1)
	go func() {
		defer close(chunks)
		defer close(errCh)
		chunks <- provider.StreamChunk{Content: "mock stream from " + m.name}
		chunks <- provider.StreamChunk{Done: true, FinishReason: "stop"}
	}()
	return provider.StreamResult{Chunks: chunks, Err: errCh}
}

func (m *mockProvider) Validate(ctx context.Context) error {
	return nil
}

func mockFactory(name string) provider.Factory {
	return func(v *viper.Viper) (provider.AIProvider, error) {
		return &mockProvider{name: name}, nil
	}
}

func TestRegistryRegisterAndGet(t *testing.T) {
	reg := provider.NewRegistry()
	reg.Register("test-provider", mockFactory("test-provider"))

	p, err := reg.Get("test-provider", viper.New())
	require.NoError(t, err)
	assert.Equal(t, "test-provider", p.Info().Name)
}

func TestRegistryGetUnknownProvider(t *testing.T) {
	reg := provider.NewRegistry()
	_, err := reg.Get("nonexistent", viper.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestRegistryDuplicateRegistrationPanics(t *testing.T) {
	reg := provider.NewRegistry()
	reg.Register("dup", mockFactory("dup"))
	assert.Panics(t, func() {
		reg.Register("dup", mockFactory("dup"))
	})
}

func TestRegistryNames(t *testing.T) {
	reg := provider.NewRegistry()
	reg.Register("beta", mockFactory("beta"))
	reg.Register("alpha", mockFactory("alpha"))
	reg.Register("gamma", mockFactory("gamma"))

	names := reg.Names()
	assert.Equal(t, []string{"alpha", "beta", "gamma"}, names)
}

func TestMockProviderComplete(t *testing.T) {
	mp := &mockProvider{name: "test"}
	resp, err := mp.Complete(context.Background(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "hello"},
		},
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Content, "mock response")
}

func TestMockProviderStream(t *testing.T) {
	mp := &mockProvider{name: "test"}
	result := mp.CompleteStream(context.Background(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "hello"},
		},
	})

	var collected string
	for chunk := range result.Chunks {
		collected += chunk.Content
	}
	err := <-result.Err
	assert.NoError(t, err)
	assert.Contains(t, collected, "mock stream")
}

func TestProviderErrorIs(t *testing.T) {
	err := &provider.ProviderError{
		Code:     provider.ErrCodeRateLimit,
		Message:  "too many requests",
		Provider: "openai",
	}

	assert.ErrorIs(t, err, provider.ErrRateLimit)
	assert.NotErrorIs(t, err, provider.ErrAuthentication)
}

func TestProviderErrorUnwrap(t *testing.T) {
	cause := &provider.ProviderError{
		Code:    provider.ErrCodeTimeout,
		Message: "inner",
	}
	outer := &provider.ProviderError{
		Code:    provider.ErrCodeUnknown,
		Message: "outer",
		Cause:   cause,
	}

	assert.ErrorIs(t, outer.Unwrap(), cause)
}
