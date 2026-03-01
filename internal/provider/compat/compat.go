// Package compat implements the AIProvider interface for any endpoint that
// exposes an OpenAI-compatible Chat Completions API. This covers a wide
// range of self-hosted and third-party services:
//
//   - Ollama (http://localhost:11434/v1)
//   - LM Studio (http://localhost:1234/v1)
//   - Groq (https://api.groq.com/openai/v1)
//   - Together AI (https://api.together.xyz/v1)
//   - Fireworks AI (https://api.fireworks.ai/inference/v1)
//   - vLLM, LocalAI, text-generation-inference, etc.
//
// Because these services speak the same protocol as OpenAI, this package
// reuses the openai package's wire types and parsing logic, differing only
// in the base URL, default model, and optional authentication.
//
// Users register a custom provider name via the config file:
//
//	provider: ollama
//	providers:
//	  ollama:
//	    base_url: http://localhost:11434/v1
//	    model: llama3
//	    # api_key is optional for local endpoints
package compat

import (
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
	// Register several well-known OpenAI-compatible services. Users can also
	// register arbitrary names through the config file; the Resolve logic in
	// provider/config.go will fall back to this factory when the name does
	// not match a specifically-registered provider.
	for _, name := range []string{"ollama", "groq", "together", "lmstudio", "openai-compat", "gemini"} {
		provider.Register(name, newFactory(name))
	}
}

// newFactory returns a Factory closure that captures the provider name.
func newFactory(name string) provider.Factory {
	return func(v *config.Store) (provider.AIProvider, error) {
		return NewProvider(name, v)
	}
}

// ---------------------------------------------------------------------------
// OpenAI-compatible wire types (same as openai package)
// ---------------------------------------------------------------------------

type apiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type apiRequest struct {
	Model       string       `json:"model"`
	Messages    []apiMessage `json:"messages"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Temperature *float64     `json:"temperature,omitempty"`
	TopP        *float64     `json:"top_p,omitempty"`
	Stream      bool         `json:"stream,omitempty"`
	Stop        []string     `json:"stop,omitempty"`
}

type apiChoice struct {
	Index        int        `json:"index"`
	Message      apiMessage `json:"message"`
	Delta        apiMessage `json:"delta"`
	FinishReason string     `json:"finish_reason"`
}

type apiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type apiResponse struct {
	ID      string      `json:"id"`
	Model   string      `json:"model"`
	Choices []apiChoice `json:"choices"`
	Usage   apiUsage    `json:"usage"`
}

type apiError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// ---------------------------------------------------------------------------
// Provider implementation
// ---------------------------------------------------------------------------

// Provider implements provider.AIProvider for OpenAI-compatible endpoints.
type Provider struct {
	name     string
	client   *http.Client
	apiKey   string
	baseURL  string
	model    string
	maxTok   int
	retryCfg provider.RetryConfig
}

// NewProvider creates a new generic OpenAI-compatible provider.
func NewProvider(name string, v *config.Store) (provider.AIProvider, error) {
	baseURL := v.GetString("base_url")
	if baseURL == "" {
		return nil, &provider.ProviderError{
			Code:     provider.ErrCodeInvalidRequest,
			Message:  fmt.Sprintf("base_url is required for provider %q", name),
			Provider: name,
		}
	}

	model := v.GetString("model")
	if model == "" {
		model = "default"
	}
	maxTok := v.GetInt("max_tokens")
	if maxTok == 0 {
		maxTok = 1024
	}
	timeout := v.GetDuration("timeout")
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return &Provider{
		name:     name,
		client:   &http.Client{Timeout: timeout},
		apiKey:   v.GetString("api_key"),
		baseURL:  strings.TrimRight(baseURL, "/"),
		model:    model,
		maxTok:   maxTok,
		retryCfg: provider.DefaultRetryConfig(),
	}, nil
}

// Info returns provider metadata.
func (p *Provider) Info() provider.ProviderInfo {
	return provider.ProviderInfo{
		Name:              p.name,
		DisplayName:       strings.Title(p.name) + " (OpenAI-compatible)", //nolint:staticcheck
		Description:       fmt.Sprintf("OpenAI-compatible endpoint (%s)", p.baseURL),
		DefaultModel:      p.model,
		SupportsStreaming: true,
	}
}

// Validate checks basic configuration. For local endpoints the API key may be
// empty, so we only verify the base URL is set.
func (p *Provider) Validate(ctx context.Context) error {
	if p.baseURL == "" {
		return &provider.ProviderError{
			Code:     provider.ErrCodeInvalidRequest,
			Message:  "base_url is not configured",
			Provider: p.name,
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
			Provider: p.name, Cause: err,
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, &provider.ProviderError{
			Code: provider.ErrCodeUnknown, Message: "failed to build request",
			Provider: p.name, Cause: err,
		}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &provider.ProviderError{
			Code: provider.ErrCodeProviderUnavailable, Message: "HTTP request failed",
			Provider: p.name, Cause: err,
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &provider.ProviderError{
			Code: provider.ErrCodeUnknown, Message: "failed to read response",
			Provider: p.name, Cause: err,
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, classifyHTTPError(p.name, resp.StatusCode, respBody)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, &provider.ProviderError{
			Code: provider.ErrCodeUnknown, Message: "failed to decode response",
			Provider: p.name, Cause: err,
		}
	}

	return toCompletionResponse(&apiResp), nil
}

// CompleteStream performs a streaming chat completion.
func (p *Provider) CompleteStream(ctx context.Context, req provider.CompletionRequest) provider.StreamResult {
	chunks := make(chan provider.StreamChunk, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errCh)

		body := p.buildRequest(req, true)
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			errCh <- &provider.ProviderError{
				Code: provider.ErrCodeUnknown, Message: "failed to marshal request",
				Provider: p.name, Cause: err,
			}
			return
		}

		httpReq, err := http.NewRequestWithContext(
			ctx, http.MethodPost,
			p.baseURL+"/chat/completions",
			bytes.NewReader(bodyBytes),
		)
		if err != nil {
			errCh <- &provider.ProviderError{
				Code: provider.ErrCodeUnknown, Message: "failed to build request",
				Provider: p.name, Cause: err,
			}
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if p.apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
		}
		httpReq.Header.Set("Accept", "text/event-stream")

		httpResp, err := p.client.Do(httpReq)
		if err != nil {
			errCh <- &provider.ProviderError{
				Code: provider.ErrCodeProviderUnavailable, Message: "stream request failed",
				Provider: p.name, Cause: err,
			}
			return
		}
		defer httpResp.Body.Close()

		if httpResp.StatusCode != http.StatusOK {
			var buf [4096]byte
			n, _ := httpResp.Body.Read(buf[:])
			errCh <- classifyHTTPError(p.name, httpResp.StatusCode, buf[:n])
			return
		}

		scanner := provider.NewSSEScanner(httpResp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				if !provider.SendStreamChunk(ctx, chunks, provider.StreamChunk{Done: true}) {
					errCh <- ctx.Err()
				}
				return
			}

			var chunk apiResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if len(chunk.Choices) == 0 {
				continue
			}

			sc := provider.StreamChunk{
				Content:      chunk.Choices[0].Delta.Content,
				FinishReason: chunk.Choices[0].FinishReason,
			}
			if chunk.Choices[0].FinishReason != "" {
				sc.Done = true
			}

			if !provider.SendStreamChunk(ctx, chunks, sc) {
				errCh <- ctx.Err()
				return
			}
		}

		if err := scanner.Err(); err != nil {
			errCh <- &provider.ProviderError{
				Code: provider.ErrCodeUnknown, Message: "stream read error",
				Provider: p.name, Cause: err,
			}
		}
	}()

	return provider.StreamResult{Chunks: chunks, Err: errCh}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (p *Provider) buildRequest(req provider.CompletionRequest, stream bool) apiRequest {
	model := req.Model
	if model == "" {
		model = p.model
	}
	maxTok := req.MaxTokens
	if maxTok == 0 {
		maxTok = p.maxTok
	}

	msgs := make([]apiMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = apiMessage{Role: string(m.Role), Content: m.Content}
	}

	return apiRequest{
		Model:       model,
		Messages:    msgs,
		MaxTokens:   maxTok,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      stream,
		Stop:        req.StopSequences,
	}
}

func toCompletionResponse(r *apiResponse) *provider.CompletionResponse {
	resp := &provider.CompletionResponse{
		ID:    r.ID,
		Model: r.Model,
		Usage: provider.Usage{
			PromptTokens:     r.Usage.PromptTokens,
			CompletionTokens: r.Usage.CompletionTokens,
			TotalTokens:      r.Usage.TotalTokens,
		},
	}
	for _, c := range r.Choices {
		resp.Choices = append(resp.Choices, provider.Choice{
			Index:        c.Index,
			Content:      c.Message.Content,
			FinishReason: c.FinishReason,
		})
	}
	if len(resp.Choices) > 0 {
		resp.Content = resp.Choices[0].Content
		resp.FinishReason = resp.Choices[0].FinishReason
	}
	return resp
}

func classifyHTTPError(providerName string, statusCode int, body []byte) *provider.ProviderError {
	var apiErr apiError
	_ = json.Unmarshal(body, &apiErr)
	msg := apiErr.Error.Message
	if msg == "" {
		msg = fmt.Sprintf("HTTP %d", statusCode)
	}

	pe := &provider.ProviderError{
		Provider:   providerName,
		Message:    msg,
		StatusCode: statusCode,
	}

	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		pe.Code = provider.ErrCodeAuthentication
	case statusCode == http.StatusTooManyRequests:
		pe.Code = provider.ErrCodeRateLimit
	case statusCode >= 500:
		pe.Code = provider.ErrCodeProviderUnavailable
	default:
		pe.Code = provider.ErrCodeUnknown
	}

	return pe
}
