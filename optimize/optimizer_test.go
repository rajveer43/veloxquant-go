package optimize

import (
	"context"
	"errors"
	"testing"

	"github.com/rajveer43/veloxquant-go/memory"
)

func testArchitecture() memory.Architecture {
	return memory.Architecture{
		NumLayers:      32,
		NumKVHeads:     8,
		HeadDim:        128,
		HiddenSize:     4096,
		ParameterCount: 7_000_000_000,
	}
}

func TestOptimizerRecommend(t *testing.T) {
	opt := NewOptimizer(memory.NewEstimator())

	rec, err := opt.Recommend(context.Background(), Request{
		ModelName:            "test-model",
		Architecture:         testArchitecture(),
		ContextLength:        4096,
		AvailableMemoryBytes: 8_000_000_000,
	})
	if err != nil {
		t.Fatalf("Recommend() error = %v", err)
	}

	if !rec.Profile.Valid() {
		t.Errorf("Profile %q is not valid", rec.Profile)
	}
	if rec.CompressionBits == 0 {
		t.Error("expected non-zero CompressionBits")
	}
	if rec.ContextLength != 4096 {
		t.Errorf("ContextLength = %d, want 4096", rec.ContextLength)
	}
	if rec.Reason == "" {
		t.Error("expected non-empty Reason")
	}
}

func TestOptimizerRecommendRespectsExplicitProfile(t *testing.T) {
	opt := NewOptimizer(memory.NewEstimator())

	rec, err := opt.Recommend(context.Background(), Request{
		ModelName:     "test-model",
		Architecture:  testArchitecture(),
		ContextLength: 4096,
		Profile:       ProfileSpeed,
	})
	if err != nil {
		t.Fatalf("Recommend() error = %v", err)
	}

	if rec.Profile != ProfileSpeed {
		t.Errorf("Profile = %s, want %s", rec.Profile, ProfileSpeed)
	}
	if rec.CompressionBits != 16 {
		t.Errorf("CompressionBits = %d, want 16 for speed profile", rec.CompressionBits)
	}
}

func TestOptimizerRecommendInvalidContextLength(t *testing.T) {
	opt := NewOptimizer(memory.NewEstimator())

	_, err := opt.Recommend(context.Background(), Request{
		ModelName:     "test-model",
		Architecture:  testArchitecture(),
		ContextLength: 0,
	})
	if err == nil {
		t.Fatal("expected error for zero context length")
	}
}

func TestOptimizerRecommendRespectsCanceledContext(t *testing.T) {
	opt := NewOptimizer(memory.NewEstimator())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := opt.Recommend(ctx, Request{
		ModelName:     "test-model",
		Architecture:  testArchitecture(),
		ContextLength: 4096,
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestProfileValid(t *testing.T) {
	valid := []Profile{ProfileSpeed, ProfileBalanced, ProfileMemory, ProfileMaximumContext}
	for _, p := range valid {
		if !p.Valid() {
			t.Errorf("%s.Valid() = false, want true", p)
		}
	}
	if Profile("bogus").Valid() {
		t.Error("bogus profile should not be valid")
	}
}
