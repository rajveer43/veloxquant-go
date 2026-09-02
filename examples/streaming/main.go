// Example: streaming a chat completion token-by-token.
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

	stream, err := client.ChatStream(ctx, veloxquant.ChatRequest{
		Model: "mlx-community/Qwen3-8B-4bit",
		Messages: []veloxquant.Message{
			{
				Role:    "user",
				Content: "Write a Go HTTP server.",
			},
		},
	})
	if err != nil {
		panic(err)
	}
	defer stream.Close()

	for stream.Next() {
		chunk := stream.Chunk()
		fmt.Print(chunk.Text)
	}

	if err := stream.Err(); err != nil {
		panic(err)
	}
}
