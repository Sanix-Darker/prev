// Package provider defines the core types and interfaces for multi-provider
// AI support. It abstracts away the differences between AI services (OpenAI,
// Anthropic Claude, Azure OpenAI, etc.) behind a unified interface, enabling
// the CLI tool to switch providers without changing application logic.
//
// Design principles:
//   - Idiomatic Go: context propagation, error values, functional options
//   - go-resty/v2 as the HTTP transport layer
//   - spf13/viper for configuration management
//   - Channel-based streaming for concurrent consumption
//   - Normalized error codes across providers
//   - Registry/factory pattern for provider discovery
package provider

import (
	"context"
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// Message types
// ---------------------------------------------------------------------------

// Role represents the role of a message participant.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message represents a single message in a conversation.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// ---------------------------------------------------------------------------
// Request types
// ---------------------------------------------------------------------------

// CompletionRequest is the provider-agnostic request structure that gets
// translated into each provider's native format by the provider implementation.
type CompletionRequest struct {
	// Model is the provider-specific model identifier (e.g. "gpt-4o",
	// "claude-sonnet-4-20250514").
	Model string `json:"model"`

	// Messages is the ordered conversation history.
	Messages []Message `json:"messages"`

	// MaxTokens limits the response length. Providers have different defaults
	// and caps; the implementation should clamp or error appropriately.
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature controls randomness (0.0 = deterministic, 1.0+ = creative).
	// A nil value means "use provider default".
	Temperature *float64 `json:"temperature,omitempty"`

	// TopP is nucleus sampling. A nil value means "use provider default".
	TopP *float64 `json:"top_p,omitempty"`

	// Stream enables server-sent event streaming when true. The caller should
	// use AIProvider.CompleteStream instead of AIProvider.Complete for streamed
	// responses.
	Stream bool `json:"stream,omitempty"`

	// StopSequences optionally tells the model to stop generating upon
	// encountering any of these strings.
	StopSequences []string `json:"stop,omitempty"`
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

// CompletionResponse is the provider-agnostic response returned from a
// non-streaming completion call.
type CompletionResponse struct {
	// ID is the provider-assigned response identifier.
	ID string `json:"id"`

	// Model is the model that actually served the request (providers may
	// alias or auto-select).
	Model string `json:"model"`

	// Content is the assistant's reply text. For multi-choice responses only
	// the first choice is placed here; all choices are in Choices.
	Content string `json:"content"`

	// Choices holds every completion choice returned by the provider. Most
	// requests return a single choice.
	Choices []Choice `json:"choices,omitempty"`

	// Usage contains token accounting for the request.
	Usage Usage `json:"usage"`

	// FinishReason indicates why generation stopped (e.g. "stop",
	// "max_tokens", "end_turn").
	FinishReason string `json:"finish_reason"`

	// ProviderMeta carries any provider-specific metadata that does not fit
	// into the normalized fields (e.g. Anthropic's stop_sequence value).
	ProviderMeta map[string]interface{} `json:"provider_meta,omitempty"`
}

// Choice represents a single completion choice from the provider.
type Choice struct {
	Index        int    `json:"index"`
	Content      string `json:"content"`
	FinishReason string `json:"finish_reason"`
}

// Usage tracks token consumption.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ---------------------------------------------------------------------------
// Streaming types
// ---------------------------------------------------------------------------

// StreamChunk represents one incremental piece of a streamed response.
type StreamChunk struct {
	// Content is the text delta for this chunk.
	Content string

	// Done is true when the stream has finished.
	Done bool

	// FinishReason is populated on the final chunk.
	FinishReason string

	// Usage is populated on the final chunk when the provider supports it.
	Usage *Usage
}

// StreamResult bundles the two channels returned from CompleteStream.
// Callers should range over Chunks and then check Err.
//
//	result := provider.CompleteStream(ctx, req)
//	for chunk := range result.Chunks {
//	    fmt.Print(chunk.Content)
//	}
//	if err := <-result.Err; err != nil { ... }
type StreamResult struct {
	Chunks <-chan StreamChunk
	Err    <-chan error
}

// ---------------------------------------------------------------------------
// Error types
// ---------------------------------------------------------------------------

// ErrorCode classifies errors returned by providers into actionable
// categories so the caller can decide how to react (retry, abort, etc.)
// without inspecting provider-specific error payloads.
type ErrorCode string

const (
	ErrCodeAuthentication      ErrorCode = "authentication"
	ErrCodeRateLimit           ErrorCode = "rate_limit"
	ErrCodeInvalidRequest      ErrorCode = "invalid_request"
	ErrCodeContextLength       ErrorCode = "context_length"
	ErrCodeContentFilter       ErrorCode = "content_filter"
	ErrCodeProviderUnavailable ErrorCode = "provider_unavailable"
	ErrCodeTimeout             ErrorCode = "timeout"
	ErrCodeUnknown             ErrorCode = "unknown"
)

// ProviderError is a structured error that carries both a normalized code
// and the original provider-specific details. It implements the standard
// error interface and supports errors.Is / errors.As unwrapping.
type ProviderError struct {
	Code       ErrorCode
	Message    string
	Provider   string
	StatusCode int
	Cause      error
}

func (e *ProviderError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %s (status %d): %v",
			e.Provider, e.Code, e.Message, e.StatusCode, e.Cause)
	}
	return fmt.Sprintf("[%s] %s: %s (status %d)",
		e.Provider, e.Code, e.Message, e.StatusCode)
}

