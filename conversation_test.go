package veloxquant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConversationSendGrowsHistory(t *testing.T) {
	var gotMessages [][]map[string]string
	reply := "ok"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []map[string]string `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		gotMessages = append(gotMessages, body.Messages)

		json.NewEncoder(w).Encode(map[string]any{
			"id":    "resp",
			"model": "test-model",
			"choices": []map[string]any{
				{"index": 0, "message": map[string]string{"role": "assistant", "content": reply}},
			},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		})
	}))
	defer srv.Close()

	c, err := NewClient(WithOpenAICompatibleRuntime(srv.URL))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	conv := c.NewConversation("test-model", "")

	if _, err := conv.Send(context.Background(), "hello"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if len(conv.History()) != 2 {
		t.Fatalf("after 1 turn: len(History()) = %d, want 2", len(conv.History()))
	}

	reply = "ok again"
	if _, err := conv.Send(context.Background(), "how are you"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if len(conv.History()) != 4 {
		t.Fatalf("after 2 turns: len(History()) = %d, want 4", len(conv.History()))
	}

	if len(gotMessages) != 2 {
		t.Fatalf("server saw %d requests, want 2", len(gotMessages))
	}
	if len(gotMessages[1]) != 3 {
		t.Fatalf("second request carried %d messages, want 3 (user, assistant, user)", len(gotMessages[1]))
	}

	history := conv.History()
	if history[0].Role != "user" || history[0].Content != "hello" {
		t.Errorf("history[0] = %+v, want user/hello", history[0])
	}
	if history[1].Role != "assistant" || history[1].Content != "ok" {
		t.Errorf("history[1] = %+v, want assistant/ok", history[1])
	}
	if history[3].Content != "ok again" {
		t.Errorf("history[3].Content = %q, want %q", history[3].Content, "ok again")
	}
}

func TestConversationSendWithSystemPrompt(t *testing.T) {
	var gotMessages []map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []map[string]string `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		gotMessages = body.Messages

		json.NewEncoder(w).Encode(map[string]any{
			"id":    "resp",
			"model": "test-model",
			"choices": []map[string]any{
				{"index": 0, "message": map[string]string{"role": "assistant", "content": "hi"}},
			},
		})
	}))
	defer srv.Close()

	c, err := NewClient(WithOpenAICompatibleRuntime(srv.URL))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	conv := c.NewConversation("test-model", "you are a helpful assistant")
	if _, err := conv.Send(context.Background(), "hello"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if len(gotMessages) != 2 {
		t.Fatalf("got %d messages, want 2 (system, user)", len(gotMessages))
	}
	if gotMessages[0]["role"] != "system" {
		t.Errorf("messages[0].role = %q, want %q", gotMessages[0]["role"], "system")
	}
}

func TestConversationSendErrorDoesNotCorruptHistory(t *testing.T) {
	fail := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"boom"}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "resp",
			"model": "test-model",
			"choices": []map[string]any{
				{"index": 0, "message": map[string]string{"role": "assistant", "content": "recovered"}},
			},
		})
	}))
	defer srv.Close()

	c, err := NewClient(WithOpenAICompatibleRuntime(srv.URL))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	conv := c.NewConversation("test-model", "")

	if _, err := conv.Send(context.Background(), "hello"); err == nil {
		t.Fatal("expected error from failed turn, got nil")
	}
	if len(conv.History()) != 0 {
		t.Fatalf("after failed turn: len(History()) = %d, want 0", len(conv.History()))
	}

	fail = false
	if _, err := conv.Send(context.Background(), "hello again"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if len(conv.History()) != 2 {
		t.Fatalf("after successful retry: len(History()) = %d, want 2", len(conv.History()))
	}
	if conv.History()[0].Content != "hello again" {
		t.Errorf("history[0].Content = %q, want %q", conv.History()[0].Content, "hello again")
	}
}

func TestConversationSendStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		chunks := []string{"hel", "lo"}
		for i, c := range chunks {
			finish := ""
			if i == len(chunks)-1 {
				finish = `,"finish_reason":"stop"`
			}
			w.Write([]byte(`data: {"id":"resp","model":"test-model","choices":[{"index":0,"delta":{"content":"` + c + `"}` + finish + `}]}` + "\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
		}
		w.Write([]byte("data: [DONE]\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer srv.Close()

	c, err := NewClient(WithOpenAICompatibleRuntime(srv.URL))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	conv := c.NewConversation("test-model", "")
	stream, err := conv.SendStream(context.Background(), "hi")
	if err != nil {
		t.Fatalf("SendStream() error = %v", err)
	}

	var got string
	for stream.Next() {
		got += stream.Chunk().Text
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream error = %v", err)
	}
	stream.Close()

	if got != "hello" {
		t.Errorf("streamed text = %q, want %q", got, "hello")
	}
	if len(conv.History()) != 2 {
		t.Fatalf("after stream: len(History()) = %d, want 2", len(conv.History()))
	}
	if conv.History()[1].Content != "hello" {
		t.Errorf("history[1].Content = %q, want %q", conv.History()[1].Content, "hello")
	}
}
