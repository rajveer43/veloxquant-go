package openai

import (
	"encoding/json"
	"testing"
)

// TestChatRequestToolsRoundTrip confirms a ChatRequest with Tools set
// marshals to OpenAI's documented tools wire shape and round-trips back
// through JSON unmodified.
func TestChatRequestToolsRoundTrip(t *testing.T) {
	req := ChatRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: "user", Content: "what's the weather in Tokyo?"},
		},
		Tools: []ToolDefinition{
			{
				Type: "function",
				Function: ToolFunction{
					Name:        "get_weather",
					Description: "Get the current weather for a location",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{"type": "string"},
						},
						"required": []any{"location"},
					},
				},
			},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var wire map[string]any
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	tools, ok := wire["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools = %#v, want a single-element array", wire["tools"])
	}
	tool, ok := tools[0].(map[string]any)
	if !ok {
		t.Fatalf("tools[0] = %#v, want an object", tools[0])
	}
	if tool["type"] != "function" {
		t.Errorf("tools[0].type = %v, want %q", tool["type"], "function")
	}
	fn, ok := tool["function"].(map[string]any)
	if !ok {
		t.Fatalf("tools[0].function = %#v, want an object", tool["function"])
	}
	if fn["name"] != "get_weather" {
		t.Errorf("tools[0].function.name = %v, want %q", fn["name"], "get_weather")
	}
	if fn["description"] != "Get the current weather for a location" {
		t.Errorf("tools[0].function.description = %v", fn["description"])
	}
	if _, ok := fn["parameters"].(map[string]any); !ok {
		t.Fatalf("tools[0].function.parameters = %#v, want an object", fn["parameters"])
	}

	var roundTripped ChatRequest
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("Unmarshal() into ChatRequest error = %v", err)
	}
	if len(roundTripped.Tools) != 1 || roundTripped.Tools[0].Function.Name != "get_weather" {
		t.Errorf("round-tripped Tools = %#v", roundTripped.Tools)
	}
}

// TestChatResponseDecodesToolCalls confirms a fixture response containing
// tool_calls (finish_reason: "tool_calls") decodes into
// Message.ToolCalls correctly, and that FinishReason isn't treated as an
// error condition.
func TestChatResponseDecodesToolCalls(t *testing.T) {
	fixture := `{
		"id": "resp-1",
		"model": "test-model",
		"choices": [
			{
				"index": 0,
				"finish_reason": "tool_calls",
				"message": {
					"role": "assistant",
					"content": "",
					"tool_calls": [
						{
							"id": "call_abc123",
							"type": "function",
							"function": {
								"name": "get_weather",
								"arguments": "{\"location\":\"Tokyo\"}"
							}
						}
					]
				}
			}
		],
		"usage": {"prompt_tokens": 20, "completion_tokens": 8, "total_tokens": 28}
	}`

	var resp ChatResponse
	if err := json.Unmarshal([]byte(fixture), &resp); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if len(resp.Choices) != 1 {
		t.Fatalf("len(Choices) = %d, want 1", len(resp.Choices))
	}
	choice := resp.Choices[0]
	if choice.FinishReason != "tool_calls" {
		t.Errorf("FinishReason = %q, want %q", choice.FinishReason, "tool_calls")
	}
	if len(choice.Message.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(choice.Message.ToolCalls))
	}
	call := choice.Message.ToolCalls[0]
	if call.ID != "call_abc123" {
		t.Errorf("ID = %q, want %q", call.ID, "call_abc123")
	}
	if call.Function.Name != "get_weather" {
		t.Errorf("Function.Name = %q, want %q", call.Function.Name, "get_weather")
	}
	if call.Function.Arguments != `{"location":"Tokyo"}` {
		t.Errorf("Function.Arguments = %q", call.Function.Arguments)
	}
}

// TestMessageToolCallIDRoundTrip confirms a "tool" role result message
// with ToolCallID set marshals with tool_call_id on the wire.
func TestMessageToolCallIDRoundTrip(t *testing.T) {
	msg := Message{
		Role:       "tool",
		Content:    `{"tempC":22}`,
		ToolCallID: "call_abc123",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var wire map[string]any
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if wire["tool_call_id"] != "call_abc123" {
		t.Errorf("tool_call_id = %v, want %q", wire["tool_call_id"], "call_abc123")
	}
}
