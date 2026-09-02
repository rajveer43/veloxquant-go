// Example: letting AutoPilot select a model, context length, and
// compression strategy based on detected hardware.
package main

import (
	"context"
	"fmt"

	veloxquant "github.com/rajveer43/veloxquant-go"
)

func main() {
	ctx := context.Background()

	client, err := veloxquant.NewClient()
	if err != nil {
		panic(err)
	}

	session, err := client.AutoPilot(ctx, veloxquant.AutoPilotConfig{
		Task:  "coding",
		Model: "auto",
	})
	if err != nil {
		panic(err)
	}

	plan := session.Plan()
	fmt.Printf("Hardware:\n%s\n%s\n\n", plan.Hardware.CPUModel, veloxquant.FormatBytes(plan.Hardware.TotalMemory))
	fmt.Printf("Selected Model:\n%s\n\n", plan.SelectedModel)
	fmt.Printf("Context:\n%d tokens\n\n", plan.ContextLength)
	fmt.Printf("KV Compression:\n%d-bit\n\n", plan.CompressionBits)
	fmt.Printf("Estimated Memory:\n%s\n\n", veloxquant.FormatBytes(plan.EstimatedMemoryBytes))
	fmt.Printf("Safety Margin:\n%s\n\n", veloxquant.FormatBytes(plan.SafetyMarginBytes))
	fmt.Printf("Profile:\n%s\n", plan.Profile)

	response, err := session.Chat(ctx, "Build a REST API in Go")
	if err != nil {
		panic(err)
	}

	fmt.Println(response.Text)
}
