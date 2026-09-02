package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	veloxquant "github.com/rajveer43/veloxquant-go"
)

func runBenchmark(args []string) error {
	fs := flag.NewFlagSet("benchmark", flag.ExitOnError)
	contextLength := fs.Int("context", 4096, "context length in tokens")
	prompt := fs.String("prompt", "Write a short haiku about the ocean.", "prompt to send")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: vq benchmark <model> [--context N] [--prompt TEXT]")
	}
	modelName := fs.Arg(0)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client, err := veloxquant.NewClient()
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	status, err := client.Runtime.Health(ctx)
	if err != nil || !status.Healthy {
		return fmt.Errorf("veloxquant runtime unavailable; start the runtime to benchmark inference (see `vq serve`)")
	}

	arch, _ := resolveArchitecture(client, modelName)
	estimate, err := client.Memory.Estimate(ctx, veloxquant.MemoryRequest{
		Model:         arch,
		ContextLength: *contextLength,
		Precision:     veloxquant.Int4,
	})
	if err != nil {
		return fmt.Errorf("estimate memory for %s: %w", modelName, err)
	}

	start := time.Now()
	resp, err := client.Chat(ctx, veloxquant.ChatRequest{
		Model: modelName,
		Messages: []veloxquant.Message{
			{Role: "user", Content: *prompt},
		},
	})
	elapsed := time.Since(start)
	if err != nil {
		return fmt.Errorf("benchmark %s: %w", modelName, err)
	}

	tokensPerSec := 0.0
	if elapsed > 0 && resp.Usage.CompletionTokens > 0 {
		tokensPerSec = float64(resp.Usage.CompletionTokens) / elapsed.Seconds()
	}

	fmt.Println("Benchmark Results")
	fmt.Println()
	fmt.Printf("Tokens/sec:\n%.1f\n\n", tokensPerSec)
	fmt.Printf("Total Duration:\n%s\n\n", elapsed.Round(time.Millisecond))
	fmt.Printf("Peak Memory (estimated):\n%s\n\n", veloxquant.FormatBytes(estimate.TotalMemoryBytes))
	fmt.Printf("KV Cache Memory (estimated):\n%s\n\n", veloxquant.FormatBytes(estimate.KVCacheMemoryBytes))
	fmt.Printf("Compression Ratio:\n%.1f%% saved\n", estimate.SavedPercent)

	return nil
}
