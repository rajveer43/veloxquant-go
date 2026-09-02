package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	veloxquant "github.com/rajveer43/veloxquant-go"
)

func runAnalyze(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	contextLength := fs.Int("context", 32768, "context length in tokens")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: vq analyze <model> [--context N]")
	}
	modelName := fs.Arg(0)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := veloxquant.NewClient()
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	arch, err := resolveArchitecture(client, modelName)
	if err != nil {
		return err
	}

	estimate, err := client.Memory.Estimate(ctx, veloxquant.MemoryRequest{
		Model:         arch,
		ContextLength: *contextLength,
		Precision:     veloxquant.FP16,
	})
	if err != nil {
		return fmt.Errorf("estimate memory for %s: %w", modelName, err)
	}

	fmt.Println("Model Analysis")
	fmt.Println()
	fmt.Printf("Model:\n%s\n\n", modelName)
	fmt.Printf("Model Memory:\n%s\n\n", veloxquant.FormatBytes(estimate.ModelMemoryBytes))
	fmt.Printf("KV Cache at %d tokens:\n%s\n\n", *contextLength, veloxquant.FormatBytes(estimate.KVCacheMemoryBytes))
	fmt.Printf("Without Optimization:\n%s\n\n", veloxquant.FormatBytes(estimate.TotalMemoryBytes))
	fmt.Printf("With VeloxQuant:\n%s\n\n", veloxquant.FormatBytes(estimate.OptimizedTotalBytes))
	fmt.Printf("Memory Saved:\n%s (%.1f%%)\n", veloxquant.FormatBytes(estimate.SavedBytes), estimate.SavedPercent)

	return nil
}

func resolveArchitecture(client *veloxquant.Client, modelName string) (veloxquant.ModelArchitecture, error) {
	for _, m := range client.Models.List() {
		if m.Name == modelName {
			return m.Architecture, nil
		}
	}

	// Fall back to a representative 7-8B-class architecture so `vq
	// analyze` remains useful for models outside the curated registry.
	return veloxquant.ModelArchitecture{
		NumLayers:      32,
		NumKVHeads:     8,
		HeadDim:        128,
		HiddenSize:     4096,
		ParameterCount: 7_000_000_000,
	}, nil
}
