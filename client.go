// Package veloxquant is the VeloxQuant Go SDK: memory intelligence and
// optimization for local AI on Apple Silicon and beyond. It hides MLX and
// Python runtime implementation details behind an idiomatic Go API for
// hardware detection, memory/KV-cache estimation, optimization profile
// selection, and communication with a local VeloxQuant runtime.
package veloxquant

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rajveer43/veloxquant-go/memory"
	"github.com/rajveer43/veloxquant-go/models"
	"github.com/rajveer43/veloxquant-go/monitor"
	"github.com/rajveer43/veloxquant-go/openai"
	"github.com/rajveer43/veloxquant-go/optimize"
	"github.com/rajveer43/veloxquant-go/runtime"
	"github.com/rajveer43/veloxquant-go/system"
)

// SystemService exposes hardware and platform detection.
type SystemService struct {
	detector system.Detector
}

// Info returns details about the host system.
func (s *SystemService) Info(ctx context.Context) (SystemInfo, error) {
	info, err := s.detector.Info(ctx)
	if err != nil {
		return SystemInfo{}, fmt.Errorf("system info: %w", err)
	}
	return SystemInfo(info), nil
}

// MemoryService exposes model and KV-cache memory estimation.
type MemoryService struct {
	estimator memory.Estimator
}

// Estimate computes memory requirements for a model at a given context
// length and precision.
func (m *MemoryService) Estimate(ctx context.Context, req MemoryRequest) (MemoryEstimate, error) {
	if err := ctx.Err(); err != nil {
		return MemoryEstimate{}, err
	}

	est, err := m.estimator.Estimate(memory.Request{
		Architecture:  req.Model,
		ContextLength: req.ContextLength,
		Precision:     req.Precision,
	})
	if err != nil {
		return MemoryEstimate{}, fmt.Errorf("estimate memory: %w", err)
	}

	return MemoryEstimate(est), nil
}

// OptimizeService exposes VeloxQuant optimization profile recommendations.
type OptimizeService struct {
	optimizer optimize.Optimizer
}

// OptimizationRequest describes an optimization recommendation query.
type OptimizationRequest struct {
	Model         string
	Architecture  ModelArchitecture
	ContextLength int
}

// OptimizationRecommendation is VeloxQuant's suggested optimization
// strategy for a model/context combination.
type OptimizationRecommendation struct {
	Profile optimize.Profile

	CompressionMethod string
	CompressionBits   int

	EstimatedMemoryBefore uint64
	EstimatedMemoryAfter  uint64

	ContextLength int

	Reason string
}

// Recommend returns VeloxQuant's recommended optimization strategy for the
// given model and context length.
func (o *OptimizeService) Recommend(ctx context.Context, req OptimizationRequest) (OptimizationRecommendation, error) {
	rec, err := o.optimizer.Recommend(ctx, optimize.Request{
		ModelName:     req.Model,
		Architecture:  req.Architecture,
		ContextLength: req.ContextLength,
	})
	if err != nil {
		return OptimizationRecommendation{}, fmt.Errorf("optimize recommend: %w", err)
	}
	return OptimizationRecommendation(rec), nil
}

// RuntimeService exposes communication with a local VeloxQuant runtime.
type RuntimeService struct {
	client *runtime.Client
}

// RuntimeStatus describes the health of a VeloxQuant runtime instance.
type RuntimeStatus struct {
	Healthy bool
	Version string
	Engine  string
}

// Health checks whether the VeloxQuant runtime is reachable and healthy.
func (r *RuntimeService) Health(ctx context.Context) (RuntimeStatus, error) {
	status, err := r.client.Health(ctx)
	if err != nil {
		return RuntimeStatus{}, err
	}
	return RuntimeStatus(status), nil
}

// ModelsService exposes the VeloxQuant model registry.
type ModelsService struct {
	registry  models.Registry
	estimator memory.Estimator
}

// List returns all known models.
func (m *ModelsService) List() []models.Info {
	return m.registry.List()
}

// ModelRecommendationRequest describes a model recommendation query.
type ModelRecommendationRequest struct {
	Task                 string
	AvailableMemoryBytes uint64
	ContextLength        int
}

// Local scans the local model cache directory (e.g. the MLX/Hugging Face
// hub cache) and reports the models found on disk, their size, and when
// they were last modified. It returns an empty slice, not an error, if the
// cache directory doesn't exist or can't be read.
func (m *ModelsService) Local(ctx context.Context) ([]LocalModelInfo, error) {
	localModels, err := models.ScanLocal(ctx)
	if err != nil {
		return nil, err
	}

	infos := make([]LocalModelInfo, len(localModels))
	for i, lm := range localModels {
		info := LocalModelInfo{
			Name:      lm.Name,
			Path:      lm.Path,
			SizeBytes: lm.SizeBytes,
		}
		if !lm.LastModified.IsZero() {
			lastModified := lm.LastModified
			info.LastModified = &lastModified
		}
		infos[i] = info
	}
	return infos, nil
}

