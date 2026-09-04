// Package langchain adapts a veloxquant.Client to the langchaingo
// (github.com/tmc/langchaingo) llms.Model interface, so a local VeloxQuant
// runtime can be used as the model backend in a langchaingo chain.
//
// This package is a separate Go module from the root veloxquant-go module
// specifically so that importing it (and its langchaingo dependency) is
// opt-in: programs that only need the core SDK are not forced to pull in
// langchaingo.
package langchain

import (
	"context"
	"errors"
	"fmt"

	veloxquant "github.com/rajveer43/veloxquant-go"
	"github.com/tmc/langchaingo/llms"
)

// LLM adapts a *veloxquant.Client to the langchaingo llms.Model interface.
// Construct one with New; the zero value is not usable.
type LLM struct {
	client       *veloxquant.Client
	defaultModel string
}

var _ llms.Model = (*LLM)(nil)

// New returns an LLM backed by client. defaultModel is used for calls that
// don't specify llms.WithModel.
func New(client *veloxquant.Client, defaultModel string) *LLM {
	return &LLM{client: client, defaultModel: defaultModel}
}

// GenerateContent asks the underlying VeloxQuant runtime to generate
// content from a sequence of messages. Only text parts are supported;
// non-text parts (images, tool calls, etc.) are rejected since the
// VeloxQuant runtime's chat completion API is text-only.
func (l *LLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	opts := llms.CallOptions{Model: l.defaultModel}
	for _, opt := range options {
		opt(&opts)
	}

	chatMessages, err := toChatMessages(messages)
	if err != nil {
		return nil, fmt.Errorf("veloxquant langchain adapter: %w", err)
	}

	req := veloxquant.ChatRequest{
		Model:       opts.Model,
		Messages:    chatMessages,
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
	}

	if opts.StreamingFunc != nil {
		return l.generateStreaming(ctx, req, opts.StreamingFunc)
	}

	resp, err := l.client.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("veloxquant langchain adapter: %w", err)
	}

	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content:    resp.Text,
				StopReason: "stop",
				GenerationInfo: map[string]any{
					"prompt_tokens":     resp.Usage.PromptTokens,
					"completion_tokens": resp.Usage.CompletionTokens,
					"total_tokens":      resp.Usage.TotalTokens,
				},
			},
		},
	}, nil
}

func (l *LLM) generateStreaming(ctx context.Context, req veloxquant.ChatRequest, streamingFunc func(ctx context.Context, chunk []byte) error) (*llms.ContentResponse, error) {
	stream, err := l.client.ChatStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("veloxquant langchain adapter: %w", err)
	}
	defer stream.Close()

	var full []byte
	for stream.Next() {
		chunk := stream.Chunk()
		if chunk.Text == "" {
			continue
		}
		full = append(full, chunk.Text...)
		if err := streamingFunc(ctx, []byte(chunk.Text)); err != nil {
			return nil, fmt.Errorf("veloxquant langchain adapter: streaming func: %w", err)
		}
	}
	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("veloxquant langchain adapter: %w", err)
	}

	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{Content: string(full), StopReason: "stop"},
		},
	}, nil
}

// Call is a simplified interface for a text-only prompt, generating a
// single string response. It's implemented in terms of GenerateContent.
func (l *LLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return llms.GenerateFromSinglePrompt(ctx, l, prompt, options...)
}

// toChatMessages converts langchaingo MessageContent to veloxquant
// Messages. Only single-part text messages are supported.
func toChatMessages(messages []llms.MessageContent) ([]veloxquant.Message, error) {
	result := make([]veloxquant.Message, 0, len(messages))
	for _, mc := range messages {
		role, err := toRole(mc.Role)
		if err != nil {
			return nil, err
		}

		var text string
		for _, part := range mc.Parts {
			tc, ok := part.(llms.TextContent)
			if !ok {
				return nil, fmt.Errorf("unsupported content part %T: the VeloxQuant runtime's chat API is text-only", part)
			}
			text += tc.Text
		}

		result = append(result, veloxquant.Message{Role: role, Content: text})
	}
	return result, nil
}

func toRole(t llms.ChatMessageType) (string, error) {
	switch t {
	case llms.ChatMessageTypeSystem:
		return "system", nil
	case llms.ChatMessageTypeHuman:
		return "user", nil
	case llms.ChatMessageTypeAI:
		return "assistant", nil
	case llms.ChatMessageTypeGeneric:
		return "user", nil
	case llms.ChatMessageTypeFunction, llms.ChatMessageTypeTool:
		return "", errors.New("function/tool messages are not supported by the VeloxQuant runtime's chat API")
	default:
		return "", fmt.Errorf("unknown chat message type %q", t)
	}
}
