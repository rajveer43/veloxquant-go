// Package system provides hardware and platform detection for VeloxQuant,
// including Apple Silicon detection and system memory inspection.
package system

import (
	"context"
	"runtime"
)

// Info describes the host system relevant to running local LLM inference.
type Info struct {
	Platform     string
	Architecture string
	CPUModel     string
	AppleSilicon bool

	TotalMemory     uint64
	AvailableMemory uint64

	RecommendedProfile string
}

// Detector inspects the host system. It is an interface so hardware
// detection can be mocked in tests.
type Detector interface {
	Info(ctx context.Context) (Info, error)
}

// detector is the default Detector implementation, backed by
// platform-specific probes (see darwin.go, linux.go, windows.go).
type detector struct{}

// NewDetector returns the default system Detector for the current platform.
func NewDetector() Detector {
	return detector{}
}

func (detector) Info(ctx context.Context) (Info, error) {
	info := Info{
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
	}

	if err := ctx.Err(); err != nil {
		return Info{}, err
	}

	probe(&info)

	total, available, err := memoryStats()
	if err == nil {
		info.TotalMemory = total
		info.AvailableMemory = available
	}

	info.RecommendedProfile = recommendProfile(info)

	return info, nil
}

// recommendProfile picks a sane default VeloxQuant optimization profile
// name based on detected hardware. Kept as a plain string here (rather than
// importing the optimize package) to avoid a system -> optimize dependency;
// the optimize package defines the canonical OptimizationProfile type.
func recommendProfile(info Info) string {
	const gib = 1024 * 1024 * 1024

	switch {
	case info.TotalMemory == 0:
		return "balanced"
	case info.TotalMemory < 8*gib:
		return "memory"
	case info.TotalMemory < 16*gib:
		return "balanced"
	case info.TotalMemory < 32*gib:
		return "balanced"
	default:
		return "maximum-context"
	}
}
