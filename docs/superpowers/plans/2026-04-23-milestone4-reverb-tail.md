# Milestone 4: Late Reverb Tail Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Append a synthetic Sabine-derived reverb tail to each IR and migrate the CLI from stdlib `flag` to Cobra.

**Architecture:** A new `reverb_tail.go` computes RT60 (Sabine) and generates a normalized noise-decay buffer (exponential envelope + one-pole HF rolloff + cosine fade-in). The engine assembles the early IR as before, then—when `TailEnabled` is set—RMS-matches the tail to the late early-reflection energy and mixes it in. The CLI is split into `cmd/airpath/main.go` (Cobra root) and `cmd/airpath/generate.go` (subcommand), replacing the stdlib-flag switch.

**Tech Stack:** Go stdlib (`math`, `math/rand`), `github.com/spf13/cobra`, `github.com/stretchr/testify` (already present).

**Spec:** `docs/superpowers/specs/2026-04-23-milestone4-reverb-tail-design.md`

---

## File Structure

| File | Change |
|---|---|
| `internal/acoustics/reverb_tail.go` | **New.** `SabineRT60`, `GenerateReverbTail` |
| `internal/acoustics/reverb_tail_test.go` | **New.** Unit tests for both functions |
| `internal/engine/engine.go` | Add `TailEnabled`, `TailOnset` to `Config`; add tail mixing step |
| `internal/engine/engine_test.go` | Add `TestRunSmallRoom_TailChangesOutput` |
| `cmd/airpath/main.go` | Rewrite as Cobra root command |
| `cmd/airpath/generate.go` | **New.** `generate` subcommand with all flags |
| `go.mod` / `go.sum` | Add `github.com/spf13/cobra` |

---

## Task 1: SabineRT60

**Files:**
- Create: `internal/acoustics/reverb_tail_test.go`
- Create: `internal/acoustics/reverb_tail.go`

- [ ] **Step 1: Create the test file with two RT60 tests**

Create `internal/acoustics/reverb_tail_test.go`:

```go
package acoustics

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/andapony/airpath/internal/scene"
)

func TestSabineRT60_KnownRoom(t *testing.T) {
	// 5×4×2.8 m room matching examples/small_room.json
	room := scene.Room{
		Width: 5.0, Depth: 4.0, Height: 2.8,
		Surfaces: scene.Surfaces{
			Floor:   "hardwood_floor", // α_1kHz = 0.06
			Ceiling: "acoustic_tile",  // α_1kHz = 0.72
			North:   "drywall",        // α_1kHz = 0.03
			South:   "drywall",
			East:    "drywall",
			West:    "glass_window", // α_1kHz = 0.12
		},
	}
	// V = 5×4×2.8 = 56 m³
	// A = (5×4)×(0.06+0.72) + (5×2.8)×(0.03+0.03) + (4×2.8)×(0.03+0.12)
	//   = 20×0.78 + 14×0.06 + 11.2×0.15
	//   = 15.6 + 0.84 + 1.68 = 18.12 m²
	// RT60 = 0.161×56/18.12 ≈ 0.4976 s
	expected := 0.161 * 56.0 / 18.12
	got := SabineRT60(room)
	assert.InDelta(t, expected, got, 0.001)
}

func TestSabineRT60_HighAbsorption(t *testing.T) {
	room := scene.Room{Width: 4.0, Depth: 5.0, Height: 3.0}
	// acoustic_foam α_1kHz = 0.80; brick α_1kHz = 0.04
	room.Surfaces = scene.Surfaces{
		Floor: "acoustic_foam", Ceiling: "acoustic_foam",
		North: "acoustic_foam", South: "acoustic_foam",
		East: "acoustic_foam", West: "acoustic_foam",
	}
	rt60Foam := SabineRT60(room)

	room.Surfaces = scene.Surfaces{
		Floor: "brick", Ceiling: "brick",
		North: "brick", South: "brick",
		East: "brick", West: "brick",
	}
	rt60Brick := SabineRT60(room)

	assert.Greater(t, rt60Brick, rt60Foam,
		"low-absorption room should have longer RT60 than high-absorption room")
	require.Positive(t, rt60Foam)
	require.Positive(t, rt60Brick)
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/acoustics/ -run "TestSabineRT60" -v
```

