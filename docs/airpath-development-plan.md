# airpath — Development Plan

## Overview

`airpath` is a Go command-line tool that computes impulse response (IR) sets for simulated acoustic spaces. Given a room description (geometry, surface materials, source positions, microphone positions/orientations, gobos), it generates an N×M matrix of WAV files representing the acoustic path from each source to each microphone. These IRs can then be loaded into a DAW for offline convolution against real multitrack sessions.

The purpose is to explore and validate the acoustic modeling that will eventually power a VST3 plugin for simulating live-room bleed on overdubbed recordings.

## Architecture

The tool has three layers:

1. **Scene description** — A JSON file defining the room, surfaces, sources, mics, and gobos.
2. **Acoustic engine** — Computes the N×M IR matrix using the image-source method, surface absorption, mic polar patterns, and gobo occlusion/diffraction.
3. **Output** — Writes labeled WAV files (one per source-mic pair) plus a summary report.

```
scene.json → [parser] → [acoustic engine] → [WAV writer] → output/
```

## Scene Description Format

The scene file is JSON. All spatial units are meters, angles are degrees, frequencies are Hz.

```json
{
  "version": 1,
  "sample_rate": 48000,
  "room": {
    "width": 8.0,
    "depth": 6.0,
    "height": 3.5,
    "surfaces": {
      "floor":   "concrete",
      "ceiling": "acoustic_tile",
      "north":   "drywall",
      "south":   "drywall",
      "east":    "brick",
      "west":    "glass_window"
    }
  },
  "sources": [
    { "id": "kick",   "x": 3.0, "y": 1.0, "z": 0.3 },
    { "id": "vocal",  "x": 5.0, "y": 3.0, "z": 1.5 },
    { "id": "guitar", "x": 1.5, "y": 4.0, "z": 1.0 }
  ],
  "mics": [
    {
      "id": "kick_close",
      "x": 3.0, "y": 1.3, "z": 0.3,
      "aim": { "azimuth": 180, "elevation": 0 },
      "pattern": "cardioid"
    },
    {
      "id": "vocal_close",
      "x": 5.0, "y": 2.7, "z": 1.5,
      "aim": { "azimuth": 180, "elevation": 0 },
      "pattern": "cardioid"
    },
    {
      "id": "room_L",
      "x": 4.0, "y": 5.5, "z": 2.0,
      "aim": { "azimuth": 270, "elevation": -10 },
      "pattern": "omni"
    }
  ],
  "gobos": [
    {
      "id": "drum_iso",
      "x1": 2.0, "y1": 2.0,
      "x2": 4.0, "y2": 2.0,
      "height": 2.0,
      "material": "plywood"
    }
  ]
}
```

## Material Library

A built-in library of absorption coefficients per octave band (125, 250, 500, 1000, 2000, 4000, 8000 Hz). Stored as an embedded Go map or a bundled JSON file. Initial set:

- concrete, brick, drywall, plaster
- glass_window, wood_panel, plywood
- acoustic_tile, acoustic_foam, heavy_curtain
- carpet, hardwood_floor, linoleum

Values sourced from published acoustic engineering tables.

## Development Milestones

### Milestone 1: Skeleton and Direct Path

**Goal:** Parse the scene file, compute the direct (line-of-sight) path from each source to each mic, and write WAV files containing a single impulse at the correct delay and amplitude.

Tasks:
- Define Go structs for scene, room, source, mic, gobo, material.
- JSON parser with validation (bounds checking, known materials, etc.).
- 3D geometry primitives: point, vector, ray, line segment, plane.
- Direct path computation: distance → delay (samples), inverse-square attenuation, air absorption (simple high-frequency rolloff proportional to distance).
- Mic polar pattern gain: implement `gain(θ) = a + (1-a) * cos(θ)` with pattern parameter lookup (omni=1.0, cardioid=0.5, supercardioid=0.37, figure8=0.0). Compute angle between mic aim vector and source direction.
- IR buffer: allocate per source-mic pair, write impulse at computed delay with computed gain.
- WAV writer: output mono WAV files, named `{source_id}_to_{mic_id}.wav`.
- CLI: `airpath generate -scene scene.json -output ./output/`

**Validation:** Load a single direct-path IR into a DAW. Verify delay corresponds to source-mic distance at speed of sound (343 m/s). Verify amplitude scales with distance. Verify cardioid rejection when source is behind mic.

