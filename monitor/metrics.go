package monitor

import "time"

// Metrics is a snapshot of memory and inference performance at a point in
// time.
type Metrics struct {
	MemoryUsedBytes      uint64
	MemoryAvailableBytes uint64

	TokensPerSecond  float64
	TimeToFirstToken time.Duration

	ContextLength int
	KVCacheBytes  uint64

	CompressionRatio float64
}