Expected: compilation error — `SabineRT60` undefined.

- [ ] **Step 3: Create `reverb_tail.go` with `SabineRT60`**

Create `internal/acoustics/reverb_tail.go`:

```go
package acoustics

import (
	"github.com/andapony/airpath/internal/scene"
)

// SabineRT60 returns the estimated reverberation time (seconds) for room using
// the Sabine equation at the 1 kHz mid-band: RT60 = 0.161 * V / A.
// A = Σ(surface_area × α_1kHz) for all six surfaces.
func SabineRT60(room scene.Room) float64 {
	V := room.Width * room.Depth * room.Height

	floorCeiling := room.Width * room.Depth
	northSouth := room.Width * room.Height
	eastWest := room.Depth * room.Height

	const band1k = 3 // index of 1000 Hz in the 7-band array

	alpha := func(mat string) float64 {
		if a, ok := scene.KnownMaterials[mat]; ok {
			return a[band1k]
		}
		return 0
	}

	A := floorCeiling*(alpha(room.Surfaces.Floor)+alpha(room.Surfaces.Ceiling)) +
		northSouth*(alpha(room.Surfaces.North)+alpha(room.Surfaces.South)) +
		eastWest*(alpha(room.Surfaces.East)+alpha(room.Surfaces.West))

	if A <= 0 {
		return 0
	}
	return 0.161 * V / A
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/acoustics/ -run "TestSabineRT60" -v
```

Expected:
```
--- PASS: TestSabineRT60_KnownRoom (0.00s)
--- PASS: TestSabineRT60_HighAbsorption (0.00s)
PASS
```

- [ ] **Step 5: Run the full test suite to confirm no regressions**

```bash
go test ./...
```

Expected: all packages PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/acoustics/reverb_tail.go internal/acoustics/reverb_tail_test.go
git commit -m "feat(acoustics): add SabineRT60 for reverberation time estimation"
```

---

## Task 2: GenerateReverbTail

**Files:**
- Modify: `internal/acoustics/reverb_tail.go`
- Modify: `internal/acoustics/reverb_tail_test.go`

- [ ] **Step 1: Add three tests to `reverb_tail_test.go`**

Append to `internal/acoustics/reverb_tail_test.go`:

```go
func TestGenerateReverbTail_Length(t *testing.T) {
	room := scene.Room{
		Width: 5.0, Depth: 4.0, Height: 2.8,
		Surfaces: scene.Surfaces{
			Floor: "hardwood_floor", Ceiling: "acoustic_tile",
			North: "drywall", South: "drywall",
			East: "drywall", West: "glass_window",
		},
	}
	const sampleRate = 48000
	const length = 48000  // 1 second
	const onset = 3840    // 80 ms

	buf := GenerateReverbTail(room, sampleRate, length, onset)

	assert.Len(t, buf, length, "output length must equal lengthSamples")
	for i := 0; i < onset; i++ {
		assert.Equal(t, 0.0, buf[i], "sample %d before onset should be zero", i)
	}
}

func TestGenerateReverbTail_OnsetRMS(t *testing.T) {
	room := scene.Room{
		Width: 5.0, Depth: 4.0, Height: 2.8,
		Surfaces: scene.Surfaces{
			Floor: "hardwood_floor", Ceiling: "acoustic_tile",
			North: "drywall", South: "drywall",
			East: "drywall", West: "glass_window",
		},
	}
	const sampleRate = 48000
	const onset = 3840 // 80 ms

	buf := GenerateReverbTail(room, sampleRate, sampleRate*2, onset)

	// RMS of the first 20 ms of the tail (the fade-in window) should be ≈ 1.0.
	fadeIn := sampleRate / 50 // 960 samples = 20 ms
	var sumSq float64
	for _, v := range buf[onset : onset+fadeIn] {
		sumSq += v * v
	}
	rms := math.Sqrt(sumSq / float64(fadeIn))
	assert.InDelta(t, 1.0, rms, 0.1, "onset RMS should be approximately 1.0")
}

