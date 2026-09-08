package veloxquant

import (
	"strings"
	"testing"
)

func uint64Ptr(v uint64) *uint64 { return &v }

func TestToMarkdownBasicFields(t *testing.T) {
	r := BenchmarkResult{
		Model:               "mlx-community/Qwen3-4B-4bit",
		Chip:                "Apple M2 Pro",
		UnifiedMemoryBytes:  32 * 1024 * 1024 * 1024,
		TokensPerSecond:     42.345,
		TimeToFirstTokenMs:  123.4,
		Method:              "default",
		OptimizedMethodUsed: "kivi",
	}

	md := r.ToMarkdown()

	for _, want := range []string{
		"VeloxQuant Benchmark",
		"Model: mlx-community/Qwen3-4B-4bit",
		"Machine: Apple M2 Pro",
		"RAM: 32GB",
		"Tokens/sec: 42.3",
		"TTFT: 123ms",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("ToMarkdown() missing %q; got:\n%s", want, md)
		}
	}
}

func TestToMarkdownUnknownChip(t *testing.T) {
	r := BenchmarkResult{Model: "m", TokensPerSecond: 1, TimeToFirstTokenMs: 1}
	md := r.ToMarkdown()
	if !strings.Contains(md, "Machine: unknown") {
		t.Errorf("ToMarkdown() = %q, want it to report Machine: unknown", md)
	}
}

func TestToMarkdownResidentMemoryReduced(t *testing.T) {
	r := BenchmarkResult{
		Model:                      "m",
		Method:                     "default",
		OptimizedMethodUsed:        "kivi",
		DefaultMethodResidentBytes: uint64Ptr(2000 * 1024 * 1024),
		OptimizedResidentBytes:     uint64Ptr(1000 * 1024 * 1024),
	}
	md := r.ToMarkdown()

	if !strings.Contains(md, "default resident memory: 2000MB") {
		t.Errorf("ToMarkdown() missing default resident memory line; got:\n%s", md)
	}
	if !strings.Contains(md, "kivi resident memory: 1000MB") {
		t.Errorf("ToMarkdown() missing optimized resident memory line; got:\n%s", md)
	}
	if !strings.Contains(md, "Resident memory reduced: 50%") {
		t.Errorf("ToMarkdown() missing reduction line; got:\n%s", md)
	}
	if strings.Contains(md, "accounting-only") {
		t.Errorf("ToMarkdown() should not include the accounting-only caveat on a real reduction; got:\n%s", md)
	}
}

// TestToMarkdownResidentMemoryIncreasedIncludesAccountingOnlyCaveat is the
// load-bearing test for the honesty note the build prompt explicitly
// requires preserving: when the optimized method's measured resident
// memory is *higher* than the default method's, ToMarkdown must say so
// plainly and include the "compression is accounting-only" caveat, rather
// than silently reporting only the percentage.
func TestToMarkdownResidentMemoryIncreasedIncludesAccountingOnlyCaveat(t *testing.T) {
	r := BenchmarkResult{
		Model:                      "m",
		Method:                     "default",
		OptimizedMethodUsed:        "kivi",
		DefaultMethodResidentBytes: uint64Ptr(1000 * 1024 * 1024),
		OptimizedResidentBytes:     uint64Ptr(1200 * 1024 * 1024),
	}
	md := r.ToMarkdown()

	if !strings.Contains(md, "Resident memory increased: 20%") {
		t.Errorf("ToMarkdown() missing increase line; got:\n%s", md)
	}
	if !strings.Contains(md, "accounting-only") {
		t.Errorf("ToMarkdown() must include the accounting-only caveat when optimized RSS is higher; got:\n%s", md)
	}
}

func TestToMarkdownOmitsResidentMemorySectionWhenUnmeasured(t *testing.T) {
	r := BenchmarkResult{Model: "m", Method: "default", OptimizedMethodUsed: "kivi"}
	md := r.ToMarkdown()
	if strings.Contains(md, "resident memory") {
		t.Errorf("ToMarkdown() should omit the resident-memory section when both measurements are nil; got:\n%s", md)
	}
}

func TestToMarkdownOmitsResidentMemorySectionWhenPartiallyMeasured(t *testing.T) {
	r := BenchmarkResult{
		Model:                      "m",
		Method:                     "default",
		OptimizedMethodUsed:        "kivi",
		DefaultMethodResidentBytes: uint64Ptr(1000 * 1024 * 1024),
		// OptimizedResidentBytes left nil, e.g. the process had already
		// exited or `ps` failed.
	}
	md := r.ToMarkdown()
	if strings.Contains(md, "resident memory:") {
		t.Errorf("ToMarkdown() should omit the resident-memory section when only one side was measured; got:\n%s", md)
	}
}

func TestGetResidentBytesReturnsNilForInvalidPID(t *testing.T) {
	// PID 0 (or a very unlikely-to-exist PID) should make `ps` fail or
	// return nothing parseable, and getResidentBytes must return nil, not
	// panic or return a bogus value.
	if got := getResidentBytes(0); got != nil {
		t.Errorf("getResidentBytes(0) = %v, want nil", got)
	}
}

func TestBenchmarkInputMaxTokensDefault(t *testing.T) {
	if got := (BenchmarkInput{}).maxTokens(); got != defaultBenchmarkMaxTokens {
		t.Errorf("maxTokens() = %d, want %d", got, defaultBenchmarkMaxTokens)
	}
	if got := (BenchmarkInput{MaxTokens: 64}).maxTokens(); got != 64 {
		t.Errorf("maxTokens() = %d, want 64", got)
	}
}

func TestBenchmarkRejectsEmptyModel(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = Benchmark(t.Context(), client, BenchmarkInput{})
	if err == nil {
		t.Fatal("expected error for empty model")
	}
}
