# VeloxQuant Go

[![CI](https://github.com/rajveer43/veloxquant-go/actions/workflows/ci.yml/badge.svg)](https://github.com/rajveer43/veloxquant-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/rajveer43/veloxquant-go.svg)](https://pkg.go.dev/github.com/rajveer43/veloxquant-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/rajveer43/veloxquant-go)](https://goreportcard.com/report/github.com/rajveer43/veloxquant-go)

**Memory intelligence and optimization for local AI, in Go.**

VeloxQuant Go is not a wrapper around MLX. It's a Go-native toolkit for
building local AI infrastructure: hardware detection, model and KV-cache
memory estimation, VeloxQuant compression recommendations, and a client for
talking to a local VeloxQuant runtime — all without needing to understand
MLX or manually calculate memory requirements.

```text
Go Application
      │
      ▼
VeloxQuant Go SDK
      │
      ├── Hardware Intelligence
      ├── Memory Estimation
      ├── KV Cache Optimization
      ├── AutoPilot
      └── Runtime Client
               │
               ▼
      VeloxQuant Runtime / MLX
               │
               ▼
        Apple Silicon
```

Part of the VeloxQuant ecosystem: [VeloxQuant-MLX](https://github.com/rajveer43) (Python optimization engine), VeloxQuant Studio (macOS app), VeloxQuant VS Code, and the VeloxQuant npm SDK.

## Installation

```bash
go get github.com/rajveer43/veloxquant-go
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"

	veloxquant "github.com/rajveer43/veloxquant-go"
)

func main() {
	client, err := veloxquant.NewClient()
	if err != nil {
		panic(err)
	}

	response, err := client.Chat(context.Background(), veloxquant.ChatRequest{
		Model: "mlx-community/Qwen3-8B-4bit",
		Messages: []veloxquant.Message{
			{Role: "user", Content: "Hello!"},
		},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(response.Text)
}
```

## Streaming

```go
stream, err := client.ChatStream(ctx, veloxquant.ChatRequest{
	Model: "mlx-community/Qwen3-8B-4bit",
	Messages: []veloxquant.Message{
		{Role: "user", Content: "Write a Go HTTP server."},
	},
})
if err != nil {
	panic(err)
}
defer stream.Close()

for stream.Next() {
	fmt.Print(stream.Chunk().Text)
}
if err := stream.Err(); err != nil {
	panic(err)
}
```

## Memory Estimation

Estimate model and KV-cache memory before you load anything:

```go
estimate, err := client.Memory.Estimate(ctx, veloxquant.MemoryRequest{
	Model: veloxquant.ModelArchitecture{
		NumLayers:      36,
		NumKVHeads:     8,
		HeadDim:        128,
		HiddenSize:     4096,
		ParameterCount: 8_000_000_000,
	},
	ContextLength: 32768,
	Precision:     veloxquant.Int4,
})

fmt.Println(veloxquant.FormatBytes(estimate.TotalMemoryBytes))
fmt.Println(veloxquant.FormatBytes(estimate.OptimizedTotalBytes))
fmt.Printf("%.1f%% saved\n", estimate.SavedPercent)
```

KV-cache memory is computed as:

```text
KV Cache Memory = Layers × Tokens × KV Heads × Head Dimension × 2 × Bytes Per Element
```

Supported precisions: `FP16`, `FP8`, `Int8`, `Int4`.

## Optimization Profiles

```go
rec, err := client.Optimize.Recommend(ctx, veloxquant.OptimizationRequest{
	Model:         "Qwen3-8B",
	Architecture:  arch,
	ContextLength: 32768,
})

fmt.Println(rec.Profile)             // speed | balanced | memory | maximum-context
fmt.Println(rec.CompressionBits)     // e.g. 4
fmt.Println(rec.Reason)
```

## AutoPilot

AutoPilot inspects your hardware, picks a compatible model, chooses a safe
context length and compression strategy, and returns a ready-to-use
session:

```go
session, err := client.AutoPilot(ctx, veloxquant.AutoPilotConfig{
	Task:  "coding",
	Model: "auto",
})
if err != nil {
	panic(err)
}

plan := session.Plan() // fully transparent decision trail
fmt.Println(plan.SelectedModel, plan.ContextLength, plan.Profile)

response, err := session.Chat(ctx, "Build a REST API in Go")
```

## System Detection

```go
info, err := client.System.Info(ctx)

fmt.Println(info.Platform, info.Architecture)
fmt.Println(info.AppleSilicon)
fmt.Println(veloxquant.FormatBytes(info.TotalMemory))
fmt.Println(info.RecommendedProfile)
```

Apple Silicon-specific detection degrades gracefully on Linux and Windows —
`AppleSilicon` is simply `false`, and the SDK never panics on unsupported
platforms.

## Monitoring

```go
mon := client.Monitor(veloxquant.WithMonitorInterval(2 * time.Second))
mon.Start(ctx)

mon.Subscribe(func(m monitor.Metrics) {
	fmt.Println(veloxquant.FormatBytes(m.MemoryUsedBytes))
	fmt.Printf("%.1f tok/s\n", m.TokensPerSecond)
})
```

The Monitor samples system memory on `WithMonitorInterval`'s schedule (5s
by default). Between samples, every `Chat`/`ChatStream` call made through
the same `Client` also pushes a live update carrying that request's
`TokensPerSecond` and `TimeToFirstToken`, so subscribers see inference
performance as it happens rather than waiting for the next tick.

## CLI

```bash
go install github.com/rajveer43/veloxquant-go/cmd/vq@latest
```

```bash
vq doctor              # check system readiness
vq analyze Qwen3-8B    # memory breakdown for a model
vq recommend           # recommended models + profile for this hardware
vq benchmark Qwen3-8B  # tokens/sec, TTFT, memory (requires a running runtime)
vq serve               # connect to a local VeloxQuant runtime
vq serve --model mlx-community/Qwen3-8B-4bit   # launch a runtime for this model
```

`vq serve --model` launches the `veloxquant` CLI (from the
[VeloxQuant-MLX](https://github.com/rajveer43/VeloxQuant-MLX) Python
package) as a subprocess, waits for it to report readiness, and prints its
URL. Press Ctrl+C to stop it. Optional flags: `--method` (KV-cache
compression method), `--host`, `--port`.

## Architecture

```text
veloxquant-go/
├── client.go, config.go, types.go, errors.go, autopilot.go   Top-level API
├── system/       Hardware & platform detection (build-tagged per OS)
├── memory/       Model + KV-cache memory estimation
├── optimize/     Optimization profile recommendations
├── models/       Curated model registry + task-based recommendations
├── runtime/      HTTP client for the local VeloxQuant runtime
├── openai/       OpenAI-compatible chat completions + streaming
├── monitor/      Thread-safe memory/inference metrics monitoring
├── cmd/vq/       CLI
└── examples/     Runnable examples
```

Every subsystem is exposed as an interface (`system.Detector`,
`memory.Estimator`, `optimize.Optimizer`, `models.Registry`) so it can be
mocked in tests without touching real hardware or a live runtime.

## Examples

See [examples/](examples/) for runnable programs: `chat`, `streaming`,
`autopilot`, and `server`.

## Testing

```bash
go test ./...
go test -race ./...
go vet ./...
```

## Package Stats

Go modules have no central download counter (unlike npm/PyPI), so adoption
is tracked with the closest available public signals:

- **[Imported by](https://pkg.go.dev/github.com/rajveer43/veloxquant-go?tab=importedby)**
  on pkg.go.dev — count of public modules that import this package.
- **Clone/view traffic** — `go get` and `git clone` both register as repo
  clones. GitHub exposes 14 days of this under
  [Insights → Traffic](https://github.com/rajveer43/veloxquant-go/graphs/traffic)
  (maintainer access required). A [scheduled workflow](.github/workflows/traffic.yml)
  snapshots these counts weekly into `traffic-history.json` (committed on
  first run) so history survives past GitHub's 14-day retention window.

To refresh the history immediately: `gh workflow run traffic.yml`.

## Releasing

Releases are cut from tags:

1. Update `CHANGELOG.md`, moving the relevant `[Unreleased]` entries under a
   new `## [x.y.z] - YYYY-MM-DD` heading.
2. Commit, then tag: `git tag vX.Y.Z && git push origin vX.Y.Z`.
3. The [release workflow](.github/workflows/release.yml) runs CI against the
   tag and publishes a GitHub release with auto-generated notes.

pkg.go.dev picks up new tags automatically via the Go module proxy — no
separate publish step is needed there.

## License

MIT