func TestGenerateReverbTail_Decays(t *testing.T) {
	// Use an all-plaster room: RT60 ≈ 2.57 s — long enough that the 20 ms
	// fade-in is negligible when measuring the RT60 decay ratio.
	// plaster α_1kHz = 0.04; 4×5×3 room: V=60, A=94×0.04=3.76, RT60=2.567 s.
	room := scene.Room{
		Width: 4.0, Depth: 5.0, Height: 3.0,
		Surfaces: scene.Surfaces{
			Floor: "plaster", Ceiling: "plaster",
			North: "plaster", South: "plaster",
			East: "plaster", West: "plaster",
		},
	}
	const sampleRate = 48000
	const onset = 3840 // 80 ms

	rt60 := SabineRT60(room) // ≈ 2.567 s
	length := onset + int(rt60*float64(sampleRate)) + sampleRate/50 + sampleRate
	buf := GenerateReverbTail(room, sampleRate, length, onset)

	fadeIn := sampleRate / 50 // 960 samples = 20 ms
	window := fadeIn

	// Reference: RMS of the 20 ms window just after the fade-in completes.
	refStart := onset + fadeIn
	var sumRef float64
	for _, v := range buf[refStart : refStart+window] {
		sumRef += v * v
	}
	rmsRef := math.Sqrt(sumRef / float64(window))
	require.Positive(t, rmsRef)

	// Decay window: 20 ms starting at onset + RT60.
	rt60Start := onset + int(rt60*float64(sampleRate))
	require.LessOrEqualf(t, rt60Start+window, len(buf),
		"buffer too short: need %d samples, have %d", rt60Start+window, len(buf))
	var sumDecay float64
	for _, v := range buf[rt60Start : rt60Start+window] {
		sumDecay += v * v
	}
	rmsDecay := math.Sqrt(sumDecay / float64(window))

	// At t=RT60 from onset, level should be well below the post-ramp reference.
	// The design target is −60 dB; we allow −55 dB to account for filter transients.
	threshold := rmsRef * math.Pow(10, -55.0/20.0)
	assert.LessOrEqual(t, rmsDecay, threshold,
		"RMS at t=RT60 should be ≤ −55 dB of post-ramp reference (design: −60 dB)")
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/acoustics/ -run "TestGenerateReverbTail" -v
```

Expected: compilation error — `GenerateReverbTail` undefined.

- [ ] **Step 3: Replace `reverb_tail.go` with the full file including `GenerateReverbTail`**

Replace the full contents of `internal/acoustics/reverb_tail.go` (adding `math` and `math/rand` imports alongside the new function):

```go
package acoustics

import (
	"math"
	"math/rand"

	"github.com/andapony/airpath/internal/scene"
)

// SabineRT60 returns the estimated reverberation time (seconds) for room using
// the Sabine equation at the 1 kHz mid-band: RT60 = 0.161 * V / A.
// A = Σ(surface_area × α_1kHz) for all six surfaces.
func SabineRT60(room scene.Room) float64 {
	V := room.Width * room.Depth * room.Height

	floorCeiling := room.Width * room.Depth
	northSouth := room.Width * room.Height
	eastWest := room.Depth * room.Height

	const band1k = 3

	alpha := func(mat string) float64 {
		if a, ok := scene.KnownMaterials[mat]; ok {
			return a[band1k]
		}
		return 0
	}

	A := floorCeiling*(alpha(room.Surfaces.Floor)+alpha(room.Surfaces.Ceiling)) +
		northSouth*(alpha(room.Surfaces.North)+alpha(room.Surfaces.South)) +
		eastWest*(alpha(room.Surfaces.East)+alpha(room.Surfaces.West))

	if A <= 0 {
		return 0
	}
	return 0.161 * V / A
}

// GenerateReverbTail returns a buffer of lengthSamples with a synthetic reverb
// tail beginning at tailOnsetSamples. Samples before tailOnsetSamples are zero.
//
// The tail is shaped by the room's Sabine RT60 and a one-pole HF lowpass
// (~3 kHz cutoff) to simulate faster high-frequency decay. A 20 ms raised-cosine
// fade-in prevents a click at the onset. The onset window is normalized to RMS 1.0;
// the engine scales each pair's tail by the IR energy at the onset time.
func GenerateReverbTail(room scene.Room, sampleRate, lengthSamples, tailOnsetSamples int) []float64 {
	buf := make([]float64, lengthSamples)
	if tailOnsetSamples >= lengthSamples {
		return buf
	}

	rt60 := SabineRT60(room)
	if rt60 <= 0 {
		return buf
	}

	tail := buf[tailOnsetSamples:]

	// Step 1: Gaussian white noise with fixed seed for reproducibility.
	rng := rand.New(rand.NewSource(1))
	for i := range tail {
		tail[i] = rng.NormFloat64()
	}

	// Step 2: Exponential decay envelope — reaches −60 dB at t = RT60.
	decayRate := math.Log(1000) / (rt60 * float64(sampleRate))
	for i := range tail {
		tail[i] *= math.Exp(-decayRate * float64(i))
	}

	// Step 3: One-pole IIR lowpass (~3 kHz) to simulate faster HF decay.
	alpha := 1 - math.Exp(-2*math.Pi*3000/float64(sampleRate))
	var y float64
	for i := range tail {
		y = alpha*tail[i] + (1-alpha)*y
		tail[i] = y
	}

	// Step 4: Raised-cosine fade-in over the first 20 ms to avoid a click at onset.
	fadeIn := sampleRate / 50 // 20 ms
	if fadeIn > len(tail) {
		fadeIn = len(tail)
	}
	for i := 0; i < fadeIn; i++ {
		tail[i] *= 0.5 * (1 - math.Cos(math.Pi*float64(i)/float64(fadeIn)))
	}

	// Step 5: Normalize so the RMS of the fade-in window is 1.0.
	var sumSq float64
	for _, v := range tail[:fadeIn] {
		sumSq += v * v
	}
	if rms := math.Sqrt(sumSq / float64(fadeIn)); rms > 1e-10 {
		for i := range tail {
			tail[i] /= rms
		}
	}

	return buf
}
```

- [ ] **Step 4: Run the new tests**

```bash
go test ./internal/acoustics/ -run "TestGenerateReverbTail" -v
```

Expected:
```
--- PASS: TestGenerateReverbTail_Length (0.00s)
--- PASS: TestGenerateReverbTail_OnsetRMS (0.00s)
--- PASS: TestGenerateReverbTail_Decays (0.00s)
PASS
```

- [ ] **Step 5: Run the full test suite**

```bash
go test ./...
```

Expected: all packages PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/acoustics/reverb_tail.go internal/acoustics/reverb_tail_test.go
git commit -m "feat(acoustics): add GenerateReverbTail with Sabine decay and HF rolloff"
```

---

## Task 3: Engine Integration

**Files:**
- Modify: `internal/engine/engine.go`
- Modify: `internal/engine/engine_test.go`

- [ ] **Step 1: Add the failing engine test**

Append to `internal/engine/engine_test.go`:

```go
func TestRunSmallRoom_TailChangesOutput(t *testing.T) {
	outWithTail := t.TempDir()
	require.NoError(t, Run(Config{
		ScenePath:       "../../examples/small_room.json",
		OutputDir:       outWithTail,
		Duration:        1.0,
		ReflectionOrder: 1,
		TailEnabled:     true,
		TailOnset:       0.08,
	}))

	outNoTail := t.TempDir()
	require.NoError(t, Run(Config{
		ScenePath:       "../../examples/small_room.json",
		OutputDir:       outNoTail,
		Duration:        1.0,
		ReflectionOrder: 1,
		TailEnabled:     false,
	}))

	withBytes, err := os.ReadFile(filepath.Join(outWithTail, "guitar_to_room.wav"))
	require.NoError(t, err)
	withoutBytes, err := os.ReadFile(filepath.Join(outNoTail, "guitar_to_room.wav"))
	require.NoError(t, err)
	assert.NotEqual(t, withBytes, withoutBytes,
		"guitar_to_room.wav should differ when tail is enabled")
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
go test ./internal/engine/ -run "TestRunSmallRoom_TailChangesOutput" -v
```

Expected: compilation error — `TailEnabled` and `TailOnset` undefined in `Config`.

- [ ] **Step 3: Update `engine.go`**

Replace the full contents of `internal/engine/engine.go` with:

```go
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
```

- [ ] **Step 4: Run the new engine test**

```bash
go test ./internal/engine/ -run "TestRunSmallRoom_TailChangesOutput" -v
```

Expected:
```
--- PASS: TestRunSmallRoom_TailChangesOutput (0.00s)
PASS
```

- [ ] **Step 5: Run the full test suite**

```bash
go test ./...
```

Expected: all packages PASS. Existing engine tests use `TailEnabled: false` (zero value) and are unaffected.

- [ ] **Step 6: Commit**

```bash
git add internal/engine/engine.go internal/engine/engine_test.go
git commit -m "feat(engine): add reverb tail mixing with RMS-matched onset scaling"
```

---

## Task 4: Cobra CLI

**Files:**
- Modify: `cmd/airpath/main.go`
- Create: `cmd/airpath/generate.go`
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add Cobra dependency**

```bash
go get github.com/spf13/cobra@latest
```

Expected: `go.mod` and `go.sum` updated. No error output.

- [ ] **Step 2: Verify the project still builds**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Create `cmd/airpath/generate.go`**

Create `cmd/airpath/generate.go`:

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/andapony/airpath/internal/engine"
)

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
```

- [ ] **Step 4: Rewrite `cmd/airpath/main.go`**

Replace the full contents of `cmd/airpath/main.go` with:

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "airpath",
		Short: "Generate impulse response WAV files for simulated acoustic spaces",
	}
	root.AddCommand(newGenerateCmd())
	root.AddCommand(newStubCmd("info", "Print room analysis (RT60, modes, path counts)"))
	root.AddCommand(newStubCmd("materials", "List available materials and absorption coefficients"))
	root.AddCommand(newStubCmd("validate", "Validate a scene file without generating output"))
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newStubCmd(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Run:   func(cmd *cobra.Command, args []string) { fmt.Println("not yet implemented") },
	}
}
```

