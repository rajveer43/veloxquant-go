package memory

// KVCacheRequest specifies the parameters needed to estimate KV-cache
// memory usage for a single inference sequence.
type KVCacheRequest struct {
	Architecture  Architecture
	ContextLength int
	Precision     Precision

	// CompressionRatio, when > 0, is the fraction of the uncompressed
	// KV-cache size retained after VeloxQuant compression (e.g. 0.5 means
	// the cache uses half as much memory). A zero value means "no
	// compression applied" (ratio of 1.0).
	CompressionRatio float64
}

// EstimateKVCacheBytes computes uncompressed KV-cache memory in bytes using:
//
//	KV Cache Memory = Layers × Tokens × KV Heads × Head Dim × 2 × BytesPerElement
//
// The factor of 2 accounts for storing both keys and values.
func EstimateKVCacheBytes(req KVCacheRequest) uint64 {
	arch := req.Architecture
	if arch.NumLayers <= 0 || arch.NumKVHeads <= 0 || arch.HeadDim <= 0 || req.ContextLength <= 0 {
		return 0
	}

	precision := req.Precision
	if !precision.Valid() {
		precision = FP16
	}

	elements := float64(arch.NumLayers) *
		float64(req.ContextLength) *
		float64(arch.NumKVHeads) *
		float64(arch.HeadDim) *
		2

	return uint64(elements * precision.BytesPerElement())
}

// ApplyCompression returns the KV-cache size after applying a VeloxQuant
// compression ratio. A ratio <= 0 or > 1 is treated as "no compression".
func ApplyCompression(uncompressedBytes uint64, ratio float64) uint64 {
	if ratio <= 0 || ratio > 1 {
		return uncompressedBytes
	}
	return uint64(float64(uncompressedBytes) * ratio)
}
