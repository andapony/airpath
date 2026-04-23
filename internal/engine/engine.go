package engine

import (
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/andapony/airpath/internal/acoustics"
	"github.com/andapony/airpath/internal/output"
	"github.com/andapony/airpath/internal/scene"
)

// Config holds runtime parameters for a generate run.
type Config struct {
	ScenePath       string
	OutputDir       string
	SampleRate      int     // overrides scene sample_rate when > 0
	Duration        float64 // IR duration in seconds
	ReflectionOrder int     // maximum reflection order; 0 = direct path only
	TailEnabled     bool    // append synthetic reverb tail
	TailOnset       float64 // tail onset in seconds; 0 uses default of 0.08
}

// Run loads the scene, computes IRs for all source-mic pairs, and writes WAV
// files to OutputDir.
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

	tailOnset := cfg.TailOnset
	if tailOnset <= 0 {
		tailOnset = 0.08
	}
	tailOnsetSamples := int(tailOnset * float64(sampleRate))

	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	for _, src := range s.Sources {
		for _, mic := range s.Mics {
			contributions := []acoustics.PathContribution{acoustics.ComputeDirect(src, mic, sampleRate, s.Gobos)}
			if cfg.ReflectionOrder > 0 {
				contributions = append(contributions,
					acoustics.ComputeReflections(src, mic, s.Room, cfg.ReflectionOrder, sampleRate, s.Gobos)...)
			}
			ir := acoustics.AssembleIR(contributions, lengthSamples)

			if cfg.TailEnabled {
				ir = mixReverbTail(ir, s.Room, sampleRate, lengthSamples, tailOnsetSamples)
			}

			filename := fmt.Sprintf("%s_to_%s.wav", src.ID, mic.ID)
			if err := output.WriteWAV(filepath.Join(cfg.OutputDir, filename), ir, sampleRate); err != nil {
				return fmt.Errorf("writing %s: %w", filename, err)
			}
		}
	}

	return nil
}

// mixReverbTail generates and mixes a reverb tail into ir, scaling it to match
// the IR energy in the ±20 ms window around the tail onset.
func mixReverbTail(ir []float64, room scene.Room, sampleRate, lengthSamples, tailOnsetSamples int) []float64 {
	window := sampleRate / 50 // 20 ms
	wStart := tailOnsetSamples - window
	if wStart < 0 {
		wStart = 0
	}
	wEnd := tailOnsetSamples + window
	if wEnd > lengthSamples {
		wEnd = lengthSamples
	}

	var sumSq float64
	for _, v := range ir[wStart:wEnd] {
		sumSq += v * v
	}
	tailScale := math.Sqrt(sumSq / float64(wEnd-wStart))

	tail := acoustics.GenerateReverbTail(room, sampleRate, lengthSamples, tailOnsetSamples)
	for i, v := range tail {
		ir[i] += v * tailScale
	}
	return ir
}