- [ ] **Step 5: Build and smoke-test the CLI**

```bash
go build ./cmd/airpath && ./airpath --help
```

Expected output (exact wording may vary):
```
Generate impulse response WAV files for simulated acoustic spaces

Usage:
  airpath [command]

Available Commands:
  generate    Generate IR WAV files from a scene description
  info        Print room analysis (RT60, modes, path counts)
  materials   List available materials and absorption coefficients
  validate    Validate a scene file without generating output
  ...
```

```bash
./airpath generate --help
```

Expected: shows `--scene`, `--output`, `--samplerate`, `--duration`, `--order`, `--tail`, `--tail-onset` flags.

```bash
./airpath generate --scene examples/small_room.json --output ./output/ --tail=false --order 0
```

Expected: `Done. Output written to ./output/` with the same WAV files as before.

- [ ] **Step 6: Run the full test suite**

```bash
go test ./...
```

Expected: all packages PASS.

- [ ] **Step 7: Commit**

```bash
git add cmd/airpath/main.go cmd/airpath/generate.go go.mod go.sum
git commit -m "feat(cli): migrate to Cobra; add --tail and --tail-onset flags to generate"
```

---

## Verification

After all tasks, run the full suite and a manual generate with the tail enabled:

```bash
go test ./...
go build ./cmd/airpath
./airpath generate --scene examples/small_room.json --output ./output/ --order 4 --tail --duration 2.0
```

The output WAVs should be 2 seconds long and `guitar_to_room.wav` should contain audible reverb decay after the direct impulse and early reflections when loaded into a DAW or audio analysis tool.
