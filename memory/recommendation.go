package memory

import "fmt"

// RecommendStrategy chooses a KV-cache precision given how much memory is
// available versus how much the naive (uncompressed) estimate would need.
// It returns the recommended precision and a human-readable reason.
func RecommendStrategy(estimate Estimate, availableMemoryBytes uint64) (Precision, string) {
	switch {
	case availableMemoryBytes == 0:
		return Int4, "available memory unknown; defaulting to the most memory-efficient option"
	case estimate.TotalMemoryBytes <= availableMemoryBytes:
		return FP16, "sufficient memory available; no compression required"
	case estimate.OptimizedTotalBytes <= availableMemoryBytes:
		return Int4, fmt.Sprintf(
			"uncompressed footprint exceeds available memory; 4-bit KV compression fits within budget",
		)
	default:
		return Int4, "even with maximum compression, memory is tight; consider a smaller model or shorter context"
	}
}
