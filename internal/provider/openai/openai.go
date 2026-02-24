// Package openai implements the AIProvider interface for the OpenAI Chat
// Completions API (and any OpenAI-compatible endpoint such as Azure, Ollama,
// LM Studio, etc.).
//
// It uses go-resty/v2 for HTTP transport and supports both blocking and
// streaming completions with SSE (server-sent events).
package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/sanix-darker/prev/internal/provider"
	"github.com/spf13/viper"
)

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func init() {
	provider.Register("openai", NewProvider)
}

// ---------------------------------------------------------------------------
// OpenAI-specific API types (request)
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

// ---------------------------------------------------------------------------
// OpenAI-specific API types (response)
// ---------------------------------------------------------------------------

type apiChoice struct {
	Index        int        `json:"index"`
	Message      apiMessage `json:"message"`
	Delta        apiMessage `json:"delta"` // used in streaming
	FinishReason string     `json:"finish_reason"`
}

type apiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type apiResponse struct {
	ID      string      `json:"id"`
	Object  string      `json:"object"`
	Created int64       `json:"created"`
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

// Provider implements provider.AIProvider for OpenAI's Chat Completions API.
type Provider struct {
	client   *resty.Client
	apiKey   string
	baseURL  string
	model    string
	maxTok   int
	retryCfg provider.RetryConfig
}

// NewProvider is the factory function registered with the provider registry.
// It reads configuration from the supplied viper instance.
func NewProvider(v *viper.Viper) (provider.AIProvider, error) {
	apiKey := v.GetString("api_key")
	baseURL := v.GetString("base_url")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := v.GetString("model")
	if model == "" {
		model = "gpt-4o"
	}
	maxTok := v.GetInt("max_tokens")
	if maxTok == 0 {
		maxTok = 1024
	}
	timeout := v.GetDuration("timeout")
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	client := resty.New().
		SetTimeout(timeout).
		SetHeader("Content-Type", "application/json")

	return &Provider{
		client:   client,
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
		Name:             "openai",
		DisplayName:      "OpenAI",
		Description:      "OpenAI Chat Completions API (GPT-4o, GPT-4, GPT-3.5-turbo, etc.)",
		DefaultModel:     "gpt-4o",
		SupportsStreaming: true,
	}
}

// Validate checks that the API key is set and the endpoint is reachable.
func (p *Provider) Validate(ctx context.Context) error {
	if p.apiKey == "" {
		return &provider.ProviderError{
			Code:     provider.ErrCodeAuthentication,
			Message:  "OPENAI_API_KEY is not set",
			Provider: "openai",
		}
	}
	// Quick connectivity check: list models.
	resp, err := p.client.R().
		SetContext(ctx).
		SetAuthToken(p.apiKey).
		Get(p.baseURL + "/models")
	if err != nil {
		return &provider.ProviderError{
			Code:     provider.ErrCodeProviderUnavailable,
			Message:  "failed to reach OpenAI API",
			Provider: "openai",
			Cause:    err,
		}
	}
	if resp.StatusCode() != http.StatusOK {
		return &provider.ProviderError{
			Code:       provider.ErrCodeAuthentication,
			Message:    "OpenAI API returned non-200 on validation",
			Provider:   "openai",
			StatusCode: resp.StatusCode(),
		}
	}
	return nil
}

// Complete performs a synchronous (non-streaming) chat completion.
func (p *Provider) Complete(ctx context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
	return provider.WithRetry(ctx, p.retryCfg, func() (*provider.CompletionResponse, error) {
		return p.doComplete(ctx, req)
	})
}

func (p *Provider) doComplete(ctx context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}
	maxTok := req.MaxTokens
	if maxTok == 0 {
		maxTok = p.maxTok
	}

	body := apiRequest{
		Model:       model,
		Messages:    toAPIMessages(req.Messages),
		MaxTokens:   maxTok,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      false,
		Stop:        req.StopSequences,
	}

	resp, err := p.client.R().
		SetContext(ctx).
		SetAuthToken(p.apiKey).
		SetBody(body).
		Post(p.baseURL + "/chat/completions")
	if err != nil {
		return nil, &provider.ProviderError{
			Code:     provider.ErrCodeProviderUnavailable,
			Message:  "HTTP request failed",
			Provider: "openai",
			Cause:    err,
		}
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, classifyHTTPError("openai", resp.StatusCode(), resp.Body())
	}

	var apiResp apiResponse
	if err := json.Unmarshal(resp.Body(), &apiResp); err != nil {
		return nil, &provider.ProviderError{
			Code:     provider.ErrCodeUnknown,
			Message:  "failed to decode response",
			Provider: "openai",
			Cause:    err,
		}
	}

	return toCompletionResponse(&apiResp), nil
}

