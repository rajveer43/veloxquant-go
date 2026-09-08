// Package openai implements a minimal client for OpenAI-compatible chat
// completion APIs, used to talk to the VeloxQuant runtime (or any other
// OpenAI-compatible local server).
package openai

// Message is a single chat message in OpenAI's wire format. ToolCalls is
// populated on an assistant message that called one or more tools;
// ToolCallID is set on a "tool" role message reporting a tool's result back
// to the model, identifying which call it answers.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolDefinition describes a callable tool in OpenAI's function-calling
// wire format, sent in a ChatRequest's Tools field.
type ToolDefinition struct {
	Type     string       `json:"type"` // always "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction is the function description within a ToolDefinition.
type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters"` // JSON Schema
}

// ToolCall is a single tool invocation the model requested, in OpenAI's
// wire format.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // always "function"
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction is the function invocation within a ToolCall.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON-encoded arguments, per OpenAI's wire format
}

// ChatRequest is an OpenAI-compatible chat completion request.
type ChatRequest struct {
	Model          string           `json:"model"`
	Messages       []Message        `json:"messages"`
	Temperature    float64          `json:"temperature,omitempty"`
	MaxTokens      int              `json:"max_tokens,omitempty"`
	Stream         bool             `json:"stream,omitempty"`
	ResponseFormat *ResponseFormat  `json:"response_format,omitempty"`
	Tools          []ToolDefinition `json:"tools,omitempty"`
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
// FinishReason is not restricted to a fixed set of values by this client:
// "tool_calls" (the model requested one or more tool calls, carried in
// Message.ToolCalls) is a normal, non-error value alongside "stop" and
// "length", and is passed through unmodified rather than rejected.
type ChatChoice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason,omitempty"`
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

// EmbeddingsRequest is an OpenAI-compatible embeddings request.
type EmbeddingsRequest struct {
	Model string `json:"model"`
	// Input is either a single string or a []string, matching OpenAI's
	// input field, which accepts one string or a batch of strings.
	Input any `json:"input"`
}

// Embedding is a single embedding result within an EmbeddingsResponse.
type Embedding struct {
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

// EmbeddingsResponse is an OpenAI-compatible embeddings response.
type EmbeddingsResponse struct {
	Model string      `json:"model"`
	Data  []Embedding `json:"data"`
	Usage Usage       `json:"usage"`
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