### Milestone 2: Image-Source Reflections

**Goal:** Add specular reflections up to configurable order using the image-source method, with surface absorption filtering.

Tasks:
- Image-source generation: for a rectangular room with 6 surfaces, recursively mirror source positions across each wall. Support configurable maximum reflection order (default: 4).
- For each image source, trace the reflection path back to identify which surfaces were hit and in what order.
- Surface absorption: for each bounce, apply the surface material's frequency-dependent absorption coefficients. Implement as a per-octave-band gain applied to the impulse. (Initially this can be a simple multiply per band; later it can be a proper FIR filter per impulse.)
- Accumulate all image-source contributions into the IR buffer for each source-mic pair.
- CLI flag: `-order N` to set maximum reflection order.

**Validation:** Visualize the IR in a DAW — should see a cluster of early reflections following the direct impulse, with decreasing amplitude and increasing density. Compare a bare concrete room (strong reflections, long decay) vs. a heavily treated room (weak reflections, fast decay). Verify that the first reflection arrives at the correct time for the room geometry.

### Milestone 3: Gobo Occlusion and Diffraction

**Goal:** Gobos block or attenuate acoustic paths that intersect them.

Tasks:
- Ray-gobo intersection: for each path segment (direct or reflected), test whether it intersects any gobo geometry. Gobos are vertical rectangular panels defined by two floor-plan endpoints and a height.
- If a path intersects a gobo, compute the frequency-dependent diffraction attenuation using the Maekawa barrier approximation. This requires computing the path-length difference between the direct (occluded) path and the shortest diffracted path over/around the gobo.
- Apply the diffraction attenuation to the impulse for that path. Low frequencies pass with mild attenuation; high frequencies are strongly attenuated.
- Multiple gobos: a single path may intersect more than one gobo; attenuations are cumulative.

**Validation:** Place a gobo between a source and mic. Verify that the direct path is attenuated (especially at high frequencies) but not eliminated. Compare the IR with and without the gobo — the gobo version should sound darker and more distant.

### Milestone 4: Late Reverb Tail

**Goal:** Append a synthetic diffuse reverb tail to each IR, matching the room's acoustic properties.

Tasks:
- Compute the room's RT60 (reverberation time) from dimensions and average absorption using the Sabine equation: `RT60 = 0.161 * V / A` where V is room volume and A is total absorption area.
- Generate a noise-based exponential decay shaped by the RT60. Apply frequency-dependent decay rates (high frequencies decay faster than low frequencies in most rooms).
- The tail onset should begin where the image-source reflections become dense enough to approximate a diffuse field (configurable, default ~80ms).
- The tail can be largely shared across source-mic pairs (diffuse field is position-independent), with per-pair scaling based on overall room coupling.
- Crossfade smoothly between the early reflection region and the synthetic tail.

**Validation:** The resulting full IR should sound like a plausible room reverb when convolved with dry audio. Compare RT60 of the output IR (measure in a DAW or analysis tool) against the Sabine prediction.

### Milestone 5: Frequency-Domain Filtering

**Goal:** Replace the per-octave-band gain approximation with proper filtering for more realistic surface absorption and air absorption.

Tasks:
- For each impulse in the IR (each direct/reflected path contribution), generate a short FIR filter representing the cumulative absorption of all surface bounces and air absorption along that path.
- Apply the filter to the impulse, replacing the single-sample impulse with a filtered impulse (a short burst).
- This produces more realistic spectral shaping — the IR will sound less like a series of clicks and more like a natural acoustic response.
- Consider using FFT-based filtering for efficiency if the number of paths is large.

### Milestone 6: Summary Report and Visualization

**Goal:** Output a human-readable report and optional visualization data alongside the WAV files.

Tasks:
- Text report: for each source-mic pair, list direct path distance/delay/gain, number of reflections computed, estimated bleed level relative to direct signal.
- Room summary: computed RT60, room modes (standing wave frequencies), total absorption.
- Optional JSON output of all computed paths (source, reflection sequence, arrival time, gain, angle) for external visualization tools.
- Optional SVG floor-plan rendering showing source positions, mic positions with pattern indicators, gobo positions, and direct paths. This is useful for verifying scene layout.

## CLI Interface

