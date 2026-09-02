package memory

import "fmt"

// runtimeOverheadBytes is a conservative fixed estimate of the memory
// overhead of the inference runtime itself (buffers, framework, activation
// scratch space) beyond weights and KV cache.
const runtimeOverheadBytes uint64 = 512 * 1024 * 1024

// compressionBitsFor maps a precision to its bit width, used for reporting
// recommended compression strategies.
func compressionBitsFor(p Precision) int {
	switch p {
	case FP16:
		return 16
	case FP8:
		return 8
	case Int8:
		return 8
	case Int4:
		return 4
	default:
		return 16
	}
}

// Request describes a memory estimation query for a specific model,
// context length, and precision.
type Request struct {
	// ModelName is used only for error messages/context, not calculation.
	ModelName string

	Architecture  Architecture
	ContextLength int
	Precision     Precision

	// OptimizedPrecision is the precision VeloxQuant would use for the KV
	// cache when compression is applied. Defaults to Int4 if unset.
	OptimizedPrecision Precision
}

// Estimate is the result of a memory estimation, covering both the
// unoptimized ("naive") footprint and the VeloxQuant-optimized footprint.
type Estimate struct {
	ModelMemoryBytes     uint64
	KVCacheMemoryBytes   uint64
	RuntimeOverheadBytes uint64
	TotalMemoryBytes     uint64

	OptimizedKVBytes    uint64
	OptimizedTotalBytes uint64
	SavedBytes          uint64
	SavedPercent        float64

	RecommendedStrategy string
}

// Estimator computes memory estimates. It is an interface so callers can
// substitute custom architecture-aware logic or mocks in tests.
type Estimator interface {
	Estimate(req Request) (Estimate, error)
}

type estimator struct{}

// NewEstimator returns the default Estimator implementation.
func NewEstimator() Estimator {
	return estimator{}
}

func (estimator) Estimate(req Request) (Estimate, error) {
	if req.ContextLength <= 0 {
		return Estimate{}, fmt.Errorf("estimate memory for %s: context length must be positive", req.ModelName)
	}

	precision := req.Precision
	if !precision.Valid() {
		precision = FP16
	}

	optimizedPrecision := req.OptimizedPrecision
	if !optimizedPrecision.Valid() {
		optimizedPrecision = Int4
	}

	modelMemory := modelMemoryBytes(req.Architecture, precision)

	kvBytes := EstimateKVCacheBytes(KVCacheRequest{
		Architecture:  req.Architecture,
		ContextLength: req.ContextLength,
		Precision:     precision,
	})

	optimizedKV := EstimateKVCacheBytes(KVCacheRequest{
		Architecture:  req.Architecture,
		ContextLength: req.ContextLength,
		Precision:     optimizedPrecision,
	})

	total := modelMemory + kvBytes + runtimeOverheadBytes
	optimizedTotal := modelMemory + optimizedKV + runtimeOverheadBytes

	var saved uint64
	var savedPercent float64
	if total > optimizedTotal {
		saved = total - optimizedTotal
		if total > 0 {
			savedPercent = float64(saved) / float64(total) * 100
		}
	}

	return Estimate{
		ModelMemoryBytes:     modelMemory,
		KVCacheMemoryBytes:   kvBytes,
		RuntimeOverheadBytes: runtimeOverheadBytes,
		TotalMemoryBytes:     total,

		OptimizedKVBytes:    optimizedKV,
		OptimizedTotalBytes: optimizedTotal,
		SavedBytes:          saved,
		SavedPercent:        savedPercent,

		RecommendedStrategy: fmt.Sprintf("%d-bit KV cache compression", compressionBitsFor(optimizedPrecision)),
	}, nil
}

// modelMemoryBytes estimates model weight memory from parameter count and
// precision. If ParameterCount is unset (0), it falls back to a rough
// estimate derived from architecture dimensions.
func modelMemoryBytes(arch Architecture, precision Precision) uint64 {
	params := arch.ParameterCount
	if params <= 0 {
		params = estimateParameterCount(arch)
	}
	return uint64(float64(params) * precision.BytesPerElement())
}

// estimateParameterCount provides a rough transformer parameter count from
// architecture dimensions when an explicit count isn't known. This is a
// coarse approximation (roughly 12 * hidden_size^2 per layer, the standard
// order-of-magnitude for attention + MLP blocks) intended only as a
// fallback.
func estimateParameterCount(arch Architecture) int64 {
	if arch.NumLayers <= 0 || arch.HiddenSize <= 0 {
		return 0
	}
	perLayer := 12 * int64(arch.HiddenSize) * int64(arch.HiddenSize)
	return perLayer * int64(arch.NumLayers)
}
