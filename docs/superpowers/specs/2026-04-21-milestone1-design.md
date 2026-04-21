# Milestone 1 Design: Skeleton and Direct Path

**Date:** 2026-04-21  
**Scope:** Parse scene JSON, compute direct line-of-sight path from each source to each mic, write mono WAV files containing a single impulse at the correct delay and amplitude.

---

## Pipeline

```
scene.json → scene.Parse() → Scene → engine.Run() → []PathContribution → acoustics.AssembleIR() → []float64 → output.WriteWAV() → .wav
```

---

## Central Type

```go
// PathContribution represents a single acoustic path's contribution to an IR.
// M1 produces one per source-mic pair (direct path).
// Later milestones append more (reflections, diffraction, reverb tail).
type PathContribution struct {
    DelaySamples int
    Amplitude    float64
}
```

The IR assembler stamps each contribution into a buffer at its delay offset and never changes across milestones — only the code generating contributions grows.

```go
func AssembleIR(contributions []PathContribution, lengthSamples int) []float64 {
    ir := make([]float64, lengthSamples)
    for _, c := range contributions {
        if c.DelaySamples < lengthSamples {
            ir[c.DelaySamples] += c.Amplitude
        }
    }
    return ir
}
```

---

## Package Structure

Only packages needed for M1. `geometry/ray.go`, `geometry/plane.go` deferred to M2/M3.

```
internal/
├── scene/
│   ├── scene.go        — Scene, Room, Source, Mic, Gobo, Aim structs
│   ├── parse.go        — JSON parsing + validation (bounds, known materials, valid patterns)
│   └── materials.go    — absorption coefficient map (125–8000 Hz octave bands)
├── geometry/
│   └── vec3.go         — Vec3 with Add, Sub, Scale, Dot, Length, Normalize
├── acoustics/
│   ├── path.go         — PathContribution struct
│   ├── direct.go       — ComputeDirect(source, mic, sampleRate) → []PathContribution
│   ├── polar.go        — PolarGain(pattern, aim, sourceDir) → float64
│   └── ir.go           — AssembleIR(contributions, lengthSamples) → []float64
├── output/
│   └── wav.go          — hand-rolled mono 32-bit float WAV writer (no external dependency)
└── engine/
    └── engine.go       — orchestration: scene → all source×mic pairs → WAVs

cmd/airpath/main.go     — CLI entry point, flag parsing, generate subcommand
materials/defaults.json — absorption coefficients (embedded via go:embed)
examples/small_room.json
```

Dependency order: `geometry` → none; `scene` → none; `acoustics` → `scene`, `geometry`; `output` → none; `engine` → all. No circular dependencies.

---

## Direct Path Computation

`ComputeDirect` computes for a single source-mic pair:

1. **Distance:** `dist = |mic.pos - source.pos|` (Vec3 length, meters)

2. **Delay:** `delaySamples = round(dist / 343.0 * float64(sampleRate))`

3. **Amplitude (inverse-distance):** `1.0 / dist`, normalized to 1.0 at 1m reference.  
   Note: the dev plan specifies "inverse-square" but convolution operates in the pressure domain, where free-field attenuation follows inverse distance (`1/dist`), not inverse intensity (`1/dist²`). Inverse-distance is the physically correct model for IR amplitude.

4. **Air absorption scalar:** `math.Pow(10, -αAvg*dist/20)` where `αAvg ≈ 0.003 dB/m` (mid-band interpolation between plan's 1kHz and 4kHz values). Structurally correct placeholder — M5 replaces this with per-band FIR filtering.

5. **Polar gain:** `a + (1-a) * cos(θ)` where θ is the angle between the mic aim vector and the direction from mic to source.

   | Pattern       | a    |
   |---------------|------|
   | Omni          | 1.0  |
   | Cardioid      | 0.5  |
   | Supercardioid | 0.37 |
   | Figure-8      | 0.0  |

6. **Final amplitude:** `(1/dist) * polarGain * airAbsorptionScalar`

Edge case: if `dist < 0.001m` (source coincident with mic), clamp to 0.001 to avoid division by zero.

---

## CLI

```
airpath generate -scene scene.json -output ./output/ [-samplerate 48000] [-duration 1.0]
```

- `-duration`: IR length in seconds. Default 1.0s (~344m max path, covers any plausible room).
- `-samplerate`: output sample rate in Hz. Default 48000.
- `info`, `materials`, `validate` subcommands: stubbed, print "not yet implemented."

Output files named `{source_id}_to_{mic_id}.wav`, one per source-mic pair, written to the output directory (created if it doesn't exist).

---

## WAV Writer

Hand-rolled. No external dependency. Writes standard RIFF WAV: 44-byte header + mono 32-bit float PCM (`fmt` chunk audio format = 3, IEEE float). No metadata chunks in M1.

---

## Testing

| Package | What to test |
|---|---|
| `geometry` | Vec3 ops: dot product, normalization, length |
| `acoustics/polar` | Omni = 1.0 for all angles; cardioid = 0.5 on-axis, ≈0.0 rear-facing |
| `acoustics/direct` | Source at origin, mic at (1,0,0) facing source: delay ≈ 140 samples at 48kHz, amplitude = 1.0 pre-factors |
| `acoustics/ir` | Contributions stamped at correct offsets; out-of-bounds contributions safely skipped |
| `output/wav` | Write known buffer, parse bytes back, verify RIFF header fields and sample values |
| Integration | `engine.Run()` on `examples/small_room.json` produces expected output files |

---

## Validation Criteria (from dev plan)

- Load a direct-path IR into a DAW. Verify delay corresponds to source-mic distance at 343 m/s.
- Verify amplitude scales with distance (closer = louder).
- Verify cardioid rejection when source is behind mic (near-zero amplitude).
