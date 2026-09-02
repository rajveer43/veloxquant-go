// Command vq is the VeloxQuant CLI: hardware diagnostics, model memory
// analysis, optimization recommendations, benchmarking, and a local
// runtime bridge.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "doctor":
		err = runDoctor(args)
	case "analyze":
		err = runAnalyze(args)
	case "recommend":
		err = runRecommend(args)
	case "benchmark":
		err = runBenchmark(args)
	case "serve":
		err = runServe(args)
	case "-h", "--help", "help":
		printUsage()
		return
	default:
		fmt.Fprintf(os.Stderr, "vq: unknown command %q\n\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "vq: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`vq - VeloxQuant CLI

Usage:
  vq doctor              Check system readiness for local AI
  vq analyze <model>     Analyze memory requirements for a model
  vq recommend           Recommend models and a profile for this hardware
  vq benchmark <model>   Benchmark inference performance for a model
  vq serve               Connect to (or report on) the VeloxQuant runtime

Flags:
  -h, --help             Show this help message`)
}
