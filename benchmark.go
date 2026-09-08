package veloxquant

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/rajveer43/veloxquant-go/runtime"
)

// benchmarkPrompt is the fixed prompt Benchmark sends for both the default
// and optimized runs, so the two measurements are comparable.
const benchmarkPrompt = "Write a three-sentence summary of how photosynthesis works."

// defaultBenchmarkMaxTokens is used when BenchmarkInput.MaxTokens is left
// at its zero value.
const defaultBenchmarkMaxTokens = 128

// BenchmarkInput configures a Benchmark run.
type BenchmarkInput struct {
	// Model is the Hugging Face model id (or local path) to benchmark.
	Model string
	// OptimizedMethod names the KV-cache compression method to compare
	// against the default (unoptimized) method, e.g. "kivi". Empty lets
	// the runtime pick automatically (optimize: "auto").
	OptimizedMethod string
	// MaxTokens caps each benchmarked generation. Defaults to 128.
	MaxTokens int
}

func (i BenchmarkInput) maxTokens() int {
	if i.MaxTokens > 0 {
		return i.MaxTokens
	}
	return defaultBenchmarkMaxTokens
}

// BenchmarkResult reports the outcome of a Benchmark run. Call ToMarkdown
// to render it as a human-readable report.
type BenchmarkResult struct {
	Model              string
	Chip               string
	UnifiedMemoryBytes uint64

	TokensPerSecond    float64
	TimeToFirstTokenMs float64

	// DefaultMethodResidentBytes and OptimizedResidentBytes are measured
	// resident memory (RSS) of the runtime subprocess for each method,
	// sampled once right after each model finishes loading. nil means the
	// measurement couldn't be taken (e.g. the process had already exited,
	// or `ps` failed) — matching TS's `| null` for the same case, not a
	// zero value that could be mistaken for "measured zero bytes".
	//
	// This is real, measured memory, but it reflects idle model-load RSS,
	// not KV-cache growth under load — and compression is not guaranteed
	// to lower it: it can measure smaller in accounting terms (see
	// MemoryEstimate) while resident memory stays flat or even increases.
	DefaultMethodResidentBytes *uint64
	OptimizedResidentBytes     *uint64

	Method              string
	OptimizedMethodUsed string
}

// getResidentBytes reads RSS (bytes) for pid via `ps`. Returns nil if the
// process has already exited or ps fails — matching benchmark.ts's
// getResidentBytes, which returns null in the same situations. There is no
// separate Windows implementation: `ps` isn't available there at all
// (mirroring runtime/process_windows.go's precedent of a degraded,
// non-fatal path on Windows rather than a parallel implementation), so RSS
// measurement simply returns nil on that platform too.
func getResidentBytes(pid int) *uint64 {
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return nil
	}
	kb, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return nil
	}
	bytes := kb * 1024
	return &bytes
}

// timedGeneration is the result of timeGeneration.
type timedGeneration struct {
	tokensPerSecond    float64
	timeToFirstTokenMs float64
}

// timeGeneration times a single streamed generation against an
// already-loaded model, measuring tokens/sec and time-to-first-token from
// chunk arrival timestamps at the SDK boundary — no new instrumentation
// added to the runtime itself, matching benchmark.ts's "measure at the SDK
// boundary" design (which is also how Client.ChatStream's own Metrics()
// method already works).
func timeGeneration(ctx context.Context, client *Client, model string, maxTokens int) (timedGeneration, error) {
	stream, err := client.ChatStream(ctx, ChatRequest{
		Model:     model,
		Messages:  []Message{{Role: "user", Content: benchmarkPrompt}},
		MaxTokens: maxTokens,
	})
	if err != nil {
		return timedGeneration{}, fmt.Errorf("benchmark: start generation: %w", err)
	}
	defer stream.Close()

	for stream.Next() {
	}
	if err := stream.Err(); err != nil {
		return timedGeneration{}, fmt.Errorf("benchmark: generation failed: %w", err)
	}

	m := stream.Metrics()
	return timedGeneration{
		tokensPerSecond:    m.TokensPerSecond,
		timeToFirstTokenMs: float64(m.TimeToFirstToken.Microseconds()) / 1000,
	}, nil
}

