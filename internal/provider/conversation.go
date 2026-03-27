package provider

import (
	"context"
	"strings"
)

// ConversationOptions configures a logical multi-request conversation that
// reuses message history across related provider calls.
type ConversationOptions struct {
	SystemPrompt  string
	Messages      []Message
	Model         string
	MaxTokens     int
	Temperature   *float64
	TopP          *float64
	StopSequences []string
}

// Conversation keeps provider-agnostic conversation state so related review
// calls can reuse prior prompts and assistant replies.
type Conversation struct {
	provider       AIProvider
	systemPrompt   string
	messages       []Message
	model          string
	maxTokens      int
	temperature    *float64
	topP           *float64
	stopSequences  []string
	lastResponseID string
}

// NewConversation constructs a conversation with optional seed history.
func NewConversation(p AIProvider, opts ConversationOptions) *Conversation {
	c := &Conversation{
		provider:      p,
		systemPrompt:  strings.TrimSpace(opts.SystemPrompt),
		model:         strings.TrimSpace(opts.Model),
		maxTokens:     opts.MaxTokens,
		temperature:   opts.Temperature,
		topP:          opts.TopP,
		stopSequences: append([]string(nil), opts.StopSequences...),
	}
	c.messages = append(c.messages, normalizeMessages(opts.Messages)...)
	return c
}

// Clone returns an independent copy of the conversation history.
func (c *Conversation) Clone() *Conversation {
	if c == nil {
		return nil
	}
	clone := &Conversation{
		provider:       c.provider,
		systemPrompt:   c.systemPrompt,
		model:          c.model,
		maxTokens:      c.maxTokens,
		temperature:    c.temperature,
		topP:           c.topP,
		stopSequences:  append([]string(nil), c.stopSequences...),
		lastResponseID: c.lastResponseID,
	}
	clone.messages = append(clone.messages, c.messages...)
	return clone
}

// Append adds normalized messages to the conversation history.
func (c *Conversation) Append(msgs ...Message) {
	if c == nil {
		return
	}
	c.messages = append(c.messages, normalizeMessages(msgs)...)
}

// Messages returns a copy of the current conversation history excluding the system prompt.
func (c *Conversation) Messages() []Message {
	if c == nil {
		return nil
	}
	out := make([]Message, len(c.messages))
	copy(out, c.messages)
	return out
}

// LastResponseID returns the most recent provider response id seen in this conversation.
func (c *Conversation) LastResponseID() string {
	if c == nil {
		return ""
	}
	return c.lastResponseID
}

// Complete sends a new user message while preserving prior conversation state.
func (c *Conversation) Complete(ctx context.Context, prompt string) (*CompletionResponse, error) {
	return c.CompleteMessage(ctx, Message{Role: RoleUser, Content: prompt})
}

// CompleteMessage sends an arbitrary message while preserving conversation state.
func (c *Conversation) CompleteMessage(ctx context.Context, msg Message) (*CompletionResponse, error) {
	if c == nil {
		return nil, nil
	}
	msg.Content = strings.TrimSpace(msg.Content)

	messages := make([]Message, 0, len(c.messages)+2)
	if c.systemPrompt != "" {
		messages = append(messages, Message{Role: RoleSystem, Content: c.systemPrompt})
	}
	messages = append(messages, c.messages...)
	if msg.Content != "" {
		messages = append(messages, msg)
	}

	resp, err := c.provider.Complete(ctx, CompletionRequest{
		Model:         c.model,
		Messages:      messages,
		MaxTokens:     c.maxTokens,
		Temperature:   c.temperature,
		TopP:          c.topP,
		StopSequences: append([]string(nil), c.stopSequences...),
	})
	if err != nil {
		return nil, err
	}

	if msg.Content != "" {
		c.messages = append(c.messages, msg)
	}
	if strings.TrimSpace(resp.Content) != "" {
		c.messages = append(c.messages, Message{Role: RoleAssistant, Content: strings.TrimSpace(resp.Content)})
	}
	c.lastResponseID = strings.TrimSpace(resp.ID)
	return resp, nil
}

func normalizeMessages(msgs []Message) []Message {
	out := make([]Message, 0, len(msgs))
	for _, msg := range msgs {
		msg.Content = strings.TrimSpace(msg.Content)
		if msg.Content == "" {
			continue
		}
		if msg.Role == "" {
			msg.Role = RoleUser
		}
		out = append(out, msg)
	}
	return out
}
