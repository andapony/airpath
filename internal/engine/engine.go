package engine

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/andapony/airpath/internal/acoustics"
	"github.com/andapony/airpath/internal/output"
	"github.com/andapony/airpath/internal/scene"
)

// Config holds runtime parameters for a generate run.
type Config struct {
	ScenePath  string
	OutputDir  string
	SampleRate int     // overrides scene sample_rate when > 0
	Duration   float64 // IR duration in seconds
}

// Run loads the scene, computes direct-path IRs for all source-mic pairs,
// and writes WAV files to OutputDir.
func Run(cfg Config) error {
	s, err := scene.Parse(cfg.ScenePath)
	if err != nil {
		return fmt.Errorf("loading scene: %w", err)
	}

	sampleRate := s.SampleRate
	if cfg.SampleRate > 0 {
		sampleRate = cfg.SampleRate
	}

	if cfg.Duration <= 0 {
		return fmt.Errorf("duration must be positive, got %v seconds", cfg.Duration)
	}

	lengthSamples := int(cfg.Duration * float64(sampleRate))

	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	for _, src := range s.Sources {
		for _, mic := range s.Mics {
			contrib := acoustics.ComputeDirect(src, mic, sampleRate)
			ir := acoustics.AssembleIR([]acoustics.PathContribution{contrib}, lengthSamples)

			filename := fmt.Sprintf("%s_to_%s.wav", src.ID, mic.ID)
			if err := output.WriteWAV(filepath.Join(cfg.OutputDir, filename), ir, sampleRate); err != nil {
				return fmt.Errorf("writing %s: %w", filename, err)
			}
		}
	}

	return nil
}
