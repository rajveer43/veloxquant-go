package memory

import "testing"

func TestEstimateKVCacheBytes(t *testing.T) {
	arch := Architecture{
		NumLayers:  32,
		NumKVHeads: 8,
		HeadDim:    128,
		HiddenSize: 4096,
	}

	tests := []struct {
		name      string
		req       KVCacheRequest
		wantBytes uint64
	}{
		{
			name: "fp16",
			req: KVCacheRequest{
				Architecture:  arch,
				ContextLength: 4096,
				Precision:     FP16,
			},
			// 32 * 4096 * 8 * 128 * 2 * 2 bytes
			wantBytes: uint64(32) * 4096 * 8 * 128 * 2 * 2,
		},
		{
			name: "int4 quarter size of fp16",
			req: KVCacheRequest{
				Architecture:  arch,
				ContextLength: 4096,
				Precision:     Int4,
			},
			wantBytes: uint64(float64(32*4096*8*128*2) * 0.5),
		},
		{
			name: "zero context length",
			req: KVCacheRequest{
				Architecture:  arch,
				ContextLength: 0,
				Precision:     FP16,
			},
			wantBytes: 0,
		},
		{
			name: "invalid architecture",
			req: KVCacheRequest{
				Architecture:  Architecture{},
				ContextLength: 4096,
				Precision:     FP16,
			},
			wantBytes: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateKVCacheBytes(tt.req)
			if got != tt.wantBytes {
				t.Errorf("EstimateKVCacheBytes() = %d, want %d", got, tt.wantBytes)
			}
		})
	}
}

func TestEstimateKVCacheBytesDefaultsToFP16(t *testing.T) {
	arch := Architecture{NumLayers: 1, NumKVHeads: 1, HeadDim: 1}
	got := EstimateKVCacheBytes(KVCacheRequest{
		Architecture:  arch,
		ContextLength: 1,
		Precision:     "invalid",
	})
	want := EstimateKVCacheBytes(KVCacheRequest{
		Architecture:  arch,
		ContextLength: 1,
		Precision:     FP16,
	})
	if got != want {
		t.Errorf("invalid precision should default to FP16: got %d, want %d", got, want)
	}
}

func TestApplyCompression(t *testing.T) {
	tests := []struct {
		name  string
		bytes uint64
		ratio float64
		want  uint64
	}{
		{"half", 1000, 0.5, 500},
		{"no compression at zero ratio", 1000, 0, 1000},
		{"no compression above one", 1000, 1.5, 1000},
		{"full retention at one", 1000, 1.0, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyCompression(tt.bytes, tt.ratio)
			if got != tt.want {
				t.Errorf("ApplyCompression(%d, %f) = %d, want %d", tt.bytes, tt.ratio, got, tt.want)
			}
		})
	}
}