// CompleteStream performs a streaming chat completion using server-sent events.
func (p *Provider) CompleteStream(ctx context.Context, req provider.CompletionRequest) provider.StreamResult {
	chunks := make(chan provider.StreamChunk, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errCh)

		model := req.Model
		if model == "" {
			model = p.model
		}
		maxTok := req.MaxTokens
		if maxTok == 0 {
			maxTok = p.maxTok
		}

		body := apiRequest{
			Model:       model,
			Messages:    toAPIMessages(req.Messages),
			MaxTokens:   maxTok,
			Temperature: req.Temperature,
			TopP:        req.TopP,
			Stream:      true,
			Stop:        req.StopSequences,
		}

		bodyBytes, _ := json.Marshal(body)

		// Use raw http.Request so we can read the SSE stream line-by-line.
		httpReq, err := http.NewRequestWithContext(
			ctx, http.MethodPost,
			p.baseURL+"/chat/completions",
			strings.NewReader(string(bodyBytes)),
		)
		if err != nil {
			errCh <- &provider.ProviderError{
				Code: provider.ErrCodeUnknown, Message: "failed to build request",
				Provider: "openai", Cause: err,
			}
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
		httpReq.Header.Set("Accept", "text/event-stream")

		httpResp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			errCh <- &provider.ProviderError{
				Code: provider.ErrCodeProviderUnavailable, Message: "stream request failed",
				Provider: "openai", Cause: err,
			}
			return
		}
		defer httpResp.Body.Close()

		if httpResp.StatusCode != http.StatusOK {
			var buf [4096]byte
			n, _ := httpResp.Body.Read(buf[:])
			errCh <- classifyHTTPError("openai", httpResp.StatusCode, buf[:n])
			return
		}

		scanner := bufio.NewScanner(httpResp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				chunks <- provider.StreamChunk{Done: true}
				return
			}

			var chunk apiResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue // skip malformed chunks
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
				if chunk.Usage.TotalTokens > 0 {
					sc.Usage = &provider.Usage{
						PromptTokens:     chunk.Usage.PromptTokens,
						CompletionTokens: chunk.Usage.CompletionTokens,
						TotalTokens:      chunk.Usage.TotalTokens,
					}
				}
			}

			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			case chunks <- sc:
			}
		}

		if err := scanner.Err(); err != nil {
			errCh <- &provider.ProviderError{
				Code: provider.ErrCodeUnknown, Message: "stream read error",
				Provider: "openai", Cause: err,
			}
		}
	}()

	return provider.StreamResult{Chunks: chunks, Err: errCh}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func toAPIMessages(msgs []provider.Message) []apiMessage {
	out := make([]apiMessage, len(msgs))
	for i, m := range msgs {
		out[i] = apiMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
	}
	return out
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

// classifyHTTPError maps HTTP status codes to normalized provider errors.
func classifyHTTPError(providerName string, statusCode int, body []byte) *provider.ProviderError {
	// Try to parse the OpenAI error body.
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
	case statusCode == http.StatusBadRequest:
		// Check if it is a context-length error.
		if strings.Contains(msg, "maximum context length") ||
			strings.Contains(msg, "max_tokens") {
			pe.Code = provider.ErrCodeContextLength
		} else {
			pe.Code = provider.ErrCodeInvalidRequest
		}
	case statusCode >= 500:
		pe.Code = provider.ErrCodeProviderUnavailable
	case statusCode == http.StatusRequestTimeout || statusCode == http.StatusGatewayTimeout:
		pe.Code = provider.ErrCodeTimeout
	default:
		pe.Code = provider.ErrCodeUnknown
	}

	return pe
}
