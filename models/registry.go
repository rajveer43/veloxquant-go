package models

import "github.com/rajveer43/veloxquant-go/memory"

// staticRegistry is a curated, hardcoded list of known-good local models.
// A future version can replace or augment this with a remote registry
// without changing the Registry interface.
var staticRegistry = []Info{
	{
		Name:       "mlx-community/Qwen3-8B-4bit",
		Parameters: 8_000_000_000,
		Architecture: memory.Architecture{
			NumLayers:      36,
			NumKVHeads:     8,
			HeadDim:        128,
			HiddenSize:     4096,
			ParameterCount: 8_000_000_000,
		},
		Tasks:       []Task{TaskChat, TaskReasoning, TaskAgent},
		Supported:   true,
		Recommended: true,
	},
	{
		Name:       "mlx-community/Qwen3-Coder-4bit",
		Parameters: 8_000_000_000,
		Architecture: memory.Architecture{
			NumLayers:      36,
			NumKVHeads:     8,
			HeadDim:        128,
			HiddenSize:     4096,
			ParameterCount: 8_000_000_000,
		},
		Tasks:       []Task{TaskCoding, TaskAgent},
		Supported:   true,
		Recommended: true,
	},
	{
		Name:       "mlx-community/gemma-2-9b-it-4bit",
		Parameters: 9_000_000_000,
		Architecture: memory.Architecture{
			NumLayers:      42,
			NumKVHeads:     8,
			HeadDim:        256,
			HiddenSize:     3584,
			ParameterCount: 9_000_000_000,
		},
		Tasks:       []Task{TaskChat, TaskTranslation},
		Supported:   true,
		Recommended: false,
	},
	{
		Name:       "mlx-community/Llama-3.2-11B-Vision-Instruct-4bit",
		Parameters: 11_000_000_000,
		Architecture: memory.Architecture{
			NumLayers:      40,
			NumKVHeads:     8,
			HeadDim:        128,
			HiddenSize:     4096,
			ParameterCount: 11_000_000_000,
		},
		Tasks:       []Task{TaskVision, TaskChat},
		Supported:   true,
		Recommended: false,
	},
}

// Registry provides access to known model metadata. It is an interface so
// tests and future remote-backed implementations can substitute their own
// data.
type Registry interface {
	List() []Info
	Get(name string) (Info, bool)
}

type staticRegistryImpl struct {
	models []Info
}

// NewRegistry returns the default Registry, backed by a curated static
// list of models.
func NewRegistry() Registry {
	return staticRegistryImpl{models: staticRegistry}
}

func (r staticRegistryImpl) List() []Info {
	out := make([]Info, len(r.models))
	copy(out, r.models)
	return out
}

func (r staticRegistryImpl) Get(name string) (Info, bool) {
	for _, m := range r.models {
		if m.Name == name {
			return m, true
		}
	}
	return Info{}, false
}
