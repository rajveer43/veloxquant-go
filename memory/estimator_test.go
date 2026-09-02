package memory

import "testing"

func testArchitecture() Architecture {
	return Architecture{
		NumLayers:      32,
		NumKVHeads:     8,
		HeadDim:        128,
		HiddenSize:     4096,
		ParameterCount: 7_000_000_000,
	}
}

func TestEstimatorEstimate(t *testing.T) {
	est := NewEstimator()

	result, err := est.Estimate(Request{
		ModelName:     "test-model",
		Architecture:  testArchitecture(),
		ContextLength: 4096,
		Precision:     FP16,
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	wantModelBytes := uint64(7_000_000_000 * 2)
	if result.ModelMemoryBytes != wantModelBytes {
		t.Errorf("ModelMemoryBytes = %d, want %d", result.ModelMemoryBytes, wantModelBytes)
	}

	if result.KVCacheMemoryBytes == 0 {
		t.Error("KVCacheMemoryBytes should be non-zero")
	}

	if result.TotalMemoryBytes != result.ModelMemoryBytes+result.KVCacheMemoryBytes+runtimeOverheadBytes {
		t.Errorf("TotalMemoryBytes does not equal sum of components")
	}

	if result.OptimizedTotalBytes >= result.TotalMemoryBytes {
		t.Errorf("OptimizedTotalBytes (%d) should be less than TotalMemoryBytes (%d)", result.OptimizedTotalBytes, result.TotalMemoryBytes)
	}

	if result.SavedBytes == 0 {
		t.Error("SavedBytes should be non-zero when optimization reduces memory")
	}

	if result.SavedPercent <= 0 || result.SavedPercent >= 100 {
		t.Errorf("SavedPercent = %f, want between 0 and 100", result.SavedPercent)
	}
}

func TestEstimatorEstimateInvalidContextLength(t *testing.T) {
	est := NewEstimator()

	_, err := est.Estimate(Request{
		ModelName:     "test-model",
		Architecture:  testArchitecture(),
		ContextLength: 0,
	})
	if err == nil {
		t.Fatal("expected error for zero context length, got nil")
	}
}

func TestEstimatorEstimateFallsBackToArchitectureDerivedParams(t *testing.T) {
	est := NewEstimator()

	arch := Architecture{
		NumLayers:  32,
		NumKVHeads: 8,
		HeadDim:    128,
		HiddenSize: 4096,
		// ParameterCount intentionally omitted
	}

	result, err := est.Estimate(Request{
		ModelName:     "no-param-count",
		Architecture:  arch,
		ContextLength: 2048,
		Precision:     FP16,
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	if result.ModelMemoryBytes == 0 {
		t.Error("ModelMemoryBytes should be estimated from architecture when ParameterCount is unset")
	}
}

func TestEstimatorEstimateDefaultsOptimizedPrecisionToInt4(t *testing.T) {
	est := NewEstimator()

	withDefault, err := est.Estimate(Request{
		ModelName:     "m",
		Architecture:  testArchitecture(),
		ContextLength: 4096,
		Precision:     FP16,
	})
	if err != nil {
		t.Fatal(err)
	}

	explicit, err := est.Estimate(Request{
		ModelName:          "m",
		Architecture:       testArchitecture(),
		ContextLength:      4096,
		Precision:          FP16,
		OptimizedPrecision: Int4,
	})
	if err != nil {
		t.Fatal(err)
	}

	if withDefault.OptimizedTotalBytes != explicit.OptimizedTotalBytes {
		t.Errorf("expected default optimized precision to be Int4: got %d, want %d", withDefault.OptimizedTotalBytes, explicit.OptimizedTotalBytes)
	}
}

func TestPrecisionBytesPerElement(t *testing.T) {
	tests := []struct {
		p    Precision
		want float64
	}{
		{FP16, 2},
		{FP8, 1},
		{Int8, 1},
		{Int4, 0.5},
		{"unknown", 2},
	}
	for _, tt := range tests {
		if got := tt.p.BytesPerElement(); got != tt.want {
			t.Errorf("%s.BytesPerElement() = %f, want %f", tt.p, got, tt.want)
		}
	}
}

func TestPrecisionValid(t *testing.T) {
	valid := []Precision{FP16, FP8, Int8, Int4}
	for _, p := range valid {
		if !p.Valid() {
			t.Errorf("%s.Valid() = false, want true", p)
		}
	}
	if Precision("bogus").Valid() {
		t.Error("bogus precision should not be valid")
	}
}
