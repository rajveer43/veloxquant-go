// Example: using a local VeloxQuant runtime as the model backend in a
// langchaingo chain, via the veloxquant-go/langchain adapter.
package main

import (
	"context"
	"fmt"

	veloxquant "github.com/rajveer43/veloxquant-go"
	vqlangchain "github.com/rajveer43/veloxquant-go/langchain"
	"github.com/tmc/langchaingo/llms"
)

func main() {
	ctx := context.Background()

	client, err := veloxquant.NewClient(
		veloxquant.WithAutoDetect(),
	)
	if err != nil {
		panic(err)
	}

	model := vqlangchain.New(client, "mlx-community/Qwen3-8B-4bit")

	completion, err := llms.GenerateFromSinglePrompt(ctx, model, "Explain KV cache in simple terms.")
	if err != nil {
		panic(err)
	}

	fmt.Println(completion)
}