func (e *ProviderError) Unwrap() error {
	return e.Cause
}

// Sentinel errors for use with errors.Is().
var (
	ErrAuthentication      = &ProviderError{Code: ErrCodeAuthentication}
	ErrRateLimit           = &ProviderError{Code: ErrCodeRateLimit}
	ErrInvalidRequest      = &ProviderError{Code: ErrCodeInvalidRequest}
	ErrContextLength       = &ProviderError{Code: ErrCodeContextLength}
	ErrContentFilter       = &ProviderError{Code: ErrCodeContentFilter}
	ErrProviderUnavailable = &ProviderError{Code: ErrCodeProviderUnavailable}
	ErrTimeout             = &ProviderError{Code: ErrCodeTimeout}
)

// Is allows errors.Is to match ProviderErrors by code.
func (e *ProviderError) Is(target error) bool {
	t, ok := target.(*ProviderError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// ---------------------------------------------------------------------------
// Retry configuration
// ---------------------------------------------------------------------------

// RetryConfig controls exponential-backoff retry behaviour. The zero value
// disables retries.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts (0 = no retries).
	MaxRetries int

	// InitialInterval is the delay before the first retry.
	InitialInterval time.Duration

	// MaxInterval caps the delay between retries.
	MaxInterval time.Duration

	// Multiplier scales the interval after each attempt.
	Multiplier float64
}

// DefaultRetryConfig returns a sensible default retry configuration:
// 3 retries, starting at 1 s, capped at 30 s, with a 2x multiplier.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:      3,
		InitialInterval: 1 * time.Second,
		MaxInterval:     30 * time.Second,
		Multiplier:      2.0,
	}
}

// ---------------------------------------------------------------------------
// Provider metadata
// ---------------------------------------------------------------------------

// ProviderInfo describes a registered provider for introspection and
// user-facing help text.
type ProviderInfo struct {
	// Name is the canonical short name used in configuration (e.g. "openai").
	Name string

	// DisplayName is the human-readable name (e.g. "OpenAI").
	DisplayName string

	// Description is a one-line summary for help text.
	Description string

	// DefaultModel is the model used when the user does not specify one.
	DefaultModel string

	// SupportsStreaming indicates whether this provider supports streaming.
	SupportsStreaming bool
}

// ---------------------------------------------------------------------------
// Core interface
// ---------------------------------------------------------------------------

// AIProvider is the central abstraction. Every AI service (OpenAI, Anthropic,
// Azure, self-hosted, etc.) implements this interface so that the rest of the
// application can work with any provider interchangeably.
//
// The design follows established Go patterns seen in langchaingo/llms,
// any-llm-go, and gollm: a small interface surface, context propagation,
// and channel-based streaming.
type AIProvider interface {
	// Info returns static metadata about this provider.
	Info() ProviderInfo

	// Complete sends a chat completion request and blocks until the full
	// response is available. The context controls cancellation and timeouts.
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

	// CompleteStream sends a chat completion request and returns a
	// StreamResult whose Chunks channel yields incremental content. The
	// caller must drain the Chunks channel; the Err channel delivers at
	// most one value after Chunks closes.
	//
	// If the provider does not support streaming, the implementation should
	// fall back to a single-chunk emission wrapping the full response.
	CompleteStream(ctx context.Context, req CompletionRequest) StreamResult

	// Validate checks that the provider is correctly configured (API key
	// present, endpoint reachable, etc.) and returns a descriptive error
	// if not. This is intended for use at CLI startup or "prev doctor".
	Validate(ctx context.Context) error
}
