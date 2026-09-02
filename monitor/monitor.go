// Package monitor provides thread-safe, subscribable monitoring of memory
// and inference metrics.
package monitor

import (
	"context"
	"sync"
	"time"
)

// Subscriber receives Metrics updates as they are sampled.
type Subscriber func(Metrics)

// Monitor periodically samples Metrics and notifies subscribers. All
// methods are safe for concurrent use.
type Monitor struct {
	sampler  Sampler
	interval time.Duration

	mu          sync.RWMutex
	latest      Metrics
	subscribers []Subscriber

	cancel context.CancelFunc
	done   chan struct{}
}

// New returns a Monitor that samples the given Sampler at the given
// interval once Start is called.
func New(sampler Sampler, interval time.Duration) *Monitor {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &Monitor{
		sampler:  sampler,
		interval: interval,
	}
}

// Start begins periodic sampling in a background goroutine. It is a no-op
// if the monitor is already running. Sampling stops when ctx is canceled
// or Stop is called.
func (m *Monitor) Start(ctx context.Context) {
	m.mu.Lock()
	if m.cancel != nil {
		m.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.done = make(chan struct{})
	m.mu.Unlock()

	go m.run(runCtx)
}

func (m *Monitor) run(ctx context.Context) {
	defer close(m.done)

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	m.sampleAndNotify(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.sampleAndNotify(ctx)
		}
	}
}

func (m *Monitor) sampleAndNotify(ctx context.Context) {
	metrics, err := m.sampler.Sample(ctx)
	if err != nil {
		return
	}

	m.mu.Lock()
	m.latest = metrics
	subs := make([]Subscriber, len(m.subscribers))
	copy(subs, m.subscribers)
	m.mu.Unlock()

	for _, sub := range subs {
		sub(metrics)
	}
}

// Stop halts periodic sampling and waits for the background goroutine to
// exit. Safe to call multiple times.
func (m *Monitor) Stop() {
	m.mu.Lock()
	cancel := m.cancel
	done := m.done
	m.cancel = nil
	m.mu.Unlock()

	if cancel == nil {
		return
	}
	cancel()
	if done != nil {
		<-done
	}
}

// Metrics returns the most recently sampled Metrics snapshot.
func (m *Monitor) Metrics() Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.latest
}

// Subscribe registers a Subscriber to be called with each new Metrics
// sample.
func (m *Monitor) Subscribe(sub Subscriber) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscribers = append(m.subscribers, sub)
}
