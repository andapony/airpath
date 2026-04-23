# Milestone 4: Late Reverb Tail — Design Spec

**Date:** 2026-04-23
**Milestone:** M4 of the airpath development plan
**Status:** Approved

---

## Overview

M4 appends a synthetic diffuse reverb tail to each IR, derived from the room's Sabine RT60. It also replaces the stdlib `flag`-based CLI with Cobra, which handles the existing subcommand structure cleanly and makes adding future flags straightforward.

No changes to `PathContribution`, `AssembleIR`, `diffraction.go`, or the output layer.

---

## Design Decisions

### Frequency treatment: single RT60 with one-pole HF rolloff

A per-octave-band RT60 (running Sabine with per-band absorption) would be physically more accurate but overlaps with M5's frequency-domain filtering work. Instead, M4 uses a single Sabine RT60 computed at the 1 kHz mid-band — consistent with the absorption scalar approximation used throughout the project — plus a one-pole IIR lowpass (~3 kHz cutoff) applied to the tail noise to simulate faster HF decay.

> **TODO (M5):** Replace the one-pole HF rolloff with per-octave-band RT60 values derived from per-band Sabine absorption. This will require either 7 noise-decay channels combined through octave-band filters, or a frequency-domain shaping approach consistent with the FIR filtering added in M5.

### Tail onset and crossfade

The tail begins at a configurable onset time (default 80 ms), where image-source reflections are dense enough to approximate a diffuse field. A 20 ms raised-cosine fade-in is applied at the onset to avoid clicks. No fade-out of the early reflection region is applied — the image-source contributions are discrete spikes and there is no energy overlap to crossfade.

### Per-pair amplitude scaling

The late reverb tail is position-independent in the diffuse field model — the same room gives the same tail shape for every source-mic pair. Amplitude varies per pair by matching the tail RMS at onset to the RMS of the assembled IR in the ±20 ms window around the onset sample. This ensures smooth blending: pairs with strong late reflections (close sources, reflective rooms) get a stronger tail; lightly absorbed rooms with sparse late reflections get a quieter tail. If the IR is silent at onset (e.g., `--order 0`), the tail is inaudible — correct, since without reflections there is no energy to feed the diffuse field.

### Reverb tail is per source-mic pair

`GenerateReverbTail` is called once per pair. All calls use a fixed noise seed, so the tail shape is identical across pairs; only the amplitude scale (Section: Per-pair amplitude scaling) differs. The decay shape (RT60, HF rolloff) derives only from the room.

### Cobra CLI

The existing `flag`-based CLI is replaced with Cobra. The subcommand structure (`generate`, `info`, `materials`, `validate`) already exists in intent; Cobra makes it explicit and adds `--help` per subcommand. The `generate` subcommand gains `--tail` (default `true`) and `--tail-onset` (default `0.08`) flags.

---

## Architecture

### Files changed

| File | Change |
|---|---|
| `internal/acoustics/reverb_tail.go` | **New.** `SabineRT60`, `GenerateReverbTail` |
| `internal/acoustics/reverb_tail_test.go` | **New.** Unit tests for RT60 and tail generation |
| `internal/engine/engine.go` | Add tail mixing step; `Config` gains `TailEnabled bool`, `TailOnset float64` |
| `internal/engine/engine_test.go` | Add tail-on vs tail-off test |
| `cmd/airpath/main.go` | Rewrite as Cobra root command |
| `cmd/airpath/generate.go` | **New.** `generate` subcommand with all flags |
| `go.mod` / `go.sum` | Add `github.com/spf13/cobra` |

---

## Reverb Tail Algorithm

### `SabineRT60(room scene.Room) float64`

```
V = Width × Depth × Height
A = (Width×Depth) × (α_floor + α_ceiling)
  + (Width×Height) × (α_north + α_south)
  + (Depth×Height) × (α_east + α_west)
RT60 = 0.161 × V / A
```

Absorption coefficients are looked up at the 1 kHz band (index 3 in the 7-band array) via `scene.KnownMaterials`.

### `GenerateReverbTail(room scene.Room, sampleRate, lengthSamples, tailOnsetSamples int) []float64`

Returns a `[]float64` of length `lengthSamples`. Samples `[0, tailOnsetSamples)` are zero. The tail occupies `[tailOnsetSamples, lengthSamples)` and is constructed in four steps:

**1. Noise**

Generate Gaussian white noise for the tail region using Box-Muller transform on `math/rand`. Use a fixed seed (`1`) so output is deterministic across runs — reproducibility matters more than inter-pair independence for a PoC diffuse tail.

**2. Exponential decay envelope**

At sample offset `n` from onset (0-indexed):

```
envelope(n) = exp(−ln(1000) × n / (RT60 × sampleRate))
```

This reaches exactly −60 dB at `n = RT60 × sampleRate`. Multiply each noise sample by its envelope value.

**3. One-pole HF rolloff**

Apply a leaky integrator in-place:

