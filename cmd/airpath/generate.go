package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/andapony/airpath/internal/engine"
)

// newGenerateCmd returns the "generate" sub-command, which parses flags,
// constructs an engine.Config, and calls engine.Run. On success it prints the
// output directory path; on failure it returns an error for Cobra to report.
//
// Flags: --scene (required), --output, --samplerate, --duration, --order,
// --tail (default true), --tail-onset.
func newGenerateCmd() *cobra.Command {
	var scenePath, outputDir string
	var sampleRate, order int
	var duration, tailOnset float64
	var tail bool

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate IR WAV files from a scene description",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := engine.Run(engine.Config{
				ScenePath:       scenePath,
				OutputDir:       outputDir,
				SampleRate:      sampleRate,
				Duration:        duration,
				ReflectionOrder: order,
				TailEnabled:     tail,
				TailOnset:       tailOnset,
			}); err != nil {
				return err
			}
			fmt.Printf("Done. Output written to %s\n", outputDir)
			return nil
		},
	}
	cmd.Flags().StringVar(&scenePath, "scene", "", "path to scene JSON file (required)")
	cmd.Flags().StringVar(&outputDir, "output", "./output", "output directory")
	cmd.Flags().IntVar(&sampleRate, "samplerate", 0, "sample rate override in Hz (default: from scene file)")
	cmd.Flags().Float64Var(&duration, "duration", 1.0, "IR duration in seconds")
	cmd.Flags().IntVar(&order, "order", 4, "maximum reflection order (0 = direct path only)")
	cmd.Flags().BoolVar(&tail, "tail", true, "append synthetic reverb tail")
	cmd.Flags().Float64Var(&tailOnset, "tail-onset", 0.08, "reverb tail onset in seconds")
	cmd.MarkFlagRequired("scene")
	return cmd
}
