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
	method := fs.String("method", "", "KV-cache compression method to compare against the default (empty: runtime picks automatically)")
	maxTokens := fs.Int("max-tokens", 0, "max tokens to generate per benchmarked request (default: 128)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: vq benchmark <model> [--method NAME] [--max-tokens N]")
	}
	modelName := fs.Arg(0)

	// Loading two full model instances sequentially (see veloxquant.Benchmark)
	// can take several minutes for larger models, well beyond a single chat
	// request's usual timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	client, err := veloxquant.NewClient()
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	result, err := veloxquant.Benchmark(ctx, client, veloxquant.BenchmarkInput{
		Model:           modelName,
		OptimizedMethod: *method,
		MaxTokens:       *maxTokens,
	})
	if err != nil {
		return fmt.Errorf("benchmark %s: %w", modelName, err)
	}

	fmt.Println(result.ToMarkdown())

	return nil
}