```
airpath generate -scene scene.json -output ./output/ [-order 4] [-samplerate 48000] [-tail true]
airpath info -scene scene.json    # print room analysis (RT60, modes, path counts)
airpath materials                  # list available materials and their coefficients
airpath validate -scene scene.json # check scene file for errors without generating
```

## Project Structure

```
airpath/
├── cmd/
│   └── airpath/
│       └── main.go              # CLI entry point
├── internal/
│   ├── scene/
│   │   ├── scene.go             # Scene, Room, Source, Mic, Gobo structs
│   │   ├── parse.go             # JSON parsing and validation
│   │   └── materials.go         # Material library and absorption coefficients
│   ├── geometry/
│   │   ├── vec3.go              # 3D vector math
│   │   ├── ray.go               # Ray type and intersection tests
│   │   └── plane.go             # Plane/surface geometry
│   ├── acoustics/
│   │   ├── image_source.go      # Image-source method implementation
│   │   ├── polar.go             # Mic polar pattern computation
│   │   ├── absorption.go        # Surface absorption and air absorption
│   │   ├── diffraction.go       # Maekawa barrier diffraction model
│   │   ├── reverb_tail.go       # Synthetic late reverb generation
│   │   └── ir.go                # IR buffer management and assembly
│   ├── output/
│   │   ├── wav.go               # WAV file writer
│   │   ├── report.go            # Text/JSON summary report
│   │   └── svg.go               # Optional SVG floor plan renderer
│   └── engine/
│       └── engine.go            # Top-level orchestration: scene → IRs → output
├── materials/
│   └── defaults.json            # Default material absorption coefficients
├── examples/
│   ├── small_room.json          # Example: small tracking room
│   ├── live_room.json           # Example: large live room with gobos
│   └── drum_iso.json            # Example: drum isolation setup
├── go.mod
├── go.sum
└── README.md
```

## Key Algorithms Reference

### Mic Polar Pattern
```
gain(θ) = a + (1 - a) * cos(θ)
```
| Pattern       | a    |
|---------------|------|
| Omni          | 1.0  |
| Cardioid      | 0.5  |
| Supercardioid | 0.37 |
| Figure-8      | 0.0  |

### Sabine RT60
```
RT60 = 0.161 * V / A
```
where V = room volume (m³), A = Σ(surface_area × absorption_coefficient) (m²).

### Maekawa Barrier Approximation
```
attenuation_dB ≈ 10 * log10(3 + 20 * N)
```
where N = 2 * δ / λ, δ = path length difference (diffracted path minus direct path), λ = wavelength. Apply per frequency band.

### Air Absorption
Approximate as exponential high-frequency attenuation proportional to path length. A simple model: attenuation at frequency f over distance d is `exp(-α(f) * d)` where α increases with frequency (roughly 0.001 dB/m at 1kHz, 0.01 dB/m at 4kHz, 0.03 dB/m at 8kHz at typical room temperature and humidity).

## Example Workflow

```bash
# Generate IRs from a scene description
airpath generate -scene examples/live_room.json -output ./output/ -order 4

# Produces files like:
#   output/kick_to_kick_close.wav
#   output/kick_to_vocal_close.wav
#   output/kick_to_room_L.wav
#   output/vocal_to_kick_close.wav
#   output/vocal_to_vocal_close.wav
#   output/vocal_to_room_L.wav
#   ... (N sources × M mics)
#   output/report.txt

# Load IRs into DAW as convolution reverb inserts or aux sends.
# Convolve against dry multitrack stems and evaluate the bleed effect.
```

## Dependencies

- Standard library only for core computation (math, encoding/json, os, flag).
- WAV writing: use a lightweight WAV encoder (go-audio/wav or hand-roll — WAV is a trivial format).
- No CGo, no external C libraries. Pure Go for portability and simplicity.

## Future Extensions (Out of Scope for PoC)

These features are documented here for context but are not part of the initial proof-of-concept:

- Sympathetic resonance modeling (nonlinear, signal-dependent).
- Environmental sound sources (exterior sources, wall transmission filtering).
- Non-rectangular room geometry (arbitrary polygonal floor plans).
- 3D GUI room designer (standalone application, OpenGL/Vulkan).
- Real-time convolution engine (C++, VST3/AU plugin).
- Frequency-dependent mic polar patterns.
- Room preset sharing format and community library.
