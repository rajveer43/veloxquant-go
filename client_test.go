package veloxquant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClientDefaults(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if c.cfg.httpTimeout != defaultHTTPTimeout {
		t.Errorf("httpTimeout = %v, want %v", c.cfg.httpTimeout, defaultHTTPTimeout)
	}
	if c.cfg.runtimeURL == "" {
		t.Error("expected non-empty default runtimeURL")
	}
}

func TestNewClientRejectsInvalidTimeout(t *testing.T) {
	_, err := NewClient(WithHTTPTimeout(-1))
	if err == nil {
		t.Fatal("expected error for negative timeout")
	}
}

func TestNewClientWithOptions(t *testing.T) {
	c, err := NewClient(
		WithRuntimeURL("http://example.com:1234"),
		WithAutoDetect(),
		WithProfile("balanced"),
		WithHTTPTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if c.cfg.runtimeURL != "http://example.com:1234" {
		t.Errorf("runtimeURL = %q", c.cfg.runtimeURL)
	}
	if !c.cfg.autoDetect {
		t.Error("expected autoDetect true")
	}
	if c.cfg.httpTimeout != 5*time.Second {
		t.Errorf("httpTimeout = %v, want 5s", c.cfg.httpTimeout)
	}
}

func TestClientSystemInfo(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	info, err := c.System.Info(context.Background())
	if err != nil {
		t.Fatalf("System.Info() error = %v", err)
	}
	if info.Platform == "" {
		t.Error("expected non-empty Platform")
	}
}

func TestClientMemoryEstimate(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	est, err := c.Memory.Estimate(context.Background(), MemoryRequest{
		Model: ModelArchitecture{
			NumLayers:      32,
			NumKVHeads:     8,
			HeadDim:        128,
			HiddenSize:     4096,
			ParameterCount: 7_000_000_000,
		},
		ContextLength: 4096,
		Precision:     FP16,
	})
	if err != nil {
		t.Fatalf("Memory.Estimate() error = %v", err)
	}
	if est.TotalMemoryBytes == 0 {
		t.Error("expected non-zero TotalMemoryBytes")
	}
}

func TestClientChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "resp-1",
			"model": "test-model",
			"choices": []map[string]any{
				{"index": 0, "message": map[string]string{"role": "assistant", "content": "hi there"}},
			},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 2, "total_tokens": 3},
		})
	}))
	defer srv.Close()

	c, err := NewClient(WithOpenAICompatibleRuntime(srv.URL))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	resp, err := c.Chat(context.Background(), ChatRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.Text != "hi there" {
		t.Errorf("Text = %q, want %q", resp.Text, "hi there")
	}
	if resp.Usage.TotalTokens != 3 {
		t.Errorf("TotalTokens = %d, want 3", resp.Usage.TotalTokens)
	}
}

func TestClientRuntimeHealthUnavailable(t *testing.T) {
	c, err := NewClient(WithRuntimeURL("http://127.0.0.1:1"), WithHTTPTimeout(500*time.Millisecond))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = c.Runtime.Health(context.Background())
	if err == nil {
		t.Fatal("expected error for unreachable runtime")
	}
}

func TestFormatBytesTopLevel(t *testing.T) {
	if got := FormatBytes(1024); got != "1.0 KB" {
		t.Errorf("FormatBytes(1024) = %q, want 1.0 KB", got)
	}
}
