package openai

import (
	"context"
	"fmt"
	"time"

	"github.com/rajveer43/veloxquant-go/internal/httpclient"
)

// Client is a minimal OpenAI-compatible HTTP client.
type Client struct {
	http *httpclient.Client
}

// New returns a Client pointed at baseURL (e.g.
// "http://localhost:8765/v1") with the given timeout.
func New(baseURL string, timeout time.Duration) *Client {
	return &Client{http: httpclient.New(baseURL, timeout)}
}

// ChatCompletion performs a non-streaming chat completion request against
// POST /v1/chat/completions.
func (c *Client) ChatCompletion(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	req.Stream = false

	var resp ChatResponse
	if err := c.http.DoJSON(ctx, "POST", "/chat/completions", req, &resp); err != nil {
		return ChatResponse{}, fmt.Errorf("chat completion: %w", err)
	}
	return resp, nil
}

// Models lists models available on the runtime via GET /v1/models.
func (c *Client) Models(ctx context.Context) ([]Model, error) {
	var resp ModelList
	if err := c.http.DoJSON(ctx, "GET", "/models", nil, &resp); err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	return resp.Data, nil
}
