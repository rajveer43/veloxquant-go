package models

import (
	"context"
	"testing"

	"github.com/rajveer43/veloxquant-go/memory"
)

func TestRecommendFiltersByTask(t *testing.T) {
	registry := NewRegistry()
	estimator := memory.NewEstimator()

	results, err := Recommend(context.Background(), registry, estimator, RecommendationRequest{
		Task: TaskCoding,
	})
	if err != nil {
		t.Fatalf("Recommend() error = %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one coding model")
	}
	for _, m := range results {
		if !hasTask(m, TaskCoding) {
			t.Errorf("model %s does not support coding task", m.Name)
		}
	}
}

func TestRecommendFiltersByMemory(t *testing.T) {
	registry := NewRegistry()
	estimator := memory.NewEstimator()

	// A tiny memory budget should exclude every model in the registry.
	results, err := Recommend(context.Background(), registry, estimator, RecommendationRequest{
		AvailableMemoryBytes: 1024 * 1024, // 1 MB
	})
	if err != nil {
		t.Fatalf("Recommend() error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected no models to fit 1MB budget, got %d", len(results))
	}
}

func TestRecommendRespectsCanceledContext(t *testing.T) {
	registry := NewRegistry()
	estimator := memory.NewEstimator()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Recommend(ctx, registry, estimator, RecommendationRequest{})
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestRegistryGet(t *testing.T) {
	registry := NewRegistry()

	all := registry.List()
	if len(all) == 0 {
		t.Fatal("expected non-empty registry")
	}

	got, ok := registry.Get(all[0].Name)
	if !ok {
		t.Fatalf("Get(%s) not found", all[0].Name)
	}
	if got.Name != all[0].Name {
		t.Errorf("Get returned wrong model: %s", got.Name)
	}

	_, ok = registry.Get("does-not-exist")
	if ok {
		t.Error("expected Get to return false for unknown model")
	}
}
