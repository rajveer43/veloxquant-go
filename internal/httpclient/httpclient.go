// Package httpclient provides a small, shared HTTP client wrapper used by
// the runtime and openai packages: context-aware requests, JSON encoding
// helpers, and typed error responses.
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps *http.Client with a base URL and JSON convenience methods.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// New returns a Client configured with the given base URL and timeout.
func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// StatusError represents a non-2xx HTTP response, carrying the response
// body so callers can surface structured API error messages.
type StatusError struct {
	StatusCode int
	Body       string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("http %d: %s", e.StatusCode, e.Body)
}

// DoJSON issues an HTTP request with an optional JSON-encoded body and
// decodes a JSON response into out (if out is non-nil).
func (c *Client) DoJSON(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader

	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &StatusError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode response body: %w", err)
		}
	}

	return nil
}

// Do issues an HTTP request and returns the raw *http.Response for callers
// that need to stream the body (e.g. Server-Sent Events). The caller is
// responsible for closing the response body.
func (c *Client) Do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reader io.Reader

	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, &StatusError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	return resp, nil
}
