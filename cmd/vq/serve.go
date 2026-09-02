package main

import (
	"context"
	"fmt"
	"time"

	veloxquant "github.com/rajveer43/veloxquant-go"
)

// runServe connects to an existing VeloxQuant runtime and reports its
// status. Launching a local runtime process directly is planned for a
// future release (see runtime.Process).
func runServe(args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := veloxquant.NewClient()
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	status, err := client.Runtime.Health(ctx)
	if err != nil {
		return fmt.Errorf("connect to veloxquant runtime: %w", err)
	}

	if !status.Healthy {
		return fmt.Errorf("veloxquant runtime reported unhealthy status")
	}

	fmt.Printf("Connected to VeloxQuant runtime (engine: %s, version: %s)\n", status.Engine, status.Version)
	return nil
}
