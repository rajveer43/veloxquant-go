// Package openai implements a minimal client for OpenAI-compatible chat
// completion APIs, used to talk to the VeloxQuant runtime (or any other
// OpenAI-compatible local server).
package openai

// Message is a single chat message in OpenAI's wire format.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is an OpenAI-compatible chat completion request.
type ChatRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Temperature    float64         `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Stream         bool            `json:"stream,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}

// ResponseFormat requests a specific output format from the model, in
// OpenAI's wire format: {"type": "json_object"} or {"type": "json_schema",
// "json_schema": {...}}.
type ResponseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`
}

// JSONSchema names and constrains a json_schema response format.
type JSONSchema struct {
	Name   string `json:"name"`
	Strict bool   `json:"strict,omitempty"`
	Schema any    `json:"schema"`
}

// Usage reports token accounting, matching OpenAI's response shape.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatChoice is a single completion choice in a non-streaming response.
type ChatChoice struct {
	Index   int     `json:"index"`
	Message Message `json:"message"`
}

// ChatResponse is an OpenAI-compatible chat completion response.
type ChatResponse struct {
	ID      string       `json:"id"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   Usage        `json:"usage"`
}

// StreamDelta is the incremental content of one streaming choice.
type StreamDelta struct {
	Content string `json:"content"`
}

// StreamChoice is a single choice within a streaming chunk.
type StreamChoice struct {
	Index        int         `json:"index"`
	Delta        StreamDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

// StreamChunk is a single Server-Sent Events data payload in an
// OpenAI-compatible streaming response.
type StreamChunk struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
}

// Model describes a model available on the runtime, as returned by
// GET /v1/models.
type Model struct {
	ID string `json:"id"`
}

// ModelList is the response body of GET /v1/models.
type ModelList struct {
	Data []Model `json:"data"`
}
