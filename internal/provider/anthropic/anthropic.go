// Package anthropic implements the AIProvider interface for the Anthropic
// Messages API (Claude models).
//
// Anthropic's API differs from OpenAI's in several key ways:
//   - The system prompt is a top-level field, not a message.
//   - The response body uses "content" as an array of typed blocks.
//   - Streaming uses distinct event types (content_block_delta, etc.).
//   - Authentication uses "x-api-key" header, not Bearer tokens.
//   - max_tokens is required (not optional).
//
// This implementation normalizes all of those differences behind the
// provider.AIProvider interface.
package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sanix-darker/prev/internal/config"
	"github.com/sanix-darker/prev/internal/provider"
)

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func init() {
	provider.Register("anthropic", NewProvider)
}

// ---------------------------------------------------------------------------
// Anthropic-specific API types (request)
// ---------------------------------------------------------------------------

type apiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type apiRequest struct {
	Model     string       `json:"model"`
	Messages  []apiMessage `json:"messages"`
	System    string       `json:"system,omitempty"`
	MaxTokens int          `json:"max_tokens"`
	Stream    bool         `json:"stream,omitempty"`

	// Optional parameters.
	Temperature   *float64 `json:"temperature,omitempty"`
	TopP          *float64 `json:"top_p,omitempty"`
	StopSequences []string `json:"stop_sequences,omitempty"`
}

// ---------------------------------------------------------------------------
// Anthropic-specific API types (response)
// ---------------------------------------------------------------------------

type apiContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type apiUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type apiResponse struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"` // "message"
	Role         string            `json:"role"`
	Content      []apiContentBlock `json:"content"`
	Model        string            `json:"model"`
	StopReason   string            `json:"stop_reason"`
	StopSequence *string           `json:"stop_sequence"`
	Usage        apiUsage          `json:"usage"`
}

type apiError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Streaming event types used by the Anthropic SSE protocol.
type streamEvent struct {
	Type         string          `json:"type"`
	Index        int             `json:"index"`
	ContentBlock apiContentBlock `json:"content_block"`
	Delta        struct {
		Type         string `json:"type"`
		Text         string `json:"text"`
		StopReason   string `json:"stop_reason"`
		StopSequence string `json:"stop_sequence"`
	} `json:"delta"`
	Message *apiResponse `json:"message"`
	Usage   *apiUsage    `json:"usage"`
}

// ---------------------------------------------------------------------------
// Provider implementation
// ---------------------------------------------------------------------------

const anthropicVersion = "2023-06-01"

// Provider implements provider.AIProvider for the Anthropic Messages API.
type Provider struct {
	client   *http.Client
	apiKey   string
	baseURL  string
	model    string
	maxTok   int
	retryCfg provider.RetryConfig
}

// NewProvider is the factory function registered with the provider registry.
func NewProvider(v *config.Store) (provider.AIProvider, error) {
	apiKey := v.GetString("api_key")
	baseURL := v.GetString("base_url")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	model := v.GetString("model")
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	maxTok := v.GetInt("max_tokens")
	if maxTok == 0 {
		maxTok = 1024
	}
	timeout := v.GetDuration("timeout")
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &Provider{
		client:   &http.Client{Timeout: timeout},
		apiKey:   apiKey,
		baseURL:  strings.TrimRight(baseURL, "/"),
		model:    model,
		maxTok:   maxTok,
		retryCfg: provider.DefaultRetryConfig(),
	}, nil
}

// Info returns provider metadata.
func (p *Provider) Info() provider.ProviderInfo {
	return provider.ProviderInfo{
		Name:             "anthropic",
		DisplayName:      "Anthropic (Claude)",
		Description:      "Anthropic Messages API (Claude Opus, Sonnet, Haiku)",
		DefaultModel:     "claude-sonnet-4-20250514",
		SupportsStreaming: true,
	}
}

// Validate checks that the API key is present.
func (p *Provider) Validate(ctx context.Context) error {
	if p.apiKey == "" {
		return &provider.ProviderError{
			Code:     provider.ErrCodeAuthentication,
			Message:  "ANTHROPIC_API_KEY is not set",
			Provider: "anthropic",
		}
	}
	return nil
}

