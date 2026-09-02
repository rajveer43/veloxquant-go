// Example: a single chat completion request against a local VeloxQuant
// runtime.
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

	response, err := client.Chat(ctx, veloxquant.ChatRequest{
		Model: "mlx-community/Qwen3-8B-4bit",
		Messages: []veloxquant.Message{
			{
				Role:    "user",
				Content: "Explain KV cache in simple terms.",
			},
		},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(response.Text)
}
