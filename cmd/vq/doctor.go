package main

import (
	"context"
	"fmt"
	"time"

	veloxquant "github.com/rajveer43/veloxquant-go"
)

func runDoctor(args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Println("VeloxQuant Doctor")
	fmt.Println()

	fmt.Println("✓ Go runtime available")

	client, err := veloxquant.NewClient()
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	info, err := client.System.Info(ctx)
	if err != nil {
		fmt.Printf("✗ System detection failed: %v\n", err)
	} else {
		if info.AppleSilicon {
			fmt.Println("✓ Apple Silicon detected")
		} else {
			fmt.Printf("- Apple Silicon not detected (platform: %s/%s)\n", info.Platform, info.Architecture)
		}

		if info.TotalMemory > 0 {
			fmt.Printf("✓ %s Unified Memory\n", veloxquant.FormatBytes(info.TotalMemory))
		} else {
			fmt.Println("- Could not determine total memory")
		}
	}

	status, err := client.Runtime.Health(ctx)
	if err != nil || !status.Healthy {
		fmt.Println("✗ VeloxQuant runtime unreachable")
	} else {
		fmt.Println("✓ VeloxQuant runtime reachable")
	}

	fmt.Println()
	if err == nil && status.Healthy {
		fmt.Println("System is ready.")
	} else {
		fmt.Println("System is ready for local memory analysis; start the VeloxQuant runtime for inference.")
	}

	return nil
}
