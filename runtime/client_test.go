package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientHealthHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("path = %s, want /health", r.URL.Path)
		}
		json.NewEncoder(w).Encode(healthResponse{Healthy: true, Version: "1.0.0", Engine: "mlx"})
	}))
	defer srv.Close()

	c := New(srv.URL, 5*time.Second)
	status, err := c.Health(context.Background())
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if !status.Healthy {
		t.Error("expected Healthy = true")
	}
	if status.Version != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", status.Version)
	}
	if status.Engine != "mlx" {
		t.Errorf("Engine = %q, want mlx", status.Engine)
	}
}

func TestClientHealthUnreachable(t *testing.T) {
	c := New("http://127.0.0.1:1", 500*time.Millisecond)

	_, err := c.Health(context.Background())
	if err == nil {
		t.Fatal("expected error for unreachable runtime")
	}
}

func TestClientHealthRespectsContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	c := New(srv.URL, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := c.Health(ctx)
	if err == nil {
		t.Fatal("expected error for expired context")
	}
}

func TestNewDefaultsToDefaultURL(t *testing.T) {
	c := New("", time.Second)
	if c.http.BaseURL != DefaultURL {
		t.Errorf("BaseURL = %q, want %q", c.http.BaseURL, DefaultURL)
	}
}
