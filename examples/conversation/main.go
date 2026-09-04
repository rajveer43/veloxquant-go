// Example: a multi-turn conversation that automatically tracks message
// history across calls.
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

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

	conv := client.NewConversation("mlx-community/Qwen3-8B-4bit", "You are a concise, helpful assistant.")

	fmt.Println("Type a message and press enter (Ctrl+D to quit).")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		prompt := scanner.Text()
		if prompt == "" {
			continue
		}

		resp, err := conv.Send(ctx, prompt)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			continue
		}

		fmt.Println(resp.Text)
	}
}
