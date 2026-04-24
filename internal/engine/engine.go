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

// Config holds all runtime parameters for a single generate run.
// Zero values are intentionally meaningful: TailEnabled=false preserves pre-M4
// behaviour (no reverb tail added), TailOnset=0 triggers the internal default of
// 80 ms, and SampleRate=0 defers to the sample_rate field in the scene file.
type Config struct {
	ScenePath       string
	OutputDir       string
	SampleRate      int     // overrides scene sample_rate when > 0
	Duration        float64 // IR duration in seconds
	ReflectionOrder int     // maximum reflection order; 0 = direct path only
	TailEnabled     bool    // append synthetic reverb tail
	TailOnset       float64 // tail onset in seconds; 0 uses default of 0.08
}

// Run loads the scene from cfg.ScenePath, computes IRs for every source-mic pair,
// and writes one WAV file per pair to cfg.OutputDir. Each IR is assembled from
// the direct path and, when cfg.ReflectionOrder > 0, specular reflections up to
// that order. When cfg.TailEnabled is true, a Sabine-based synthetic reverb tail
// is mixed in starting at cfg.TailOnset seconds (default 80 ms).
//
// Output files are named "<source_id>_to_<mic_id>.wav". The output directory is
// created if it does not already exist.
//
// Returns an error if the scene cannot be parsed, the duration is non-positive,
// the output directory cannot be created, or any WAV write fails.
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

// mixReverbTail generates a synthetic reverb tail for room and mixes it into ir
// in-place, returning the modified buffer. The tail amplitude is scaled to match
// the RMS of ir in a ±20 ms window centred on tailOnsetSamples, so the tail
// energy blends smoothly into the late-reverb portion of the IR.
//
// Window clamping: when tailOnsetSamples is near the start or end of the buffer,
// the window is clamped to [0, lengthSamples). If the window contains no energy
// (zero RMS), tailScale is zero and no tail is mixed in — acoustically correct
// for source-mic pairs where no reflections reach the onset point.
//
// Assumption: ir and the generated tail have the same length (lengthSamples).
// The tail is added sample-by-sample; samples before tailOnsetSamples are zero
// by construction and do not alter the early part of the IR.
func mixReverbTail(ir []float64, room scene.Room, sampleRate, lengthSamples, tailOnsetSamples int) []float64 {
	window := sampleRate / 50 // ±20 ms window for RMS measurement
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
