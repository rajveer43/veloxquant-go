package veloxquant

import (
	"time"

	"github.com/rajveer43/veloxquant-go/optimize"
	"github.com/rajveer43/veloxquant-go/runtime"
)

const defaultHTTPTimeout = 60 * time.Second

// config holds Client configuration assembled from functional Options.
type config struct {
	runtimeURL    string
	openAIBaseURL string
	httpTimeout   time.Duration
	autoDetect    bool
	profile       optimize.Profile
}

func defaultConfig() config {
	return config{
		runtimeURL:  runtime.DefaultURL,
		httpTimeout: defaultHTTPTimeout,
	}
}

// Option configures a Client. Use the With* functions to build options.
type Option func(*config)

// WithRuntimeURL sets the base URL of the VeloxQuant runtime. Defaults to
// http://localhost:8765.
func WithRuntimeURL(url string) Option {
	return func(c *config) { c.runtimeURL = url }
}

// WithOpenAICompatibleRuntime configures the client to send chat requests
// to an OpenAI-compatible endpoint (e.g. "http://localhost:8765/v1")
// instead of the native VeloxQuant runtime API.
func WithOpenAICompatibleRuntime(baseURL string) Option {
	return func(c *config) { c.openAIBaseURL = baseURL }
}

// WithAutoDetect enables automatic hardware detection and profile
// selection when the Client is constructed.
func WithAutoDetect() Option {
	return func(c *config) { c.autoDetect = true }
}

// WithProfile forces a specific VeloxQuant optimization profile rather
// than letting the SDK choose one automatically.
func WithProfile(profile string) Option {
	return func(c *config) { c.profile = optimize.Profile(profile) }
}

// WithHTTPTimeout sets the timeout used for HTTP requests to the runtime.
func WithHTTPTimeout(timeout time.Duration) Option {
	return func(c *config) { c.httpTimeout = timeout }
}
