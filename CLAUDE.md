# airpath

Go CLI tool that computes impulse response (IR) WAV files for simulated acoustic spaces. Proof-of-concept for acoustic modeling that will eventually power a VST3 live-room bleed plugin.

## Build & Run

```bash
go build ./cmd/airpath
go test ./...
./airpath generate --scene examples/small_room.json --output ./output/
```

## Architecture

Three-layer pipeline: `scene.json → [parser] → [acoustic engine] → [WAV writer] → output/`

- `internal/scene/` — structs, JSON parsing, validation, material library
- `internal/geometry/` — 3D vector math (vec3, ray, plane)
- `internal/acoustics/` — image-source method, polar patterns, absorption, diffraction, reverb tail, IR buffers
- `internal/output/` — WAV writer, text/JSON report, SVG floor plan
- `internal/engine/` — top-level orchestration
- `cmd/airpath/` — CLI (flag parsing, subcommands)
- `materials/defaults.json` — embedded absorption coefficient data

## Key Constraints

- Pure Go, stdlib only (plus one lightweight WAV library). No CGo.
- All spatial units in meters, angles in degrees, frequencies in Hz.
- Speed of sound: 343 m/s.
- Octave bands: 125, 250, 500, 1000, 2000, 4000, 8000 Hz.

## Development Milestones

1. **Skeleton + direct path** — parse scene, compute direct LOS impulses, write WAVs ← *start here*
2. **Image-source reflections** — specular reflections up to configurable order with surface absorption
3. **Gobo occlusion** — ray-gobo intersection + Maekawa diffraction attenuation
4. **Late reverb tail** — Sabine RT60-based synthetic diffuse tail
5. **Frequency-domain filtering** — proper FIR filtering replacing per-band gain approximation
6. **Report + visualization** — text report, JSON path data, SVG floor plan

## Algorithms

**Polar pattern:** `gain(θ) = a + (1-a)*cos(θ)` — omni a=1.0, cardioid a=0.5, supercardioid a=0.37, figure-8 a=0.0

**Sabine RT60:** `RT60 = 0.161 * V / A` where V = room volume (m³), A = Σ(surface_area × absorption_coeff)

**Maekawa diffraction:** `attenuation_dB ≈ 10 * log10(3 + 20*N)` where N = 2δ/λ, δ = path length difference

**Air absorption:** `exp(-α(f) * d)` — α ≈ 0.001/m at 1kHz, 0.01/m at 4kHz, 0.03/m at 8kHz

## Development Workflow

When executing implementation plans, always use `superpowers:subagent-driven-development`. Dispatch one subagent per task rather than implementing inline.

**Model selection — use the least capable model that can handle the task:**
- Mechanical implementation (isolated functions, clear spec, 1–2 files): `haiku`
- Multi-file integration or tasks requiring judgment: `sonnet`
- Architecture, design, and spec/quality review: `sonnet`

This conserves tokens without sacrificing quality on tasks that don't need it.

## Full Plan

See `docs/airpath-development-plan.md` for complete scene format spec, algorithm reference, and validation criteria per milestone.
