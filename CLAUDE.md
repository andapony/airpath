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

## Commenting Requirements

Every function — including test functions — must have a doc comment that describes:
- What the function does
- Any assumptions it relies on (input preconditions, invariants)
- Any limitations it has (approximations made, known edge cases, deferred work)

All non-trivial or non-obvious code must have inline comments explaining *why*, not *what* (the code already says what; the comment explains the reasoning, the constraint, or the subtlety).

Obvious one-liners (simple getters, trivial arithmetic, standard library calls) do not need inline comments.

## Commit Message Requirements

Every commit must have a meaningful message consisting of:
1. A header line in conventional commit format: `<type>: <description>` — e.g. `feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`.
2. A blank line.
3. At least one paragraph of body text explaining the context: what problem this solves, why this approach was chosen, or any non-obvious constraints a reader would need to understand the change.

The header must be specific enough to identify the change at a glance. The body must add information that isn't already obvious from the diff — motivation, trade-offs, edge cases addressed, or deferred work. All lines (header and body) must be word-wrapped at 72 characters.

Good header: `fix: clamp tailScale to zero when onset window has no energy`
Bad header: `fix bug`, `update code`, `changes`

Good body: `The ±20 ms onset window can fall entirely between reflections in sparse IRs (e.g. direct-only at order 0). Previously the zero RMS produced a NaN tailScale via 0/0. Clamping to zero is acoustically correct — no measured energy means no tail to match.`
Bad body: `Fixed the issue.`, `Updated as requested.`

## Development Workflow

When executing implementation plans, always use `superpowers:subagent-driven-development`. Dispatch one subagent per task rather than implementing inline.

**Model selection — use the least capable model that can handle the task:**
- Mechanical implementation (isolated functions, clear spec, 1–2 files): `haiku`
- Multi-file integration or tasks requiring judgment: `sonnet`
- Architecture, design, and spec/quality review: `sonnet`

This conserves tokens without sacrificing quality on tasks that don't need it.

## Full Plan

See `docs/airpath-development-plan.md` for complete scene format spec, algorithm reference, and validation criteria per milestone.