// Recommend returns models suited to the requested task that fit within
// AvailableMemoryBytes, ranked best first.
func (m *ModelsService) Recommend(ctx context.Context, req ModelRecommendationRequest) ([]models.Info, error) {
	return models.Recommend(ctx, m.registry, m.estimator, models.RecommendationRequest{
		Task:                 models.Task(req.Task),
		AvailableMemoryBytes: req.AvailableMemoryBytes,
		ContextLength:        req.ContextLength,
	})
}

// RecommendScored behaves like Recommend but also returns the score and
// human-readable reasoning behind each candidate's ranking.
func (m *ModelsService) RecommendScored(ctx context.Context, req ModelRecommendationRequest) ([]models.Scored, error) {
	return models.RecommendScored(ctx, m.registry, m.estimator, models.RecommendationRequest{
		Task:                 models.Task(req.Task),
		AvailableMemoryBytes: req.AvailableMemoryBytes,
		ContextLength:        req.ContextLength,
	})
}

// Client is the main entry point to the VeloxQuant Go SDK. Construct one
// with NewClient. Client is safe for concurrent use.
type Client struct {
	cfg config

	System   *SystemService
	Memory   *MemoryService
	Optimize *OptimizeService
	Runtime  *RuntimeService
	Models   *ModelsService

	runtimeClient *runtime.Client
	openaiClient  *openai.Client

	metricsMu   sync.RWMutex
	metricsSink func(monitor.Metrics)
}

// setMetricsSink registers the sink that Chat and ChatStream report live
// inference metrics to. Passing nil disables reporting. It is safe for
// concurrent use.
func (c *Client) setMetricsSink(sink func(monitor.Metrics)) {
	c.metricsMu.Lock()
	c.metricsSink = sink
	c.metricsMu.Unlock()
}

func (c *Client) reportMetrics(m monitor.Metrics) {
	c.metricsMu.RLock()
	sink := c.metricsSink
	c.metricsMu.RUnlock()
	if sink != nil {
		sink(m)
	}
}

// NewClient constructs a VeloxQuant Client. By default it connects to a
// runtime at http://localhost:8765 with a 60s HTTP timeout; use the With*
// options to customize behavior.
func NewClient(opts ...Option) (*Client, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.httpTimeout <= 0 {
		return nil, fmt.Errorf("new client: %w: http timeout must be positive", ErrInvalidConfig)
	}

	runtimeClient := runtime.New(cfg.runtimeURL, cfg.httpTimeout)

	var openaiClient *openai.Client
	if cfg.openAIBaseURL != "" {
		openaiClient = openai.New(cfg.openAIBaseURL, cfg.httpTimeout)
	} else {
		openaiClient = openai.New(cfg.runtimeURL+"/v1", cfg.httpTimeout)
	}

	estimator := memory.NewEstimator()
	registry := models.NewRegistry()

	c := &Client{
		cfg: cfg,

		System:   &SystemService{detector: system.NewDetector()},
		Memory:   &MemoryService{estimator: estimator},
		Optimize: &OptimizeService{optimizer: optimize.NewOptimizer(estimator)},
		Runtime:  &RuntimeService{client: runtimeClient},
		Models:   &ModelsService{registry: registry, estimator: estimator},

		runtimeClient: runtimeClient,
		openaiClient:  openaiClient,
	}

	return c, nil
}

// Chat sends a chat completion request to the configured runtime and
// returns the full response.
func (c *Client) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	start := time.Now()

	resp, err := c.openaiClient.ChatCompletion(ctx, toOpenAIRequest(req))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("chat: %w", err)
	}

	var text string
	if len(resp.Choices) > 0 {
		text = resp.Choices[0].Message.Content
	}

	elapsed := time.Since(start)

	metrics := InferenceMetrics{TotalDuration: elapsed}
	if resp.Usage.CompletionTokens > 0 && elapsed > 0 {
		metrics.TokensPerSecond = float64(resp.Usage.CompletionTokens) / elapsed.Seconds()
	}

	chatResp := ChatResponse{
		ID:    resp.ID,
		Model: resp.Model,
		Text:  text,
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
		Metrics: metrics,
	}

	c.reportMetrics(monitor.Metrics{
		TokensPerSecond:  metrics.TokensPerSecond,
		TimeToFirstToken: metrics.TimeToFirstToken,
	})

	return chatResp, nil
}

// ChatStream is a handle to a streaming chat completion. Call Next to
// advance, Chunk to read the current piece of text, and Err to check for
// errors after iteration ends. Always call Close when done. Once the
// stream is finished (Next returns false with a nil Err), Metrics reports
// the completed request's tokens/sec and time-to-first-token.
type ChatStream struct {
	inner *openai.Stream
	chunk ChatChunk

	start        time.Time
	firstTokenAt time.Time
	tokenChunks  int
	sink         func(monitor.Metrics)
	reported     bool
}

