package veloxquant

import (
	"context"
	"fmt"

	"github.com/rajveer43/veloxquant-go/models"
	"github.com/rajveer43/veloxquant-go/optimize"
)

// AutoPilotConfig describes the intent behind an AutoPilot session: what
// task the caller wants to accomplish, and optionally which model to use.
type AutoPilotConfig struct {
	// Task is used to select a suitable model, e.g. "coding", "chat",
	// "reasoning", "vision", "agent", "translation".
	Task string

	// Model may be a specific model name, or "auto" (or empty) to let
	// AutoPilot choose one based on Task and available hardware.
	Model string

	// ContextLength, if set, overrides AutoPilot's automatic context
	// length selection.
	ContextLength int
}

// AutoPilotPlan documents every decision AutoPilot made when constructing
// a Session, so the process is transparent and debuggable.
type AutoPilotPlan struct {
	Hardware SystemInfo

	SelectedModel string

	ContextLength int

	CompressionBits int

	EstimatedMemoryBytes uint64
	SafetyMarginBytes    uint64

	Profile optimize.Profile

	Reason string
}

// Session is a ready-to-use AI session produced by AutoPilot, bound to a
// specific model and optimization plan.
type Session struct {
	client *Client
	plan   AutoPilotPlan
}

// Plan returns the decisions AutoPilot made to construct this Session.
func (s *Session) Plan() AutoPilotPlan {
	return s.plan
}

// Chat sends a message using the model AutoPilot selected for this
// session.
func (s *Session) Chat(ctx context.Context, prompt string) (ChatResponse, error) {
	return s.client.Chat(ctx, ChatRequest{
		Model: s.plan.SelectedModel,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	})
}

// defaultContextLength is used when the caller doesn't specify one and no
// safer estimate can be derived.
const defaultContextLength = 8192

// safetyMarginRatio is the fraction of available memory AutoPilot reserves
// as headroom beyond the estimated model + KV-cache footprint.
const safetyMarginRatio = 0.15

// AutoPilot inspects the host system, selects a compatible model and
// context length for the given task, chooses a VeloxQuant compression
// strategy, and returns a ready-to-use Session.
func (c *Client) AutoPilot(ctx context.Context, cfg AutoPilotConfig) (*Session, error) {
	hardware, err := c.System.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("autopilot: %w", err)
	}

	contextLength := cfg.ContextLength
	if contextLength <= 0 {
		contextLength = defaultContextLength
	}

	modelInfo, err := c.selectModel(ctx, cfg, hardware)
	if err != nil {
		return nil, fmt.Errorf("autopilot: %w", err)
	}

	rec, err := c.Optimize.Recommend(ctx, OptimizationRequest{
		Model:         modelInfo.Name,
		Architecture:  modelInfo.Architecture,
		ContextLength: contextLength,
	})
	if err != nil {
		return nil, fmt.Errorf("autopilot: %w", err)
	}

	safetyMargin := uint64(float64(hardware.AvailableMemory) * safetyMarginRatio)

	plan := AutoPilotPlan{
		Hardware:             hardware,
		SelectedModel:        modelInfo.Name,
		ContextLength:        contextLength,
		CompressionBits:      rec.CompressionBits,
		EstimatedMemoryBytes: rec.EstimatedMemoryAfter,
		SafetyMarginBytes:    safetyMargin,
		Profile:              rec.Profile,
		Reason:               rec.Reason,
	}

	return &Session{client: c, plan: plan}, nil
}

func (c *Client) selectModel(ctx context.Context, cfg AutoPilotConfig, hardware SystemInfo) (models.Info, error) {
	if cfg.Model != "" && cfg.Model != "auto" {
		if info, ok := c.Models.registry.Get(cfg.Model); ok {
			return info, nil
		}
		return models.Info{}, fmt.Errorf("%w: %s", ErrModelNotFound, cfg.Model)
	}

	candidates, err := c.Models.Recommend(ctx, ModelRecommendationRequest{
		Task:                 cfg.Task,
		AvailableMemoryBytes: hardware.AvailableMemory,
		ContextLength:        cfg.ContextLength,
	})
	if err != nil {
		return models.Info{}, err
	}

	if len(candidates) == 0 {
		all := c.Models.List()
		if len(all) == 0 {
			return models.Info{}, fmt.Errorf("%w: no models in registry", ErrModelNotFound)
		}
		return models.Info{}, fmt.Errorf("%w: no model fits available memory (%s) for task %q", ErrInsufficientMemory, FormatBytes(hardware.AvailableMemory), cfg.Task)
	}

	return candidates[0], nil
}
