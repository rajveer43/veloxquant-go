package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	veloxquant "github.com/rajveer43/veloxquant-go"
)

// fakeTool is a test-only Tool implementation.
type fakeTool struct {
	name        string
	description string
	parameters  any
	execute     func(ctx context.Context, args json.RawMessage) (any, error)
}

func (f *fakeTool) Name() string        { return f.name }
func (f *fakeTool) Description() string { return f.description }
func (f *fakeTool) Parameters() any     { return f.parameters }
func (f *fakeTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	return f.execute(ctx, args)
}

// newTestClient returns a veloxquant.Client whose Chat requests are served
// by handler.
func newTestClient(t *testing.T, handler http.HandlerFunc) *veloxquant.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := veloxquant.NewClient(veloxquant.WithOpenAICompatibleRuntime(srv.URL))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func writeChatResponse(t *testing.T, w http.ResponseWriter, content, finishReason string, toolCalls []map[string]any) {
	t.Helper()
	message := map[string]any{"role": "assistant", "content": content}
	if toolCalls != nil {
		message["tool_calls"] = toolCalls
	}
	body := map[string]any{
		"id":    "resp-1",
		"model": "test-model",
		"choices": []map[string]any{
			{"index": 0, "finish_reason": finishReason, "message": message},
		},
		"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
	}
	json.NewEncoder(w).Encode(body)
}