// Complete performs a synchronous chat completion.
func (p *Provider) Complete(ctx context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
	return provider.WithRetry(ctx, p.retryCfg, func() (*provider.CompletionResponse, error) {
		return p.doComplete(ctx, req)
	})
}

func (p *Provider) doComplete(ctx context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
	body := p.buildRequest(req, false)

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, &provider.ProviderError{
			Code: provider.ErrCodeUnknown, Message: "failed to marshal request",
			Provider: "anthropic", Cause: err,
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, &provider.ProviderError{
			Code: provider.ErrCodeUnknown, Message: "failed to build request",
			Provider: "anthropic", Cause: err,
		}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &provider.ProviderError{
			Code: provider.ErrCodeProviderUnavailable, Message: "HTTP request failed",
			Provider: "anthropic", Cause: err,
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &provider.ProviderError{
			Code: provider.ErrCodeUnknown, Message: "failed to read response",
			Provider: "anthropic", Cause: err,
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, classifyHTTPError(resp.StatusCode, respBody)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, &provider.ProviderError{
			Code: provider.ErrCodeUnknown, Message: "failed to decode response",
			Provider: "anthropic", Cause: err,
		}
	}

	return toCompletionResponse(&apiResp), nil
}

// CompleteStream performs a streaming chat completion using Anthropic's SSE
// protocol.
func (p *Provider) CompleteStream(ctx context.Context, req provider.CompletionRequest) provider.StreamResult {
	chunks := make(chan provider.StreamChunk, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errCh)

		body := p.buildRequest(req, true)
		bodyBytes, _ := json.Marshal(body)

		httpReq, err := http.NewRequestWithContext(
			ctx, http.MethodPost,
			p.baseURL+"/v1/messages",
			strings.NewReader(string(bodyBytes)),
		)
		if err != nil {
			errCh <- &provider.ProviderError{
				Code: provider.ErrCodeUnknown, Message: "failed to build request",
				Provider: "anthropic", Cause: err,
			}
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("x-api-key", p.apiKey)
		httpReq.Header.Set("anthropic-version", anthropicVersion)
		httpReq.Header.Set("Accept", "text/event-stream")

		httpResp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			errCh <- &provider.ProviderError{
				Code: provider.ErrCodeProviderUnavailable, Message: "stream request failed",
				Provider: "anthropic", Cause: err,
			}
			return
		}
		defer httpResp.Body.Close()

		if httpResp.StatusCode != http.StatusOK {
			var buf [4096]byte
			n, _ := httpResp.Body.Read(buf[:])
			errCh <- classifyHTTPError(httpResp.StatusCode, buf[:n])
			return
		}

		// Anthropic SSE format:
		//   event: <event_type>
		//   data: <json>
		scanner := bufio.NewScanner(httpResp.Body)
		var currentEvent string

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "event: ") {
				currentEvent = strings.TrimPrefix(line, "event: ")
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			var evt streamEvent
			if err := json.Unmarshal([]byte(data), &evt); err != nil {
				continue
			}

			switch currentEvent {
			case "content_block_delta":
				if evt.Delta.Type == "text_delta" {
					select {
					case <-ctx.Done():
						errCh <- ctx.Err()
						return
					case chunks <- provider.StreamChunk{Content: evt.Delta.Text}:
					}
				}

			case "message_delta":
				sc := provider.StreamChunk{
					Done:         true,
					FinishReason: evt.Delta.StopReason,
				}
				if evt.Usage != nil {
					sc.Usage = &provider.Usage{
						PromptTokens:     evt.Usage.InputTokens,
						CompletionTokens: evt.Usage.OutputTokens,
						TotalTokens:      evt.Usage.InputTokens + evt.Usage.OutputTokens,
					}
				}
				select {
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				case chunks <- sc:
				}

			case "message_stop":
				// Stream complete.
				return

			case "error":
				errCh <- &provider.ProviderError{
					Code: provider.ErrCodeUnknown, Message: "stream error event received",
					Provider: "anthropic",
				}
				return
			}
		}

		if err := scanner.Err(); err != nil {
			errCh <- &provider.ProviderError{
				Code: provider.ErrCodeUnknown, Message: "stream read error",
				Provider: "anthropic", Cause: err,
			}
		}
	}()

	return provider.StreamResult{Chunks: chunks, Err: errCh}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildRequest converts the provider-agnostic CompletionRequest into
// Anthropic's native format. The key difference is that the "system" message
// is extracted from Messages and placed in the top-level System field.
func (p *Provider) buildRequest(req provider.CompletionRequest, stream bool) apiRequest {
	model := req.Model
	if model == "" {
		model = p.model
	}
	maxTok := req.MaxTokens
	if maxTok == 0 {
		maxTok = p.maxTok
	}

	var systemPrompt string
	var messages []apiMessage

	for _, m := range req.Messages {
		if m.Role == provider.RoleSystem {
			// Concatenate system messages (Anthropic supports only one).
			if systemPrompt != "" {
				systemPrompt += "\n\n"
			}
			systemPrompt += m.Content
			continue
		}
		messages = append(messages, apiMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	// Anthropic requires alternating user/assistant turns and the first
	// message must be from the user. If the conversation starts with an
	// assistant message (as the existing code does with an "assistant"
	// system-like preamble), we fold it into the system prompt.
	if len(messages) > 0 && messages[0].Role == "assistant" {
		if systemPrompt != "" {
			systemPrompt += "\n\n"
		}
		systemPrompt += messages[0].Content
		messages = messages[1:]
	}

	return apiRequest{
		Model:         model,
		System:        systemPrompt,
		Messages:      messages,
		MaxTokens:     maxTok,
		Stream:        stream,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		StopSequences: req.StopSequences,
	}
}

func toCompletionResponse(r *apiResponse) *provider.CompletionResponse {
	var content string
	for _, block := range r.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	resp := &provider.CompletionResponse{
		ID:           r.ID,
		Model:        r.Model,
		Content:      content,
		FinishReason: r.StopReason,
		Usage: provider.Usage{
			PromptTokens:     r.Usage.InputTokens,
			CompletionTokens: r.Usage.OutputTokens,
			TotalTokens:      r.Usage.InputTokens + r.Usage.OutputTokens,
		},
		Choices: []provider.Choice{
			{
				Index:        0,
				Content:      content,
				FinishReason: r.StopReason,
			},
		},
	}

	if r.StopSequence != nil {
		resp.ProviderMeta = map[string]interface{}{
			"stop_sequence": *r.StopSequence,
		}
	}

	return resp
}

// classifyHTTPError maps HTTP status codes to normalized provider errors for
// the Anthropic API.
func classifyHTTPError(statusCode int, body []byte) *provider.ProviderError {
	var apiErr apiError
	_ = json.Unmarshal(body, &apiErr)
	msg := apiErr.Error.Message
	if msg == "" {
		msg = fmt.Sprintf("HTTP %d", statusCode)
	}

	pe := &provider.ProviderError{
		Provider:   "anthropic",
		Message:    msg,
		StatusCode: statusCode,
	}

	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		pe.Code = provider.ErrCodeAuthentication
	case statusCode == http.StatusTooManyRequests:
		pe.Code = provider.ErrCodeRateLimit
	case statusCode == http.StatusBadRequest:
		errType := apiErr.Error.Type
		if errType == "invalid_request_error" &&
			(strings.Contains(msg, "max_tokens") || strings.Contains(msg, "too long")) {
			pe.Code = provider.ErrCodeContextLength
		} else {
			pe.Code = provider.ErrCodeInvalidRequest
		}
	case statusCode == 529: // Anthropic overloaded
		pe.Code = provider.ErrCodeProviderUnavailable
	case statusCode >= 500:
		pe.Code = provider.ErrCodeProviderUnavailable
	default:
		pe.Code = provider.ErrCodeUnknown
	}

	return pe
}
