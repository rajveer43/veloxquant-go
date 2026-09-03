package veloxquant

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rajveer43/veloxquant-go/monitor"
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

func TestClientChatPopulatesTokensPerSecond(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "resp-1",
			"model": "test-model",
			"choices": []map[string]any{
				{"index": 0, "message": map[string]string{"role": "assistant", "content": "hi there"}},
			},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 10, "total_tokens": 11},
		})
	}))
	defer srv.Close()

	c, err := NewClient(WithOpenAICompatibleRuntime(srv.URL))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	resp, err := c.Chat(context.Background(), ChatRequest{Model: "test-model"})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.Metrics.TokensPerSecond <= 0 {
		t.Errorf("TokensPerSecond = %f, want > 0", resp.Metrics.TokensPerSecond)
	}
}

func writeSSEChunks(w http.ResponseWriter, chunks []string) {
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

func TestClientChatStreamMetrics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSEChunks(w, []string{
			`{"id":"1","model":"m","choices":[{"index":0,"delta":{"content":"Hello"}}]}`,
			`{"id":"1","model":"m","choices":[{"index":0,"delta":{"content":" world"}}]}`,
		})
	}))
	defer srv.Close()

	c, err := NewClient(WithOpenAICompatibleRuntime(srv.URL))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	stream, err := c.ChatStream(context.Background(), ChatRequest{Model: "m"})
	if err != nil {
		t.Fatalf("ChatStream() error = %v", err)
	}
	defer stream.Close()

	for stream.Next() {
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream error: %v", err)
	}

	m := stream.Metrics()
	if m.TokensPerSecond <= 0 {
		t.Errorf("TokensPerSecond = %f, want > 0", m.TokensPerSecond)
	}
	if m.TimeToFirstToken <= 0 {
		t.Errorf("TimeToFirstToken = %v, want > 0", m.TimeToFirstToken)
	}
}

func TestClientChatFeedsMonitor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "resp-1",
			"model":   "test-model",
			"choices": []map[string]any{{"index": 0, "message": map[string]string{"role": "assistant", "content": "hi"}}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 5, "total_tokens": 6},
		})
	}))
	defer srv.Close()

	c, err := NewClient(WithOpenAICompatibleRuntime(srv.URL))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	mon := c.Monitor(WithMonitorInterval(time.Hour)) // long interval: only Report should fire
	defer mon.Stop()

	updates := make(chan monitor.Metrics, 1)
	mon.Subscribe(func(m monitor.Metrics) {
		select {
		case updates <- m:
		default:
		}
	})

	if _, err := c.Chat(context.Background(), ChatRequest{Model: "test-model"}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	select {
	case m := <-updates:
		if m.TokensPerSecond <= 0 {
			t.Errorf("TokensPerSecond = %f, want > 0", m.TokensPerSecond)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for monitor update from Chat")
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