// Next advances the stream. It returns false when the stream ends (check
// Err for failures).
func (s *ChatStream) Next() bool {
	if !s.inner.Next() {
		s.reportMetrics()
		return false
	}
	chunk := s.inner.Chunk()

	var text string
	done := false
	if len(chunk.Choices) > 0 {
		text = chunk.Choices[0].Delta.Content
		done = chunk.Choices[0].FinishReason != nil
	}

	if text != "" {
		if s.tokenChunks == 0 {
			s.firstTokenAt = time.Now()
		}
		s.tokenChunks++
	}

	s.chunk = ChatChunk{Text: text, Done: done}
	return true
}

// Chunk returns the most recently read chunk.
func (s *ChatStream) Chunk() ChatChunk {
	return s.chunk
}

// Err returns the first error encountered while streaming, if any.
func (s *ChatStream) Err() error {
	return s.inner.Err()
}

// Metrics reports performance characteristics of the stream so far.
// TokensPerSecond and TimeToFirstToken are approximate: they're derived
// from the number of non-empty content chunks and wall-clock time, since
// OpenAI-compatible streaming responses don't report per-chunk token
// counts.
func (s *ChatStream) Metrics() InferenceMetrics {
	m := InferenceMetrics{TotalDuration: time.Since(s.start)}
	if !s.firstTokenAt.IsZero() {
		m.TimeToFirstToken = s.firstTokenAt.Sub(s.start)
	}
	if s.tokenChunks > 0 && m.TotalDuration > 0 {
		m.TokensPerSecond = float64(s.tokenChunks) / m.TotalDuration.Seconds()
	}
	return m
}

func (s *ChatStream) reportMetrics() {
	if s.reported || s.sink == nil {
		return
	}
	s.reported = true
	m := s.Metrics()
	s.sink(monitor.Metrics{
		TokensPerSecond:  m.TokensPerSecond,
		TimeToFirstToken: m.TimeToFirstToken,
	})
}

// Close releases the underlying connection.
func (s *ChatStream) Close() error {
	return s.inner.Close()
}

// ChatStream starts a streaming chat completion request.
func (c *Client) ChatStream(ctx context.Context, req ChatRequest) (*ChatStream, error) {
	stream, err := c.openaiClient.ChatCompletionStream(ctx, toOpenAIRequest(req))
	if err != nil {
		return nil, fmt.Errorf("chat stream: %w", err)
	}
	return &ChatStream{
		inner: stream,
		start: time.Now(),
		sink:  c.reportMetrics,
	}, nil
}

func toOpenAIRequest(req ChatRequest) openai.ChatRequest {
	messages := make([]openai.Message, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = openai.Message{Role: m.Role, Content: m.Content}
	}
	return openai.ChatRequest{
		Model:          req.Model,
		Messages:       messages,
		Temperature:    req.Temperature,
		MaxTokens:      req.MaxTokens,
		ResponseFormat: req.ResponseFormat,
	}
}

const defaultMonitorInterval = 5 * time.Second

// MonitorConfig configures a Monitor returned by Client.Monitor.
type MonitorConfig struct {
	interval time.Duration
}

// MonitorOption configures a Monitor. Use the WithMonitor* functions to
// build options.
type MonitorOption func(*MonitorConfig)

// WithMonitorInterval sets how often the Monitor samples system memory.
// Defaults to 5 seconds.
func WithMonitorInterval(interval time.Duration) MonitorOption {
	return func(c *MonitorConfig) { c.interval = interval }
}

// Monitor returns a Monitor sampling memory from this client's system
// detector at a periodic interval (5s by default; override with
// WithMonitorInterval). Between samples, any Chat or ChatStream call made
// through this Client also pushes a live update carrying that request's
// TokensPerSecond and TimeToFirstToken, merged onto the most recent memory
// sample — so subscribers see inference performance as it happens rather
// than waiting for the next tick.
func (c *Client) Monitor(opts ...MonitorOption) *monitor.Monitor {
	cfg := MonitorConfig{interval: defaultMonitorInterval}
	for _, opt := range opts {
		opt(&cfg)
	}

	interval := cfg.interval
	if interval <= 0 {
		interval = defaultMonitorInterval
	}

	var mu sync.Mutex
	var lastMemUsed, lastMemAvailable uint64

	sampler := monitor.SamplerFunc(func(ctx context.Context) (monitor.Metrics, error) {
		info, err := c.System.Info(ctx)
		if err != nil {
			return monitor.Metrics{}, err
		}
		mu.Lock()
		lastMemUsed = info.TotalMemory - info.AvailableMemory
		lastMemAvailable = info.AvailableMemory
		mu.Unlock()
		return monitor.Metrics{
			MemoryUsedBytes:      lastMemUsed,
			MemoryAvailableBytes: lastMemAvailable,
		}, nil
	})

	m := monitor.New(sampler, interval)

	c.setMetricsSink(func(inference monitor.Metrics) {
		mu.Lock()
		inference.MemoryUsedBytes = lastMemUsed
		inference.MemoryAvailableBytes = lastMemAvailable
		mu.Unlock()
		m.Report(inference)
	})

	return m
}

// FormatBytes renders a byte count as a human-readable string, e.g.
// "24.0 GB".
func FormatBytes(b uint64) string {
	return system.FormatBytes(b)
}
