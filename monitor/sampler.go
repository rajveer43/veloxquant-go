package monitor

import "context"

// Sampler produces a Metrics snapshot on demand. The default
// implementation samples system memory via the system package; tests can
// substitute a fake Sampler.
type Sampler interface {
	Sample(ctx context.Context) (Metrics, error)
}

// SamplerFunc adapts a plain function to the Sampler interface.
type SamplerFunc func(ctx context.Context) (Metrics, error)

func (f SamplerFunc) Sample(ctx context.Context) (Metrics, error) {
	return f(ctx)
}
