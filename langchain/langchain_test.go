package langchain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	veloxquant "github.com/rajveer43/veloxquant-go"
	"github.com/tmc/langchaingo/llms"
)

func newTestLLM(t *testing.T, handler http.HandlerFunc) *LLM {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := veloxquant.NewClient(veloxquant.WithOpenAICompatibleRuntime(srv.URL))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return New(client, "test-model")
}

func TestGenerateContent(t *testing.T) {
	var gotBody map[string]any
	llm := newTestLLM(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "resp-1",
			"model": "test-model",
			"choices": []map[string]any{
				{"index": 0, "message": map[string]string{"role": "assistant", "content": "hi there"}},
			},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 2, "total_tokens": 3},
		})
	})

	resp, err := llm.GenerateContent(context.Background(), []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, "be helpful"),
		llms.TextParts(llms.ChatMessageTypeHuman, "hello"),
	})
	if err != nil {
		t.Fatalf("GenerateContent() error = %v", err)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Content != "hi there" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	messages, _ := gotBody["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("sent %d messages, want 2", len(messages))
	}
	first := messages[0].(map[string]any)
	if first["role"] != "system" {
		t.Errorf("messages[0].role = %v, want system", first["role"])
	}
	second := messages[1].(map[string]any)
	if second["role"] != "user" {
		t.Errorf("messages[1].role = %v, want user", second["role"])
	}
}

func TestGenerateContentUsesWithModel(t *testing.T) {
	var gotModel string
	llm := newTestLLM(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "resp-1",
			"model": body.Model,
			"choices": []map[string]any{
				{"index": 0, "message": map[string]string{"role": "assistant", "content": "ok"}},
			},
		})
	})

	_, err := llm.GenerateContent(context.Background(),
		[]llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, "hi")},
		llms.WithModel("override-model"),
	)
	if err != nil {
		t.Fatalf("GenerateContent() error = %v", err)
	}
	if gotModel != "override-model" {
		t.Errorf("model sent = %q, want %q", gotModel, "override-model")
	}
}

func TestGenerateContentRejectsNonTextParts(t *testing.T) {
	llm := newTestLLM(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called for a rejected request")
	})

	_, err := llm.GenerateContent(context.Background(), []llms.MessageContent{
		{Role: llms.ChatMessageTypeHuman, Parts: []llms.ContentPart{llms.ImageURLPart("https://example.com/x.png")}},
	})
	if err == nil {
		t.Fatal("expected error for non-text content part, got nil")
	}
}

func TestCall(t *testing.T) {
	llm := newTestLLM(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "resp-1",
			"model": "test-model",
			"choices": []map[string]any{
				{"index": 0, "message": map[string]string{"role": "assistant", "content": "called"}},
			},
		})
	})

	out, err := llm.Call(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if out != "called" {
		t.Errorf("Call() = %q, want %q", out, "called")
	}
}

func TestGenerateContentStreaming(t *testing.T) {
	llm := newTestLLM(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		w.Write([]byte(`data: {"id":"r","model":"test-model","choices":[{"index":0,"delta":{"content":"a"}}]}` + "\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		w.Write([]byte(`data: {"id":"r","model":"test-model","choices":[{"index":0,"delta":{"content":"b"},"finish_reason":"stop"}]}` + "\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		w.Write([]byte("data: [DONE]\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	})

	var streamed string
	resp, err := llm.GenerateContent(context.Background(),
		[]llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, "hi")},
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			streamed += string(chunk)
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("GenerateContent() error = %v", err)
	}
	if streamed != "ab" {
		t.Errorf("streamed = %q, want %q", streamed, "ab")
	}
	if resp.Choices[0].Content != "ab" {
		t.Errorf("final content = %q, want %q", resp.Choices[0].Content, "ab")
	}
}

func TestModelSatisfiesInterface(t *testing.T) {
	var _ llms.Model = (*LLM)(nil)
}
