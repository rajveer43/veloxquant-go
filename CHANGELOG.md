# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `veloxquant.Benchmark(ctx, client, input)`: a reusable library function
  measuring tokens/sec, time-to-first-token, and measured resident memory
  (RSS, via `ps -o rss= -p <pid>`) for a model, comparing the default serve
  method against an optimized one across two full sequential runtime loads
  (never concurrent). `BenchmarkResult.ToMarkdown()` renders a report,
  including an explicit "compression is accounting-only" caveat when
  optimized RSS measures *higher* than the default method's. `runtime.Process`
  gains `PID()`/`Method()` accessors to support this. `cmd/vq/benchmark.go`
  is rewritten to call this function instead of its previous ad hoc
  single-shot wall-clock timing — a CLI output-shape change (see README).
  Not unit-testable in CI (real Apple Silicon + a downloaded model
  required); see the new build-tagged (`manual`) `benchmark_manual_test.go`.
- `mcp/`: a separate Go module (own `go.mod`, isolated like `langchain/`)
  letting an `agent.Agent` pull tools from a Model Context Protocol server,
  backed by the official `github.com/modelcontextprotocol/go-sdk` v1.7.0.
  `mcp.Connect`/`mcp.FromSession` build an `mcp.ToolSource`, passed to the
  new `Agent.UseMcpServer(ctx, source)` — a deliberate API-shape divergence
  from the TS SDK's `useMcpServer(config)`, documented in the package doc
  comment, since Go has no dynamic-import equivalent to keep the `agent`
  package MCP-SDK-free otherwise. `unwrapMcpToolResult` matches `mcp.ts`'s
  content-handling rules exactly (structuredContent preferred; single text
  block tried as JSON then raw string; other content types return an
  actionable error). A tool-name collision closes the newly-opened MCP
  connection before returning the error. See `examples/mcp`.
- `agent/`: a single-turn tool-calling loop (`agent.New`, `Agent.RegisterTool`,
  `Agent.Run`) over `*veloxquant.Client`, reusing the Phase-0 tool-calling
  wire types end to end. `Tool` is a plain interface so any type (including
  MCP-sourced tools) can implement it. `RunOptions.MaxSteps` defaults to 8;
  exceeding it returns an error wrapping the new `agent.ErrAgentMaxStepsExceeded`.
  A malformed tool-call-arguments payload, an unknown tool name, or a
  tool's `Execute` returning an error are all fed back to the model as a
  structured error result rather than aborting the run. No third-party
  dependency; lives in the root module. See `examples/agent`.
- OpenAI-compatible tool-calling wire types: `openai.ToolDefinition`/
  `ToolFunction`/`ToolCall`/`ToolCallFunction`, `Message.ToolCalls`/
  `ToolCallID`, `ChatRequest.Tools`, and `ChatChoice.FinishReason` (also
  re-exported at the root package level). Purely additive; prerequisite
  for the `agent` package.
- `models.Pull` / `models.Delete` (and `Client.Models.Pull` / `.Delete`),
  exposed via `vq models pull <id>` / `vq models delete <id>`. Unlike
  `Models.Local` (a dependency-free filesystem scan), these shell out to a
  Python interpreter with `huggingface_hub` importable — see the new
  "Local Model Cache" README section for why. New sentinel errors
  `models.ErrHuggingFaceHubUnavailable` (re-exported as
  `veloxquant.ErrHuggingFaceHubUnavailable`) and `models.ErrLocalModelNotFound`.
- `ChatRequest.ResponseFormat`, with `veloxquant.JSONMode()` and
  `veloxquant.JSONSchema(name, schema, strict)` helpers, passed through to
  the runtime's OpenAI-compatible endpoint. The VeloxQuant runtime serves
  completions via `mlx_lm.server`, which does not enforce `response_format`
  server-side as of this writing; see the `ResponseFormat` doc comment and
  the new `examples/structured` for the prompt-and-validate pattern this
  requires in the meantime.
