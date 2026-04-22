package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/andapony/airpath/internal/engine"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: airpath <command> [flags]")
		fmt.Fprintln(os.Stderr, "Commands: generate, info, materials, validate")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "generate":
		runGenerate(os.Args[2:])
	case "info", "materials", "validate":
		fmt.Println("not yet implemented")
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runGenerate(args []string) {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	scenePath := fs.String("scene", "", "path to scene JSON file (required)")
	outputDir := fs.String("output", "./output", "output directory")
	sampleRate := fs.Int("samplerate", 0, "sample rate override in Hz (default: from scene file)")
	duration := fs.Float64("duration", 1.0, "IR duration in seconds")
	fs.Parse(args)

	if *scenePath == "" {
		fmt.Fprintln(os.Stderr, "error: -scene is required")
		os.Exit(1)
	}

	if err := engine.Run(engine.Config{
		ScenePath:  *scenePath,
		OutputDir:  *outputDir,
		SampleRate: *sampleRate,
		Duration:   *duration,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Done. Output written to %s\n", *outputDir)
}
