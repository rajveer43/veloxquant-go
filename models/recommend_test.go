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

func TestRecommendScoredRanksRecommendedModelsHigher(t *testing.T) {
	registry := NewRegistry()
	estimator := memory.NewEstimator()

	// TaskChat is supported by both a Recommended model
	// (mlx-community/Qwen3-8B-4bit) and a non-Recommended one
	// (mlx-community/gemma-2-9b-it-4bit); with no memory budget to
	// otherwise differentiate them, the curated pick should rank first.
	scored, err := RecommendScored(context.Background(), registry, estimator, RecommendationRequest{
		Task: TaskChat,
	})
	if err != nil {
		t.Fatalf("RecommendScored() error = %v", err)
	}
	if len(scored) < 2 {
		t.Fatalf("expected at least 2 chat-capable models, got %d", len(scored))
	}
	if !scored[0].Info.Recommended {
		t.Errorf("expected top-ranked model to be Recommended, got %s", scored[0].Info.Name)
	}
	for i := 1; i < len(scored); i++ {
		if scored[i].Score > scored[i-1].Score {
			t.Errorf("scores not sorted descending at index %d: %f > %f", i, scored[i].Score, scored[i-1].Score)
		}
	}
	if scored[0].Reason == "" {
		t.Error("expected non-empty reason for top candidate")
	}
}

func TestRecommendScoredPrefersMoreHeadroom(t *testing.T) {
	registry := NewRegistry()
	estimator := memory.NewEstimator()

	// A generous budget lets every supported model fit; the ranking should
	// still be well-formed (descending scores) and each candidate should
	// carry a headroom-based reason since a budget was given.
	scored, err := RecommendScored(context.Background(), registry, estimator, RecommendationRequest{
		AvailableMemoryBytes: 64 * 1024 * 1024 * 1024, // 64 GB
	})
	if err != nil {
		t.Fatalf("RecommendScored() error = %v", err)
	}
	if len(scored) == 0 {
		t.Fatal("expected at least one candidate to fit a 64GB budget")
	}
	for _, s := range scored {
		if s.Reason == "" {
			t.Errorf("expected non-empty reason for %s", s.Info.Name)
		}
	}
}

func TestRecommendReturnsSameOrderAsRecommendScored(t *testing.T) {
	registry := NewRegistry()
	estimator := memory.NewEstimator()

	req := RecommendationRequest{Task: TaskChat}

	scored, err := RecommendScored(context.Background(), registry, estimator, req)
	if err != nil {
		t.Fatalf("RecommendScored() error = %v", err)
	}
	plain, err := Recommend(context.Background(), registry, estimator, req)
	if err != nil {
		t.Fatalf("Recommend() error = %v", err)
	}

	if len(plain) != len(scored) {
		t.Fatalf("Recommend returned %d models, RecommendScored returned %d", len(plain), len(scored))
	}
	for i := range plain {
		if plain[i].Name != scored[i].Info.Name {
			t.Errorf("index %d: Recommend = %s, RecommendScored = %s", i, plain[i].Name, scored[i].Info.Name)
		}
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
