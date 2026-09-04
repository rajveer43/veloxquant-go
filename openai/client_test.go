package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientChatCompletion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %s, want /chat/completions", r.URL.Path)
		}
		var req ChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Stream {
			t.Error("expected Stream = false for non-streaming request")
		}

		json.NewEncoder(w).Encode(ChatResponse{
			ID:    "resp-1",
			Model: req.Model,
			Choices: []ChatChoice{
				{Index: 0, Message: Message{Role: "assistant", Content: "hello"}},
			},
			Usage: Usage{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, 5*time.Second)

	resp, err := c.ChatCompletion(context.Background(), ChatRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "hello" {
		t.Errorf("unexpected response: %+v", resp)
	}
	if resp.Usage.TotalTokens != 8 {
		t.Errorf("TotalTokens = %d, want 8", resp.Usage.TotalTokens)
	}
}

func TestClientEmbeddings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Errorf("path = %s, want /embeddings", r.URL.Path)
		}
		var req EmbeddingsRequest
		json.NewDecoder(r.Body).Decode(&req)

		json.NewEncoder(w).Encode(EmbeddingsResponse{
			Model: req.Model,
			Data: []Embedding{
				{Index: 0, Embedding: []float64{0.1, 0.2, 0.3}},
			},
			Usage: Usage{PromptTokens: 4, TotalTokens: 4},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, 5*time.Second)

	resp, err := c.Embeddings(context.Background(), EmbeddingsRequest{
		Model: "test-embed-model",
		Input: "hello world",
	})
	if err != nil {
		t.Fatalf("Embeddings() error = %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("len(Data) = %d, want 1", len(resp.Data))
	}
	if len(resp.Data[0].Embedding) != 3 {
		t.Errorf("len(Embedding) = %d, want 3", len(resp.Data[0].Embedding))
	}
	if resp.Usage.TotalTokens != 4 {
		t.Errorf("TotalTokens = %d, want 4", resp.Usage.TotalTokens)
	}
}

func TestClientModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ModelList{Data: []Model{{ID: "m1"}, {ID: "m2"}}})
	}))
	defer srv.Close()

	c := New(srv.URL, 5*time.Second)
	models, err := c.Models(context.Background())
	if err != nil {
		t.Fatalf("Models() error = %v", err)
	}
	if len(models) != 2 {
		t.Errorf("len(models) = %d, want 2", len(models))
	}
}
