package veloxquant

import (
	"time"

	"github.com/rajveer43/veloxquant-go/memory"
)

// Precision re-exports memory.Precision at the top level so callers don't
// need to import the memory subpackage for common usage.
type Precision = memory.Precision

const (
	FP16 = memory.FP16
	FP8  = memory.FP8
	Int8 = memory.Int8
	Int4 = memory.Int4
)

// ModelArchitecture re-exports memory.Architecture at the top level.
type ModelArchitecture = memory.Architecture

// SystemInfo describes the host system relevant to running local LLM
// inference.
type SystemInfo struct {
	Platform     string
	Architecture string
	CPUModel     string
	AppleSilicon bool

	TotalMemory     uint64
	AvailableMemory uint64

	RecommendedProfile string
}

// MemoryRequest describes a memory estimation query.
type MemoryRequest struct {
	Model         ModelArchitecture
	ContextLength int
	Precision     Precision
}

// MemoryEstimate is the result of a memory estimation.
type MemoryEstimate struct {
	ModelMemoryBytes     uint64
	KVCacheMemoryBytes   uint64
	RuntimeOverheadBytes uint64
	TotalMemoryBytes     uint64

	OptimizedKVBytes    uint64
	OptimizedTotalBytes uint64
	SavedBytes          uint64
	SavedPercent        float64

	RecommendedStrategy string
}

// Message is a single chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest describes a chat completion request.
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`

	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`

	Stream bool `json:"stream,omitempty"`
}

// Usage reports token accounting for a chat completion.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// InferenceMetrics reports performance characteristics of a completed
// inference request.
type InferenceMetrics struct {
	TokensPerSecond  float64       `json:"tokens_per_second"`
	TimeToFirstToken time.Duration `json:"time_to_first_token"`
	TotalDuration    time.Duration `json:"total_duration"`
}

// ChatResponse is the result of a chat completion request.
type ChatResponse struct {
	ID    string `json:"id"`
	Model string `json:"model"`
	Text  string `json:"text"`

	Usage Usage `json:"usage"`

	Metrics InferenceMetrics `json:"metrics"`
}

// ChatChunk is a single incremental piece of a streamed chat response.
type ChatChunk struct {
	Text string
	Done bool
}

// LocalModelInfo describes a model found in the local model cache, as
// returned by ModelsService.Local.
type LocalModelInfo struct {
	Name         string     `json:"name"`
	Path         string     `json:"path"`
	SizeBytes    uint64     `json:"size_bytes"`
	LastModified *time.Time `json:"last_modified,omitempty"`
}
