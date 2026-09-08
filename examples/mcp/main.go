// Example: registering an Agent's tools from an MCP server, via the
// separate mcp module. Requires an MCP server binary on PATH (adjust
// Command/Args below to point at a real one).
package main

import (
	"context"
	"fmt"

	veloxquant "github.com/rajveer43/veloxquant-go"
	"github.com/rajveer43/veloxquant-go/agent"
	vqmcp "github.com/rajveer43/veloxquant-go/mcp"
)

func main() {
	ctx := context.Background()

	client, err := veloxquant.NewClient(veloxquant.WithAutoDetect())
	if err != nil {
		panic(err)
	}

	source, err := vqmcp.Connect(ctx, vqmcp.ServerConfig{
		Name:      "example-server",
		Transport: vqmcp.TransportStdio,
		Command:   "my-mcp-server", // replace with a real MCP server binary
	})
	if err != nil {
		panic(err)
	}

	a := agent.New(client, "mlx-community/Qwen3-4B-4bit")
	if err := a.UseMcpServer(ctx, source); err != nil {
		panic(err)
	}
	defer a.Close(ctx)

	result, err := a.Run(ctx, "Use the tools available to you to help with my request.", agent.RunOptions{})
	if err != nil {
		panic(err)
	}

	fmt.Println(result.Text)
}