// runOneMethod launches a runtime process for model using the given
// method ("" lets the runtime choose automatically), measures its
// resident memory right after it becomes ready, times one generation
// against it (only when measureTiming is true — the default-method run
// times generation, the optimized-method run does not, matching
// benchmark.ts's scope: TS only measures tokens/sec and TTFT once, against
// the default method), and fully stops the process before returning —
// this benchmark never runs two model instances concurrently, since
// resource contention between them would skew both measurements.
func runOneMethod(ctx context.Context, model, method string, maxTokens int, measureTiming bool) (residentBytes *uint64, methodUsed string, timing timedGeneration, err error) {
	cfg := runtime.ProcessConfig{Model: model, Method: method}
	proc, startErr := runtime.StartProcess(ctx, cfg)
	if startErr != nil {
		return nil, "", timedGeneration{}, fmt.Errorf("benchmark: start runtime for method %q: %w", method, startErr)
	}
	defer func() { _ = proc.Stop(context.Background()) }()

	residentBytes = getResidentBytes(proc.PID())
	methodUsed = proc.Method()

	if measureTiming {
		client, clientErr := NewClient(WithRuntimeURL(proc.URL()))
		if clientErr != nil {
			return residentBytes, methodUsed, timedGeneration{}, fmt.Errorf("benchmark: build client: %w", clientErr)
		}
		timing, err = timeGeneration(ctx, client, model, maxTokens)
		if err != nil {
			return residentBytes, methodUsed, timedGeneration{}, err
		}
	}

	return residentBytes, methodUsed, timing, nil
}

// Benchmark measures tokens/sec, time-to-first-token, and resident memory
// for input.Model on this machine, comparing the default (unoptimized)
// serve method against an optimized one (input.OptimizedMethod, or the
// runtime's own automatic choice when empty). It launches two separate
// runtime processes sequentially — never concurrently, so resource
// contention doesn't skew either measurement — and fully stops each one
// before starting the next.
//
// Requires real Apple Silicon hardware and a downloaded model; this is not
// unit-testable in CI. See runtime/benchmark_manual_test.go (build tag
// "manual") for a by-hand verification harness.
func Benchmark(ctx context.Context, client *Client, input BenchmarkInput) (BenchmarkResult, error) {
	if input.Model == "" {
		return BenchmarkResult{}, fmt.Errorf("benchmark: %w: model must not be empty", ErrInvalidConfig)
	}

	info, err := client.System.Info(ctx)
	if err != nil {
		return BenchmarkResult{}, fmt.Errorf("benchmark: %w", err)
	}

	defaultResident, defaultMethodUsed, timing, err := runOneMethod(ctx, input.Model, "", input.maxTokens(), true)
	if err != nil {
		return BenchmarkResult{}, err
	}

	optimizedResident, optimizedMethodUsed, _, err := runOneMethod(ctx, input.Model, input.OptimizedMethod, input.maxTokens(), false)
	if err != nil {
		return BenchmarkResult{}, err
	}

	return BenchmarkResult{
		Model:                      input.Model,
		Chip:                       info.CPUModel,
		UnifiedMemoryBytes:         info.TotalMemory,
		TokensPerSecond:            timing.tokensPerSecond,
		TimeToFirstTokenMs:         timing.timeToFirstTokenMs,
		DefaultMethodResidentBytes: defaultResident,
		OptimizedResidentBytes:     optimizedResident,
		Method:                     defaultMethodUsed,
		OptimizedMethodUsed:        optimizedMethodUsed,
	}, nil
}

// ToMarkdown renders r as a human-readable Markdown report, matching the
// TS SDK's benchmark().toMarkdown() format. When both resident-memory
// measurements are available and the optimized method measured *higher*
// resident memory than the default, the report includes an explicit
// accounting-only caveat rather than silently reporting an increase as if
// it were a simple regression — compression byte counts (see
// MemoryEstimate) are not the same thing as measured RSS, and the two can
// diverge in either direction.
func (r BenchmarkResult) ToMarkdown() string {
	var b bytes.Buffer

	fmt.Fprintln(&b, "VeloxQuant Benchmark")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Model: %s\n", r.Model)
	if r.Chip != "" {
		fmt.Fprintf(&b, "Machine: %s\n", r.Chip)
	} else {
		fmt.Fprintln(&b, "Machine: unknown")
	}
	if r.UnifiedMemoryBytes > 0 {
		fmt.Fprintf(&b, "RAM: %.0fGB\n", float64(r.UnifiedMemoryBytes)/(1024*1024*1024))
	}
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Tokens/sec: %.1f\n", r.TokensPerSecond)
	fmt.Fprintf(&b, "TTFT: %.0fms\n", r.TimeToFirstTokenMs)
	fmt.Fprintln(&b)

	if r.DefaultMethodResidentBytes != nil && r.OptimizedResidentBytes != nil {
		beforeMB := float64(*r.DefaultMethodResidentBytes) / (1024 * 1024)
		afterMB := float64(*r.OptimizedResidentBytes) / (1024 * 1024)
		deltaPercent := ((beforeMB - afterMB) / beforeMB) * 100

		fmt.Fprintf(&b, "%s resident memory: %.0fMB\n", r.Method, beforeMB)
		fmt.Fprintf(&b, "%s resident memory: %.0fMB\n", r.OptimizedMethodUsed, afterMB)
		if deltaPercent >= 0 {
			fmt.Fprintf(&b, "Resident memory reduced: %.0f%%\n", deltaPercent)
		} else {
			fmt.Fprintf(&b, "Resident memory increased: %.0f%% (compression is accounting-only — see README)\n", -deltaPercent)
		}
	}

	return strings.TrimRight(b.String(), "\n")
}