```
α = 1 − exp(−2π × 3000 / sampleRate)
y[n] = α × x[n] + (1−α) × y[n−1]
```

Cutoff ≈ 3 kHz. This suppresses high-frequency energy, simulating faster HF decay characteristic of real rooms.

**4. Cosine fade-in and normalize**

Fade-in over the first 20 ms of the tail (`fadeInSamples = sampleRate / 50`):

```
w(n) = 0.5 × (1 − cos(π × n / fadeInSamples))    for n < fadeInSamples
w(n) = 1.0                                          for n ≥ fadeInSamples
```

Then normalize so the RMS of the first 20 ms of the (faded, filtered) tail is 1.0.

---

## Engine Integration

### `Config` changes

```go
type Config struct {
    ScenePath       string
    OutputDir       string
    SampleRate      int
    Duration        float64
    ReflectionOrder int
    TailEnabled     bool    // default false (zero value); set true by --tail flag
    TailOnset       float64 // seconds; default 0.08
}
```

### Per-pair loop

```
contributions = ComputeDirect + ComputeReflections
ir = AssembleIR(contributions, lengthSamples)
if cfg.TailEnabled:
    tailOnsetSamples = int(cfg.TailOnset × sampleRate)
    windowStart = max(0, tailOnsetSamples − sampleRate/50)
    windowEnd   = min(lengthSamples, tailOnsetSamples + sampleRate/50)
    tailScale   = RMS(ir[windowStart:windowEnd])
    tail        = GenerateReverbTail(s.Room, sampleRate, lengthSamples, tailOnsetSamples)
    for i, v := range tail:
        ir[i] += v × tailScale
write ir to WAV
```

---

## Cobra CLI

### `cmd/airpath/main.go`

```go
func main() {
    root := &cobra.Command{Use: "airpath"}
    root.AddCommand(newGenerateCmd())
    root.AddCommand(newInfoCmd())
    root.AddCommand(newMaterialsCmd())
    root.AddCommand(newValidateCmd())
    if err := root.Execute(); err != nil {
        os.Exit(1)
    }
}
```

### `cmd/airpath/generate.go`

```go
func newGenerateCmd() *cobra.Command {
    var scenePath, outputDir string
    var sampleRate, order int
    var duration, tailOnset float64
    var tail bool

    cmd := &cobra.Command{
        Use:   "generate",
        Short: "Generate IR WAV files from a scene description",
        RunE: func(cmd *cobra.Command, args []string) error {
            return engine.Run(engine.Config{
                ScenePath:       scenePath,
                OutputDir:       outputDir,
                SampleRate:      sampleRate,
                Duration:        duration,
                ReflectionOrder: order,
                TailEnabled:     tail,
                TailOnset:       tailOnset,
            })
        },
    }
    cmd.Flags().StringVar(&scenePath, "scene", "", "path to scene JSON file (required)")
    cmd.Flags().StringVar(&outputDir, "output", "./output", "output directory")
    cmd.Flags().IntVar(&sampleRate, "samplerate", 0, "sample rate override in Hz (default: from scene file)")
    cmd.Flags().Float64Var(&duration, "duration", 1.0, "IR duration in seconds")
    cmd.Flags().IntVar(&order, "order", 4, "maximum reflection order (0 = direct path only)")
    cmd.Flags().BoolVar(&tail, "tail", true, "append synthetic reverb tail")
    cmd.Flags().Float64Var(&tailOnset, "tail-onset", 0.08, "reverb tail onset in seconds (default 80ms)")
    cmd.MarkFlagRequired("scene")
    return cmd
}
```

The `info`, `materials`, and `validate` subcommands are stubs returning `"not yet implemented"`.

---

## Function Signatures

```go
// internal/acoustics/reverb_tail.go

func SabineRT60(room scene.Room) float64

func GenerateReverbTail(room scene.Room, sampleRate, lengthSamples, tailOnsetSamples int) []float64
```

---

## Testing

### `internal/acoustics/reverb_tail_test.go`

| Test | Verifies |
|---|---|
| `TestSabineRT60_KnownRoom` | 5×4×2.8m room with known materials → RT60 within 5% of hand-calculated value |
| `TestSabineRT60_HighAbsorption` | Heavily treated room gives shorter RT60 than lightly treated room |
| `TestGenerateReverbTail_Length` | Output length equals `lengthSamples`; all samples before `tailOnsetSamples` are exactly zero |
| `TestGenerateReverbTail_OnsetRMS` | RMS of first 20 ms of tail ≈ 1.0 (within 10%) |
| `TestGenerateReverbTail_Decays` | RMS of tail in a 20 ms window at t=RT60 is ≤ −58 dB relative to onset RMS |

### `internal/engine/engine_test.go`

| Test | Verifies |
|---|---|
| `TestRunSmallRoom_TailChangesOutput` | `TailEnabled: true` vs `false` with `ReflectionOrder: 1` → `guitar_to_room.wav` differs |
| Existing tests | All pass unchanged — `TailEnabled` zero value is `false`, preserving prior behaviour |
