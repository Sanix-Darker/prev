// Package azure implements the AIProvider interface for Azure OpenAI Service.
//
// Azure OpenAI differs from the standard OpenAI API in URL structure and
// authentication:
//   - URL format: {endpoint}/openai/deployments/{deployment}/chat/completions?api-version={version}
//   - Authentication via "api-key" header (not Bearer token)
//   - The "model" field in config maps to the Azure deployment name
//
// The wire format is otherwise identical to OpenAI, so this package reuses
// the same request/response types.
package azure

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
	provider.Register("azure", NewProvider)
}

// ---------------------------------------------------------------------------
// Reused OpenAI wire types
// ---------------------------------------------------------------------------

type apiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type apiRequest struct {
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
		Code    string `json:"code"`
	} `json:"error"`
}

// ---------------------------------------------------------------------------
// Provider implementation
// ---------------------------------------------------------------------------

// Provider implements provider.AIProvider for Azure OpenAI Service.
type Provider struct {
	client     *resty.Client
	apiKey     string
	endpoint   string // e.g. https://<resource>.openai.azure.com
	deployment string // Azure deployment name
	apiVersion string
	maxTok     int
	retryCfg   provider.RetryConfig
}

// NewProvider is the factory function registered with the provider registry.
func NewProvider(v *viper.Viper) (provider.AIProvider, error) {
	apiKey := v.GetString("api_key")
	endpoint := strings.TrimRight(v.GetString("base_url"), "/")
	if endpoint == "" {
		return nil, &provider.ProviderError{
			Code:     provider.ErrCodeInvalidRequest,
			Message:  "base_url (Azure endpoint) is required for provider azure",
			Provider: "azure",
		}
	}
	deployment := v.GetString("model")
	if deployment == "" {
		return nil, &provider.ProviderError{
			Code:     provider.ErrCodeInvalidRequest,
			Message:  "model (Azure deployment name) is required for provider azure",
			Provider: "azure",
		}
	}
	apiVersion := v.GetString("api_version")
	if apiVersion == "" {
		apiVersion = "2024-02-01"
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
		client:     client,
		apiKey:     apiKey,
		endpoint:   endpoint,
		deployment: deployment,
		apiVersion: apiVersion,
		maxTok:     maxTok,
		retryCfg:   provider.DefaultRetryConfig(),
	}, nil
}

// completionsURL builds the Azure-specific Chat Completions URL.
func (p *Provider) completionsURL() string {
	return fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		p.endpoint, p.deployment, p.apiVersion)
}

// Info returns provider metadata.
func (p *Provider) Info() provider.ProviderInfo {
	return provider.ProviderInfo{
		Name:             "azure",
		DisplayName:      "Azure OpenAI",
		Description:      "Azure OpenAI Service (enterprise endpoint)",
		DefaultModel:     p.deployment,
		SupportsStreaming: true,
	}
}

// Validate checks that required config fields are present.
func (p *Provider) Validate(ctx context.Context) error {
	if p.apiKey == "" {
		return &provider.ProviderError{
			Code:     provider.ErrCodeAuthentication,
			Message:  "AZURE_OPENAI_API_KEY is not set",
			Provider: "azure",
		}
	}
	if p.endpoint == "" {
		return &provider.ProviderError{
			Code:     provider.ErrCodeInvalidRequest,
			Message:  "Azure endpoint (base_url) is not configured",
			Provider: "azure",
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

	resp, err := p.client.R().
		SetContext(ctx).
		SetHeader("api-key", p.apiKey).
		SetBody(body).
		Post(p.completionsURL())
	if err != nil {
		return nil, &provider.ProviderError{
			Code: provider.ErrCodeProviderUnavailable, Message: "HTTP request failed",
			Provider: "azure", Cause: err,
		}
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, classifyHTTPError(resp.StatusCode(), resp.Body())
	}

	var apiResp apiResponse
	if err := json.Unmarshal(resp.Body(), &apiResp); err != nil {
		return nil, &provider.ProviderError{
			Code: provider.ErrCodeUnknown, Message: "failed to decode response",
			Provider: "azure", Cause: err,
		}
	}

	return toCompletionResponse(&apiResp), nil
}

// CompleteStream performs a streaming chat completion via SSE.
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
			p.completionsURL(),
			strings.NewReader(string(bodyBytes)),
		)
		if err != nil {
			errCh <- &provider.ProviderError{
				Code: provider.ErrCodeUnknown, Message: "failed to build request",
				Provider: "azure", Cause: err,
			}
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("api-key", p.apiKey)
		httpReq.Header.Set("Accept", "text/event-stream")

		httpResp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			errCh <- &provider.ProviderError{
				Code: provider.ErrCodeProviderUnavailable, Message: "stream request failed",
				Provider: "azure", Cause: err,
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

			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			case chunks <- sc:
			}
		}
	}()

	return provider.StreamResult{Chunks: chunks, Err: errCh}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (p *Provider) buildRequest(req provider.CompletionRequest, stream bool) apiRequest {
	maxTok := req.MaxTokens
	if maxTok == 0 {
		maxTok = p.maxTok
	}

	msgs := make([]apiMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = apiMessage{Role: string(m.Role), Content: m.Content}
	}

	return apiRequest{
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

func classifyHTTPError(statusCode int, body []byte) *provider.ProviderError {
	var apiErr apiError
	_ = json.Unmarshal(body, &apiErr)
	msg := apiErr.Error.Message
	if msg == "" {
		msg = fmt.Sprintf("HTTP %d", statusCode)
	}

	pe := &provider.ProviderError{
		Provider:   "azure",
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
