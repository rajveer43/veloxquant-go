package main

import (
	"context"
	"fmt"
	"time"

	veloxquant "github.com/rajveer43/veloxquant-go"
)

func runRecommend(args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := veloxquant.NewClient()
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	info, err := client.System.Info(ctx)
	if err != nil {
		return fmt.Errorf("detect system: %w", err)
	}

	fmt.Println("Hardware:")
	if info.CPUModel != "" {
		fmt.Println(info.CPUModel)
	} else {
		fmt.Printf("%s/%s\n", info.Platform, info.Architecture)
	}
	if info.TotalMemory > 0 {
		fmt.Printf("%s Unified Memory\n", veloxquant.FormatBytes(info.TotalMemory))
	}
	fmt.Println()

	recommendations, err := client.Models.Recommend(ctx, veloxquant.ModelRecommendationRequest{
		AvailableMemoryBytes: info.AvailableMemory,
	})
	if err != nil {
		return fmt.Errorf("recommend models: %w", err)
	}

	fmt.Println("Recommended Models:")
	fmt.Println()
	if len(recommendations) == 0 {
		fmt.Println("(none fit available memory)")
	}
	for i, m := range recommendations {
		fmt.Printf("%d. %s\n", i+1, m.Name)
	}
	fmt.Println()

	fmt.Printf("Recommended Profile:\n%s\n", info.RecommendedProfile)

	return nil
}
