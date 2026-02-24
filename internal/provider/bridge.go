package provider

import (
	"context"
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// Bridge: connects the new AIProvider interface to the existing codebase
// ---------------------------------------------------------------------------

// SimpleComplete is a convenience function that mirrors the existing
// ChatGptHandler signature, making migration straightforward. Existing call
// sites can replace:
//
//	apis.ChatGptHandler(system, assistant, user)
//
// with:
//
//	provider.SimpleComplete(p, system, assistant, user)
//
// It returns the same (id, choices, error) tuple as the old function.
func SimpleComplete(
	p AIProvider,
	systemPrompt string,
	assistantPrompt string,
	questionPrompt string,
) (string, []string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := CompletionRequest{
		Messages: []Message{
			{Role: RoleSystem, Content: systemPrompt},
			{Role: RoleAssistant, Content: assistantPrompt},
			{Role: RoleUser, Content: questionPrompt},
		},
	}

	resp, err := p.Complete(ctx, req)
	if err != nil {
		return "", nil, err
	}

	choices := make([]string, len(resp.Choices))
	for i, c := range resp.Choices {
		choices[i] = c.Content
	}

	return resp.ID, choices, nil
}

// SimpleCompleteStream is the streaming counterpart of SimpleComplete. It
// prints each chunk's content to the provided callback as it arrives.
func SimpleCompleteStream(
	p AIProvider,
	systemPrompt string,
	assistantPrompt string,
	questionPrompt string,
	onChunk func(content string),
) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	req := CompletionRequest{
		Messages: []Message{
			{Role: RoleSystem, Content: systemPrompt},
			{Role: RoleAssistant, Content: assistantPrompt},
			{Role: RoleUser, Content: questionPrompt},
		},
		Stream: true,
	}

	result := p.CompleteStream(ctx, req)

	var fullContent string
	for chunk := range result.Chunks {
		fullContent += chunk.Content
		if onChunk != nil {
			onChunk(chunk.Content)
		}
	}

	if err := <-result.Err; err != nil {
		return "", err
	}

	return fullContent, nil
}

// ApiCallWithProvider replaces the existing apis.ApiCall function. It takes
// a resolved AIProvider instead of a callback function.
//
// Migration example in cmd/diff.go:
//
//	// Before:
//	apis.ApiCall(conf, prompt, apis.ChatGptHandler)
//
//	// After:
//	p, _ := provider.Get(providerCfg.Name, providerCfg.Viper)
//	provider.ApiCallWithProvider(conf.Debug, p, prompt)
func ApiCallWithProvider(debug bool, p AIProvider, prompt string) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	req := CompletionRequest{
		Messages: []Message{
			{Role: RoleSystem, Content: "You are a helpful assistant and source code reviewer."},
			{Role: RoleAssistant, Content: "You are code reviewer for a project"},
			{Role: RoleUser, Content: prompt},
		},
	}

	// Use streaming if supported, otherwise fall back to blocking.
	info := p.Info()
	if info.SupportsStreaming {
		result := p.CompleteStream(ctx, req)
		for chunk := range result.Chunks {
			fmt.Print(chunk.Content)
		}
		if err := <-result.Err; err != nil {
			fmt.Printf("\nError: %v\n", err)
		}
		fmt.Println()
	} else {
		resp, err := p.Complete(ctx, req)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Println(resp.Content)
	}
}
