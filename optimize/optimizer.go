package optimize

import (
	"context"
	"fmt"

	"github.com/rajveer43/veloxquant-go/memory"
)

// Request describes an optimization recommendation query.
type Request struct {
	ModelName            string
	Architecture         memory.Architecture
	ContextLength        int
	AvailableMemoryBytes uint64

	// Profile, if set, forces a specific optimization profile instead of
	// letting Recommend choose one based on available memory.
	Profile Profile
}

// Recommendation is VeloxQuant's suggested optimization strategy for a
// model/context combination.
type Recommendation struct {
	Profile Profile

	CompressionMethod string
	CompressionBits   int

	EstimatedMemoryBefore uint64
	EstimatedMemoryAfter  uint64

	ContextLength int

	Reason string
}

// Optimizer produces optimization recommendations. It is an interface to
// allow mocking in tests and callers that supply custom estimators.
type Optimizer interface {
	Recommend(ctx context.Context, req Request) (Recommendation, error)
}

type optimizer struct {
	estimator memory.Estimator
}

// NewOptimizer returns the default Optimizer, backed by the given memory
// Estimator (pass memory.NewEstimator() for standard behavior).
func NewOptimizer(estimator memory.Estimator) Optimizer {
	return optimizer{estimator: estimator}
}

func (o optimizer) Recommend(ctx context.Context, req Request) (Recommendation, error) {
	if err := ctx.Err(); err != nil {
		return Recommendation{}, err
	}

	if req.ContextLength <= 0 {
		return Recommendation{}, fmt.Errorf("optimize recommend for %s: context length must be positive", req.ModelName)
	}

	estimate, err := o.estimator.Estimate(memory.Request{
		ModelName:          req.ModelName,
		Architecture:       req.Architecture,
		ContextLength:      req.ContextLength,
		Precision:          memory.FP16,
		OptimizedPrecision: memory.Int4,
	})
	if err != nil {
		return Recommendation{}, fmt.Errorf("optimize recommend for %s: %w", req.ModelName, err)
	}

	profile := req.Profile
	var reason string
	var precision memory.Precision

	if profile.Valid() {
		precision = precisionForProfile(profile)
		reason = fmt.Sprintf("using explicitly requested %q profile", profile)
	} else {
		precision, reason = memory.RecommendStrategy(estimate, req.AvailableMemoryBytes)
		profile = profileForPrecision(precision, estimate, req.AvailableMemoryBytes)
	}

	optimizedEstimate, err := o.estimator.Estimate(memory.Request{
		ModelName:          req.ModelName,
		Architecture:       req.Architecture,
		ContextLength:      req.ContextLength,
		Precision:          memory.FP16,
		OptimizedPrecision: precision,
	})
	if err != nil {
		return Recommendation{}, fmt.Errorf("optimize recommend for %s: %w", req.ModelName, err)
	}

	return Recommendation{
		Profile:               profile,
		CompressionMethod:     "VeloxQuant KV-cache compression",
		CompressionBits:       bitsForPrecision(precision),
		EstimatedMemoryBefore: optimizedEstimate.TotalMemoryBytes,
		EstimatedMemoryAfter:  optimizedEstimate.OptimizedTotalBytes,
		ContextLength:         req.ContextLength,
		Reason:                reason,
	}, nil
}

func precisionForProfile(p Profile) memory.Precision {
	switch p {
	case ProfileSpeed:
		return memory.FP16
	case ProfileBalanced:
		return memory.Int8
	case ProfileMemory, ProfileMaximumContext:
		return memory.Int4
	default:
		return memory.Int8
	}
}

func profileForPrecision(precision memory.Precision, estimate memory.Estimate, available uint64) Profile {
	switch precision {
	case memory.FP16:
		return ProfileSpeed
	case memory.Int8:
		return ProfileBalanced
	default:
		if available > 0 && estimate.OptimizedTotalBytes > 0 && available < estimate.OptimizedTotalBytes*2 {
			return ProfileMemory
		}
		return ProfileMaximumContext
	}
}

func bitsForPrecision(p memory.Precision) int {
	switch p {
	case memory.FP16:
		return 16
	case memory.FP8, memory.Int8:
		return 8
	case memory.Int4:
		return 4
	default:
		return 16
	}
}
