// Example: a single-turn tool-calling agent, using the model's native
// tool-calling support (confirmed working against
// mlx-community/Qwen3-4B-4bit — not every model's tokenizer supports
// this).
package main

import (
	"context"
	"encoding/json"
	"fmt"

	veloxquant "github.com/rajveer43/veloxquant-go"
	"github.com/rajveer43/veloxquant-go/agent"
)

// weatherTool implements agent.Tool.
type weatherTool struct{}

func (weatherTool) Name() string        { return "get_weather" }
func (weatherTool) Description() string { return "Get the current weather for a location" }
func (weatherTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"location": map[string]any{"type": "string", "description": "City name"},
		},
		"required": []string{"location"},
	}
}

func (weatherTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Location string `json:"location"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}
	// A real tool would call a weather API here.
	return map[string]any{"location": params.Location, "tempC": 22, "condition": "sunny"}, nil
}

func main() {
	ctx := context.Background()

	client, err := veloxquant.NewClient(veloxquant.WithAutoDetect())
	if err != nil {
		panic(err)
	}

	a := agent.New(client, "mlx-community/Qwen3-4B-4bit")
	if err := a.RegisterTool(weatherTool{}); err != nil {
		panic(err)
	}

	result, err := a.Run(ctx,
		"What is the weather in Tokyo? Use the get_weather tool, then tell me if I need an umbrella.",
		agent.RunOptions{},
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(result.Text)

	stepsJSON, _ := json.MarshalIndent(result.Steps, "", "  ")
	fmt.Println("\nSteps taken:", string(stepsJSON))
}
