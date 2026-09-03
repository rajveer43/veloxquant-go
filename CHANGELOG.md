# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/rajveer43/veloxquant-go/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/rajveer43/veloxquant-go/releases/tag/v0.1.0
