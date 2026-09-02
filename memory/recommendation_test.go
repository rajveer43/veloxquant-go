package memory

import "testing"

func TestRecommendStrategy(t *testing.T) {
	est := Estimate{
		TotalMemoryBytes:    10_000_000_000,
		OptimizedTotalBytes: 5_000_000_000,
	}

	tests := []struct {
		name          string
		available     uint64
		wantPrecision Precision
	}{
		{"unknown memory defaults to int4", 0, Int4},
		{"ample memory needs no compression", 20_000_000_000, FP16},
		{"tight memory needs compression", 6_000_000_000, Int4},
		{"insufficient even compressed", 1_000_000_000, Int4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			precision, reason := RecommendStrategy(est, tt.available)
			if precision != tt.wantPrecision {
				t.Errorf("RecommendStrategy() precision = %s, want %s", precision, tt.wantPrecision)
			}
			if reason == "" {
				t.Error("expected non-empty reason")
			}
		})
	}
}
