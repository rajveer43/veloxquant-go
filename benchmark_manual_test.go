//go:build manual

// Package veloxquant's Benchmark cannot be exercised in CI: it requires
// real Apple Silicon hardware, a working `veloxquant` CLI installation
// (see runtime.DefaultCommand), and a model already downloaded to the
// local Hugging Face cache (see models.Pull or `vq models pull`) — none of
// which this repository's test environment provides or fakes, unlike
// runtime/process_test.go's re-exec'd fake subprocess (that seam works
// because it doesn't need the fake process to actually serve a chat
// completions API or occupy real memory; Benchmark's whole point is
// measuring both of those against a real runtime).
//
// This file is therefore excluded from `go test ./...` by the "manual"
// build tag and is not part of this repository's CI coverage. Run it
// by hand, with a real model already pulled:
//
//	go test -tags manual -run TestBenchmarkManual -v -timeout 20m ./... \
//	  -args -model mlx-community/Qwen3-4B-4bit
//
// There is no other manual/integration-test convention already
// established in this repository to match (runtime/'s existing tests are
// all either build-tagged per-OS, like process_unix.go/process_windows.go,
// or driven by the re-exec'd fake-subprocess seam in process_test.go); the
// "manual" tag name and invocation shape here follow this SDK family's TS
// sibling's test/integration/benchmark.manual.ts precedent instead.
package veloxquant

import (
	"context"
	"flag"
	"testing"
	"time"
)

var manualBenchmarkModel = flag.String("model", "mlx-community/Qwen3-4B-4bit", "model id to benchmark (must already be pulled locally)")

// TestBenchmarkManual exercises Benchmark end to end against a real,
// locally running VeloxQuant runtime on real Apple Silicon hardware. It is
// not run by `go test ./...` (see the "manual" build tag above) and is not
// claimed as part of this repository's automated coverage.
func TestBenchmarkManual(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	result, err := Benchmark(ctx, client, BenchmarkInput{Model: *manualBenchmarkModel})
	if err != nil {
		t.Fatalf("Benchmark() error = %v", err)
	}

	t.Logf("\n%s", result.ToMarkdown())

	if result.TokensPerSecond <= 0 {
		t.Errorf("TokensPerSecond = %f, want > 0", result.TokensPerSecond)
	}
	if result.TimeToFirstTokenMs <= 0 {
		t.Errorf("TimeToFirstTokenMs = %f, want > 0", result.TimeToFirstTokenMs)
	}
}
