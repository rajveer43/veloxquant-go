package monitor

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMonitorSamplesAndNotifies(t *testing.T) {
	var calls int32
	sampler := SamplerFunc(func(ctx context.Context) (Metrics, error) {
		atomic.AddInt32(&calls, 1)
		return Metrics{MemoryUsedBytes: 100}, nil
	})

	m := New(sampler, 10*time.Millisecond)

	var received int32
	var wg sync.WaitGroup
	wg.Add(1)
	m.Subscribe(func(metrics Metrics) {
		if atomic.AddInt32(&received, 1) == 1 {
			wg.Done()
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	m.Start(ctx)
	defer cancel()

	wg.Wait()
	m.Stop()

	if atomic.LoadInt32(&calls) == 0 {
		t.Error("expected at least one sample")
	}
	if m.Metrics().MemoryUsedBytes != 100 {
		t.Errorf("Metrics().MemoryUsedBytes = %d, want 100", m.Metrics().MemoryUsedBytes)
	}
}

func TestMonitorStopIsIdempotent(t *testing.T) {
	sampler := SamplerFunc(func(ctx context.Context) (Metrics, error) {
		return Metrics{}, nil
	})
	m := New(sampler, 10*time.Millisecond)

	m.Start(context.Background())
	m.Stop()
	m.Stop() // should not panic or block
}

func TestMonitorStartIsIdempotent(t *testing.T) {
	sampler := SamplerFunc(func(ctx context.Context) (Metrics, error) {
		return Metrics{}, nil
	})
	m := New(sampler, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m.Start(ctx)
	m.Start(ctx) // should be a no-op, not start a second goroutine
	m.Stop()
}

func TestMonitorConcurrentSubscribeAndMetrics(t *testing.T) {
	sampler := SamplerFunc(func(ctx context.Context) (Metrics, error) {
		return Metrics{MemoryUsedBytes: 42}, nil
	})
	m := New(sampler, time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	m.Start(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Subscribe(func(Metrics) {})
			_ = m.Metrics()
		}()
	}
	wg.Wait()
	m.Stop()
}
