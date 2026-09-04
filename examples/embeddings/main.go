// Example: requesting embeddings for a batch of strings from a local
// VeloxQuant runtime.
package main

import (
	"context"
	"fmt"

	veloxquant "github.com/rajveer43/veloxquant-go"
)

func main() {
	ctx := context.Background()

	client, err := veloxquant.NewClient(
		veloxquant.WithAutoDetect(),
	)
	if err != nil {
		panic(err)
	}

	response, err := client.Embed(ctx, veloxquant.EmbedRequest{
		Model: "mlx-community/all-MiniLM-L6-v2-4bit",
		Input: []string{
			"The quick brown fox jumps over the lazy dog.",
			"VeloxQuant runs local LLMs efficiently on Apple Silicon.",
		},
	})
	if err != nil {
		panic(err)
	}

	for _, e := range response.Data {
		fmt.Printf("embedding %d: dim=%d first-values=%v\n", e.Index, len(e.Vector), e.Vector[:min(3, len(e.Vector))])
	}
}
