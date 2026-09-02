package models

import (
	"context"
	"fmt"

	"github.com/rajveer43/veloxquant-go/memory"
)

// RecommendationRequest describes a model recommendation query.
type RecommendationRequest struct {
	Task                 Task
	AvailableMemoryBytes uint64
	ContextLength        int
}

// recommendationEstimator is the subset of memory.Estimator used here,
// declared locally to keep this package's dependency surface minimal and
// mockable in tests.
type recommendationEstimator interface {
	Estimate(req memory.Request) (memory.Estimate, error)
}

// Recommend returns models from the registry suited to the given task,
// ordered by suitability, filtered to those that plausibly fit within
// AvailableMemoryBytes (when specified).
func Recommend(ctx context.Context, registry Registry, estimator recommendationEstimator, req RecommendationRequest) ([]Info, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	contextLength := req.ContextLength
	if contextLength <= 0 {
		contextLength = 8192
	}

	var candidates []Info
	for _, m := range registry.List() {
		if !m.Supported {
			continue
		}
		if req.Task != "" && !hasTask(m, req.Task) {
			continue
		}

		if req.AvailableMemoryBytes > 0 {
			estimate, err := estimator.Estimate(memory.Request{
				ModelName:          m.Name,
				Architecture:       m.Architecture,
				ContextLength:      contextLength,
				Precision:          memory.Int4,
				OptimizedPrecision: memory.Int4,
			})
			if err != nil {
				return nil, fmt.Errorf("recommend models: estimate %s: %w", m.Name, err)
			}
			if estimate.TotalMemoryBytes > req.AvailableMemoryBytes {
				continue
			}
		}

		candidates = append(candidates, m)
	}

	return candidates, nil
}

func hasTask(m Info, task Task) bool {
	for _, t := range m.Tasks {
		if t == task {
			return true
		}
	}
	return false
}
