package apis

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-resty/resty/v2"
	"github.com/sanix-darker/prev/internal/common"
)

// req
type MessageReq struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RequestReq struct {
	Model     string       `json:"model"`
	Messages  []MessageReq `json:"messages"`
	MaxTokens int          `json:"max_tokens"`
}

// resp
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type JSONResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Some globals
const (
	API_ENDPOINT = "https://api.openai.com/v1/chat/completions"
	MAX_TOKENS   = 500
	GPT_MODEL    = "gpt-3.5-turbo"
)

var (
	RESTY_CLIENT = resty.New()
	API_KEY      = os.Getenv("OPEN_AI") // "" // need to fix this letter os.Getenv("OPEN_AI")
)

// ReqBuilder the request builder with all necessary stuffs
func ReqBuilder() *resty.Request {
	if len(API_KEY) == 0 {
		common.LogError(
			"No API-KEY set for chatGPT client !",
			true,
			false,
			nil,
		)
		os.Exit(1)
	}
	return RESTY_CLIENT.R().SetAuthToken(
		API_KEY,
	).SetHeader(
		"Content-Type",
		"application/json",
	)
}

// Handler
func ChatGptHandler(systemPrompt string, questionPrompt string) (string, []string, error) {
	response, err := ReqBuilder().SetBody(RequestReq{
		Model: GPT_MODEL,
		Messages: []MessageReq{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: questionPrompt,
			},
		},
		MaxTokens: MAX_TOKENS,
	}).Post(API_ENDPOINT)

	if err != nil {
		common.LogError(
			fmt.Sprintf("Error while sending send the request: %v", err),
			true,
			false,
			nil,
		)
	}

	jsonData := response.Body()
	var jsonResponse JSONResponse
	if err := json.Unmarshal([]byte(jsonData), &jsonResponse); err != nil {
		common.LogError(
			err.Error(),
			true,
			false,
			nil,
		)
		return "", nil, err
	}

	// TODO: should save responses in a .cache and return only the content
	responseId := jsonResponse.ID
	responseChoices := []string{}
	for _, choice := range jsonResponse.Choices {
		responseChoices = append(responseChoices, choice.Message.Content)
	}

	return responseId, responseChoices, nil
}
