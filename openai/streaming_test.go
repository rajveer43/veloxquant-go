package openai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func writeSSE(w http.ResponseWriter, chunks []string) {
	flusher, _ := w.(http.Flusher)
	for _, c := range chunks {
		fmt.Fprintf(w, "data: %s\n\n", c)
		if flusher != nil {
			flusher.Flush()
		}
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}

func TestChatCompletionStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		_ = req

		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, []string{
			`{"id":"1","model":"m","choices":[{"index":0,"delta":{"content":"Hello"}}]}`,
			`{"id":"1","model":"m","choices":[{"index":0,"delta":{"content":" world"}}]}`,
		})
	}))
	defer srv.Close()

	c := New(srv.URL, 5*time.Second)

	stream, err := c.ChatCompletionStream(context.Background(), ChatRequest{
		Model: "m",
		Messages: []Message{
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletionStream() error = %v", err)
	}
	defer stream.Close()

	var text string
	count := 0
	for stream.Next() {
		text += stream.Chunk().Choices[0].Delta.Content
		count++
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream error: %v", err)
	}

	if count != 2 {
		t.Errorf("received %d chunks, want 2", count)
	}
	if text != "Hello world" {
		t.Errorf("text = %q, want %q", text, "Hello world")
	}
}

func TestChatCompletionStreamHandlesMalformedChunk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: not-json\n\n")
	}))
	defer srv.Close()

	c := New(srv.URL, 5*time.Second)

	stream, err := c.ChatCompletionStream(context.Background(), ChatRequest{Model: "m"})
	if err != nil {
		t.Fatalf("ChatCompletionStream() error = %v", err)
	}
	defer stream.Close()

	for stream.Next() {
	}

	if stream.Err() == nil {
		t.Fatal("expected error for malformed chunk")
	}
}

func TestChatCompletionStreamRespectsContextCancellation(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		<-block
	}))
	defer srv.Close()
	defer close(block)

	c := New(srv.URL, 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	stream, err := c.ChatCompletionStream(ctx, ChatRequest{Model: "m"})
	if err != nil {
		t.Fatalf("ChatCompletionStream() error = %v", err)
	}
	defer stream.Close()

	cancel()

	for stream.Next() {
	}
}
