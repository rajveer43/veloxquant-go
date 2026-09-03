package models

import (
	"context"
	"fmt"
	"sort"

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

// Scored pairs a candidate model with the score and reasoning Recommend
// used to rank it, so callers (e.g. AutoPilot) can surface *why* a model
// was chosen rather than just which one.
type Scored struct {
	Info Info

	// Score is Recommend's internal ranking value for this candidate,
	// higher is better. It has no meaning outside of comparing candidates
	// from the same Recommend call.
	Score float64

	// Reason is a human-readable explanation of the score.
	Reason string
}

// Recommend returns models from the registry suited to the given task,
// ranked by suitability (best first), filtered to those that plausibly fit
// within AvailableMemoryBytes (when specified). Ranking considers task fit,
// how much memory headroom the model leaves, and how well its KV cache
// scales to the requested context length — not just registry order.
func Recommend(ctx context.Context, registry Registry, estimator recommendationEstimator, req RecommendationRequest) ([]Info, error) {
	scored, err := RecommendScored(ctx, registry, estimator, req)
	if err != nil {
		return nil, err
	}
	out := make([]Info, len(scored))
	for i, s := range scored {
		out[i] = s.Info
	}
	return out, nil
}

// RecommendScored behaves like Recommend but also returns the score and
// reasoning behind each candidate's ranking, most suitable first.
func RecommendScored(ctx context.Context, registry Registry, estimator recommendationEstimator, req RecommendationRequest) ([]Scored, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	contextLength := req.ContextLength
	if contextLength <= 0 {
		contextLength = 8192
	}

	var candidates []Scored
	for _, m := range registry.List() {
		if !m.Supported {
			continue
		}
		if req.Task != "" && !hasTask(m, req.Task) {
			continue
		}

		score := baseScore(m)

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

			headroom := memoryHeadroomRatio(estimate.TotalMemoryBytes, req.AvailableMemoryBytes)
			score += headroom * headroomWeight

			candidates = append(candidates, Scored{
				Info:  m,
				Score: score,
				Reason: fmt.Sprintf(
					"fits task %q with %.0f%% memory headroom at %d-token context",
					req.Task, headroom*100, contextLength,
				),
			})
			continue
		}

		candidates = append(candidates, Scored{
			Info:   m,
			Score:  score,
			Reason: fmt.Sprintf("matches task %q; no memory budget given to rank by headroom", req.Task),
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	return candidates, nil
}

// headroomWeight controls how much memory headroom (relative to a
// perfectly tight fit) contributes to a candidate's score, relative to the
// fixed recommendedBonus below.
const headroomWeight = 1.0

// recommendedBonus is added to the score of models the registry marks as
// Recommended, so a curated pick outranks an equally-fitting alternative.
const recommendedBonus = 0.5

func baseScore(m Info) float64 {
	if m.Recommended {
		return recommendedBonus
	}
	return 0
}

// memoryHeadroomRatio returns how much spare memory remains after loading
// a model of the given footprint, as a fraction of available memory
// (0 = uses all of it, close to 1 = barely uses any). Smaller footprints
// relative to budget score higher, since they leave more room for other
// applications and reduce OOM risk from estimation error.
func memoryHeadroomRatio(totalBytes, availableBytes uint64) float64 {
	if availableBytes == 0 {
		return 0
	}
	return 1 - float64(totalBytes)/float64(availableBytes)
}

func hasTask(m Info, task Task) bool {
	for _, t := range m.Tasks {
		if t == task {
			return true
		}
	}
	return false
}