- `Models.Local` scans the local model cache directory (MLX/Hugging Face
  hub convention) and reports downloaded models, their size on disk, and
  last-modified time; exposed via `vq models --local`.
- `Client.NewConversation` / `Conversation.Send` / `Conversation.SendStream`:
  a history-tracking wrapper around `Chat`/`ChatStream` so callers don't
  have to rebuild `[]Message` on every turn. A failed turn leaves history
  unchanged. Also available as `session.Conversation(system)` on an
  AutoPilot `Session`.
- `Client.Embed` and `openai.Client.Embeddings`: embeddings support via the
  OpenAI-compatible `/v1/embeddings` endpoint, mirroring the `Chat` request
  pattern. `EmbedRequest.Input` accepts a single string or a `[]string`.
- `langchain/`: a separate Go module implementing langchaingo's
  (`github.com/tmc/langchaingo`) `llms.Model` interface backed by
  `veloxquant.Client`, so a local VeloxQuant runtime can serve as the model
  in a langchaingo chain. Kept out of the root module's dependency graph;
  see `examples/langchain`.

### Changed
- `vq benchmark`'s output format and flags (`--method`/`--max-tokens`
  replace `--context`/`--prompt`), now backed by `veloxquant.Benchmark`
  instead of the previous ad hoc single-shot wall-clock timing — see the
  README's Benchmark section for the full rationale. This is a behavior
  change for existing `vq benchmark` users, even though the underlying
  measurement is strictly more capable.

## [0.3.0] - 2026-09-03

### Added
- `models.RecommendScored` ranks model candidates by task fit, curated-registry
  bonus, and memory headroom instead of registry order; `AutoPilotPlan.Reason`
  now explains why a model was selected, not just the compression choice.
- `Chat` and `ChatStream` report real `TokensPerSecond` and `TimeToFirstToken`.
- `Client.Monitor` accepts `WithMonitorInterval`, and a new `Monitor.Report`
  pushes live per-request inference metrics to subscribers between periodic
  memory samples.
- `runtime.Process`: launches the `veloxquant` CLI as a subprocess, waits for
  its `VELOXQUANT_READY` stdout handshake, and stops it via SIGTERM (Kill on
  Windows).
- `vq serve --model <id>` launches a local VeloxQuant runtime process
  directly, with `--method`/`--host`/`--port`; bare `vq serve` still connects
  to and reports on an already-running runtime.

## [0.2.0] - 2026-09-03

### Added
- CI workflow running `go build`, `go vet`, `gofmt -l`, `go test`, `go test -race`,
  and `staticcheck` on every push and pull request.
- Release workflow that publishes a GitHub release with auto-generated notes
  on every `v*` tag push.

## [0.1.0] - 2026-09-02

### Added
- Hardware and platform detection (`system`), including Apple Silicon
  identification with graceful degradation on other platforms.
- Model and KV-cache memory estimation (`memory`) across FP16, FP8, Int8,
  and Int4 precisions.
- VeloxQuant optimization profile recommendations (`optimize`): speed,
  balanced, memory, and maximum-context profiles.
- Curated model registry with task-based recommendations (`models`).
- HTTP client for a local VeloxQuant runtime, including health checks
  (`runtime`).
- OpenAI-compatible chat completions, including SSE streaming (`openai`).
- AutoPilot: automatic hardware-aware model, context length, and
  compression selection with a fully transparent decision trail
  (`autopilot.go`).
- Thread-safe memory and inference metrics monitoring (`monitor`).
- `vq` CLI: `doctor`, `analyze`, `recommend`, `benchmark`, `serve`.
- Runnable examples: `chat`, `streaming`, `autopilot`, `server`.

[Unreleased]: https://github.com/rajveer43/veloxquant-go/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/rajveer43/veloxquant-go/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/rajveer43/veloxquant-go/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/rajveer43/veloxquant-go/releases/tag/v0.1.0
