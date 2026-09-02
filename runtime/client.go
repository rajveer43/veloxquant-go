// Package runtime implements the HTTP client used to communicate with a
// local VeloxQuant runtime process (typically at http://localhost:8765).
package runtime

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/rajveer43/veloxquant-go/internal/httpclient"
)

// DefaultURL is the default address of a local VeloxQuant runtime.
const DefaultURL = "http://localhost:8765"

// Client talks to a VeloxQuant runtime over HTTP.
type Client struct {
	http *httpclient.Client
}

// New returns a runtime Client pointed at baseURL with the given timeout.
func New(baseURL string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = DefaultURL
	}
	return &Client{http: httpclient.New(baseURL, timeout)}
}

// healthResponse mirrors the JSON shape returned by the runtime's /health
// endpoint.
type healthResponse struct {
	Healthy bool   `json:"healthy"`
	Version string `json:"version"`
	Engine  string `json:"engine"`
}

// Health checks whether the VeloxQuant runtime is reachable and healthy.
// A connection failure is reported as an unhealthy Status alongside a
// wrapped ErrRuntimeUnavailable-compatible error, rather than only an error,
// so callers can inspect Status directly if they choose to ignore the error.
func (c *Client) Health(ctx context.Context) (Status, error) {
	var resp healthResponse

	if err := c.http.DoJSON(ctx, "GET", "/health", nil, &resp); err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) || errors.Is(err, context.DeadlineExceeded) {
			return Status{}, fmt.Errorf("check runtime health: %w", ErrUnavailable)
		}
		return Status{}, fmt.Errorf("check runtime health: %w", err)
	}

	return Status{
		Healthy: resp.Healthy,
		Version: resp.Version,
		Engine:  resp.Engine,
	}, nil
}

// HTTP exposes the underlying shared HTTP client for use by other packages
// (e.g. openai) that need to talk to the same runtime base URL.
func (c *Client) HTTP() *httpclient.Client {
	return c.http
}
