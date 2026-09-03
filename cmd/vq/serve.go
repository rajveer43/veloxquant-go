package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	veloxquant "github.com/rajveer43/veloxquant-go"
	"github.com/rajveer43/veloxquant-go/runtime"
)

// runServe either launches a local VeloxQuant runtime process for --model,
// or (with no --model) connects to one already running and reports its
// status.
func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	model := fs.String("model", "", "model to serve; launches a local runtime process for it")
	method := fs.String("method", "", "KV-cache compression method (default: the runtime's own default)")
	host := fs.String("host", "", "listen address for a launched runtime (default: 127.0.0.1)")
	port := fs.Int("port", 0, "listen port for a launched runtime (default: 8000)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *model == "" {
		return connectToRunningRuntime()
	}
	return launchRuntime(*model, *method, *host, *port)
}

func connectToRunningRuntime() error {
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

func launchRuntime(model, method, host string, port int) error {
	fmt.Printf("Starting VeloxQuant runtime for %s...\n", model)

	startCtx, cancelStart := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelStart()

	proc, err := runtime.StartProcess(startCtx, runtime.ProcessConfig{
		Model:  model,
		Method: method,
		Host:   host,
		Port:   port,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if err != nil {
		return fmt.Errorf("start veloxquant runtime: %w", err)
	}

	fmt.Printf("VeloxQuant runtime ready at %s\n", proc.URL())
	fmt.Println("Press Ctrl+C to stop.")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigCh:
		fmt.Println("\nStopping VeloxQuant runtime...")
		stopCtx, cancelStop := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancelStop()
		return proc.Stop(stopCtx)
	case <-proc.Done():
		return fmt.Errorf("veloxquant runtime exited: %w", proc.Wait())
	}
}