func TestRunReturnsImmediatelyWithNoToolCalls(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeChatResponse(t, w, "hello from the model", "stop", nil)
	})

	a := New(client, "test-model")
	result, err := a.Run(context.Background(), "hi", RunOptions{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Text != "hello from the model" {
		t.Errorf("Text = %q", result.Text)
	}
	if len(result.Steps) != 0 {
		t.Errorf("Steps = %+v, want none", result.Steps)
	}
}

func TestRunDispatchesToolCallAndFeedsResultBack(t *testing.T) {
	var callCount int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n == 1 {
			var req map[string]any
			json.NewDecoder(r.Body).Decode(&req)
			writeChatResponse(t, w, "", "tool_calls", []map[string]any{
				{
					"id":   "call_1",
					"type": "function",
					"function": map[string]any{
						"name":      "get_weather",
						"arguments": `{"location":"Tokyo"}`,
					},
				},
			})
			return
		}

		// Second call: verify the tool result message was fed back.
		var req struct {
			Messages []map[string]any `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		foundToolMsg := false
		for _, m := range req.Messages {
			if m["role"] == "tool" && m["tool_call_id"] == "call_1" {
				foundToolMsg = true
				content, _ := m["content"].(string)
				if content == "" {
					t.Errorf("tool message content empty")
				}
			}
		}
		if !foundToolMsg {
			t.Errorf("expected a tool-role message with tool_call_id=call_1, messages = %+v", req.Messages)
		}
		writeChatResponse(t, w, "It's sunny in Tokyo.", "stop", nil)
	})

	a := New(client, "test-model")
	var gotArgs any
	err := a.RegisterTool(&fakeTool{
		name:        "get_weather",
		description: "Get weather",
		parameters:  map[string]any{"type": "object"},
		execute: func(ctx context.Context, args json.RawMessage) (any, error) {
			var parsed map[string]any
			json.Unmarshal(args, &parsed)
			gotArgs = parsed
			return map[string]any{"location": parsed["location"], "tempC": 22}, nil
		},
	})
	if err != nil {
		t.Fatalf("RegisterTool() error = %v", err)
	}

	result, err := a.Run(context.Background(), "What's the weather in Tokyo?", RunOptions{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Text != "It's sunny in Tokyo." {
		t.Errorf("Text = %q", result.Text)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1", len(result.Steps))
	}
	if result.Steps[0].ToolName != "get_weather" {
		t.Errorf("Steps[0].ToolName = %q", result.Steps[0].ToolName)
	}
	if m, ok := gotArgs.(map[string]any); !ok || m["location"] != "Tokyo" {
		t.Errorf("tool received args = %#v", gotArgs)
	}
}

func TestRunEnforcesMaxSteps(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Always calls a tool, never stops.
		writeChatResponse(t, w, "", "tool_calls", []map[string]any{
			{
				"id":   "call_x",
				"type": "function",
				"function": map[string]any{
					"name":      "loop_tool",
					"arguments": `{}`,
				},
			},
		})
	})

	a := New(client, "test-model")
	if err := a.RegisterTool(&fakeTool{
		name:        "loop_tool",
		description: "loops",
		parameters:  map[string]any{"type": "object"},
		execute: func(ctx context.Context, args json.RawMessage) (any, error) {
			return map[string]any{"ok": true}, nil
		},
	}); err != nil {
		t.Fatalf("RegisterTool() error = %v", err)
	}

	_, err := a.Run(context.Background(), "loop forever", RunOptions{MaxSteps: 3})
	if err == nil {
		t.Fatal("expected error for exceeded max steps")
	}
	if !errors.Is(err, ErrAgentMaxStepsExceeded) {
		t.Errorf("error = %v, want wrapping ErrAgentMaxStepsExceeded", err)
	}
}

func TestRunDefaultMaxStepsIsEight(t *testing.T) {
	var callCount int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		writeChatResponse(t, w, "", "tool_calls", []map[string]any{
			{
				"id":       "call_x",
				"type":     "function",
				"function": map[string]any{"name": "loop_tool", "arguments": `{}`},
			},
		})
	})

	a := New(client, "test-model")
	a.RegisterTool(&fakeTool{
		name: "loop_tool", description: "loops", parameters: map[string]any{"type": "object"},
		execute: func(ctx context.Context, args json.RawMessage) (any, error) { return "ok", nil },
	})

	// Zero-value RunOptions{} must resolve MaxSteps to 8, not loop forever
	// and not fail immediately with 0 steps.
	_, err := a.Run(context.Background(), "loop", RunOptions{})
	if !errors.Is(err, ErrAgentMaxStepsExceeded) {
		t.Fatalf("error = %v, want wrapping ErrAgentMaxStepsExceeded", err)
	}
	if got := atomic.LoadInt32(&callCount); got != defaultMaxSteps {
		t.Errorf("chat call count = %d, want %d (default MaxSteps)", got, defaultMaxSteps)
	}
}

func TestRunHandlesMalformedToolArguments(t *testing.T) {
	var callCount int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n == 1 {
			writeChatResponse(t, w, "", "tool_calls", []map[string]any{
				{
					"id":       "call_bad",
					"type":     "function",
					"function": map[string]any{"name": "get_weather", "arguments": `not-json{{{`},
				},
			})
			return
		}
		var req struct {
			Messages []map[string]any `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		last := req.Messages[len(req.Messages)-1]
		content, _ := last["content"].(string)
		if last["role"] != "tool" || content == "" {
			t.Errorf("expected a tool error message fed back, got %+v", last)
		}
		writeChatResponse(t, w, "handled the bad args", "stop", nil)
	})

	a := New(client, "test-model")
	a.RegisterTool(&fakeTool{
		name: "get_weather", description: "x", parameters: map[string]any{"type": "object"},
		execute: func(ctx context.Context, args json.RawMessage) (any, error) {
			t.Fatal("Execute should not be called for malformed arguments")
			return nil, nil
		},
	})

	result, err := a.Run(context.Background(), "bad tool call", RunOptions{})
	if err != nil {
		t.Fatalf("Run() error = %v, want run to continue past malformed args", err)
	}
	if result.Text != "handled the bad args" {
		t.Errorf("Text = %q", result.Text)
	}
	// Malformed-argument calls don't produce a Step (never dispatched).
	if len(result.Steps) != 0 {
		t.Errorf("Steps = %+v, want none", result.Steps)
	}
}

func TestRunHandlesUnknownToolName(t *testing.T) {
	var callCount int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n == 1 {
			writeChatResponse(t, w, "", "tool_calls", []map[string]any{
				{
					"id":       "call_unknown",
					"type":     "function",
					"function": map[string]any{"name": "does_not_exist", "arguments": `{}`},
				},
			})
			return
		}
		writeChatResponse(t, w, "ok", "stop", nil)
	})

	a := New(client, "test-model")
	result, err := a.Run(context.Background(), "call unknown tool", RunOptions{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Text != "ok" {
		t.Errorf("Text = %q", result.Text)
	}
}

func TestRunHandlesToolExecuteError(t *testing.T) {
	var callCount int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n == 1 {
			writeChatResponse(t, w, "", "tool_calls", []map[string]any{
				{
					"id":       "call_err",
					"type":     "function",
					"function": map[string]any{"name": "failing_tool", "arguments": `{}`},
				},
			})
			return
		}
		writeChatResponse(t, w, "recovered", "stop", nil)
	})

	a := New(client, "test-model")
	a.RegisterTool(&fakeTool{
		name: "failing_tool", description: "x", parameters: map[string]any{"type": "object"},
		execute: func(ctx context.Context, args json.RawMessage) (any, error) {
			return nil, errors.New("boom")
		},
	})

	result, err := a.Run(context.Background(), "call failing tool", RunOptions{})
	if err != nil {
		t.Fatalf("Run() error = %v, want run to continue past tool execution error", err)
	}
	if result.Text != "recovered" {
		t.Errorf("Text = %q", result.Text)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1", len(result.Steps))
	}
}

func TestRegisterToolRejectsDuplicateName(t *testing.T) {
	a := New(nil, "test-model")
	tool := &fakeTool{name: "dup", description: "x", parameters: map[string]any{"type": "object"}}

	if err := a.RegisterTool(tool); err != nil {
		t.Fatalf("first RegisterTool() error = %v", err)
	}
	if err := a.RegisterTool(tool); err == nil {
		t.Fatal("expected error registering a duplicate tool name")
	}
}
