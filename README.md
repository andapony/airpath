# airpath

A Go command-line tool that computes impulse response (IR) sets for simulated acoustic spaces. Given a room description — geometry, surface materials, source positions, microphone positions/orientations, and gobos — it generates an N×M matrix of WAV files representing the acoustic path from each source to each microphone.

The primary use case is loading these IRs into a DAW for offline convolution against dry multitrack recordings, simulating realistic live-room bleed on overdubbed tracks. It is also a proof-of-concept for the acoustic modeling that will power a VST3 plugin.

## Usage

```bash
# Generate IRs from a scene description
airpath generate -scene scene.json -output ./output/ [-order 4] [-samplerate 48000] [-tail]

# Print room analysis (RT60, modes, path counts)
airpath info -scene scene.json

# List available materials and absorption coefficients
airpath materials

# Validate a scene file without generating output
airpath validate -scene scene.json
```

Output files are named `{source_id}_to_{mic_id}.wav`, one per source-mic pair, plus a `report.txt` summary.

## Scene File Format

Scenes are described in JSON. Spatial units are meters, angles are degrees.

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
    { "id": "vocal",  "x": 5.0, "y": 3.0, "z": 1.5 }
  ],
  "mics": [
    {
      "id": "kick_close",
      "x": 3.0, "y": 1.3, "z": 0.3,
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

## Materials

Built-in absorption coefficients (per octave band: 125, 250, 500, 1k, 2k, 4k, 8k Hz):

`concrete`, `brick`, `drywall`, `plaster`, `glass_window`, `wood_panel`, `plywood`, `acoustic_tile`, `acoustic_foam`, `heavy_curtain`, `carpet`, `hardwood_floor`, `linoleum`

Run `airpath materials` for coefficient values.

## How It Works

### Impulse Responses

An *impulse response* (IR) captures everything a room does to sound. Imagine firing a perfectly instantaneous click from a speaker — the IR is what a microphone would record: the direct sound arriving first, then discrete early echoes from nearby walls, then a increasingly dense cloud of reflections as sound bounces around the room, and finally a diffuse reverb tail that decays gradually to silence. Load that recording into a convolution reverb plugin and you can apply the room's acoustic character to any dry signal.

airpath builds an IR by identifying every significant acoustic path from a sound source to a microphone and summing their contributions. Each path is described by two numbers:

- **Delay** — how long the sound takes to travel that path, converted to a sample offset. Distance ÷ 343 m/s × sample rate gives the exact arrival time in samples.
- **Amplitude** — how loud the sound is when it arrives, after accounting for distance, air absorption, surface bounces, and the microphone's directional sensitivity.

The final IR buffer is assembled by stamping each path's amplitude at its arrival sample. Multiple paths can arrive at the same sample and their contributions accumulate, just as real sound waves superimpose.

### The Direct Path

The simplest path is the straight line from source to microphone with no bounces. Its amplitude follows the *inverse distance law*: pressure halves every time distance doubles, so amplitude = 1/distance (normalized to 1 at 1 metre). High-frequency energy is also absorbed by the air itself — a subtle rolloff that grows with distance and is more pronounced at 4 kHz and above.

### Reflections: The Image-Source Method

The key insight behind the image-source method is that a reflection off a flat wall sounds exactly like a second source placed behind that wall — a mirror image of the original. From the microphone's perspective, there is no difference between "sound that bounced off the east wall" and "sound that came directly from a virtual copy of the source positioned symmetrically on the other side of the east wall."

For a rectangular room this extends beautifully into three dimensions. Tiling all of space with mirror-image copies of the room, each containing a reflected copy of the source, produces an infinite 3D lattice of *image sources*. Each image source represents a unique sequence of wall bounces. Computing the straight-line distance from any image source to the microphone gives the travel distance for that reflection path — no ray tracing required.

#### The Lattice

Each image source is identified by an integer triple (p, q, r), where each coordinate counts reflections along one axis:

- `p` = reflections across the west/east walls (x-axis)
- `q` = reflections across the south/north walls (y-axis)
- `r` = reflections across the floor/ceiling (z-axis)

The triple (0, 0, 0) is the real source. The *reflection order* of an image source is |p| + |q| + |r| — the total number of wall bounces the path takes. Order 1 produces 6 image sources (one per wall face). Order 4 produces 258 image sources in total.

The position of image source (p, q, r) is computed analytically from the room dimensions and the source's position. For a room of width W with source at x-coordinate s:

```
x_image(p) = p × W + s          if p is even
x_image(p) = p × W + (W − s)    if p is odd
```

The same formula applies independently to each axis. There is no iteration, no ray marching — just arithmetic.

The number of bounces off each wall is also read directly from the indices. For index p along the x-axis, the east wall (positive side) is hit `⌈|p|/2⌉` times when p > 0 and `⌊|p|/2⌋` times when p < 0, with west and east swapped for negative p. This hit count is what drives the surface absorption calculation.

### Surface Properties

Every wall in the scene has a *material* that determines how much energy it absorbs at each bounce. The material library stores absorption coefficients at seven octave bands (125 Hz to 8 kHz). A coefficient of 0 means the surface is a perfect reflector; 1 means it absorbs everything.

For each image source, airpath multiplies the path's amplitude by a *surface absorption scalar*: the product of `(1 − α)^n` for each wall, where α is the absorption coefficient and n is the number of times that wall was hit. A single bounce off concrete (α ≈ 0.02) retains 98% of amplitude. A single bounce off acoustic foam (α ≈ 0.90) retains only 10%. A 4-bounce path through an absorbent room can lose the majority of its energy.

This is why material choice dramatically changes the character of the resulting IR. Compare two extremes:

- **Bare concrete room** — low absorption on every surface, reflections arrive with high amplitude and take many bounces to decay. The IR shows a dense, bright cluster of early reflections.
- **Heavily treated room** — high absorption, reflections lose energy fast. The IR shows only a few faint early reflections before falling to silence.

The current implementation uses a mid-band (1 kHz) absorption value as a single scalar per path. Milestone 5 will replace this with per-octave-band FIR filtering for accurate spectral shaping at each reflection.

### Microphone Polar Patterns

Real microphones are not equally sensitive in all directions. A close-mic pointed at a source rejects sounds arriving from behind; a room mic may be omnidirectional. airpath models this with the standard *cardioid family* formula:

```
gain(θ) = a + (1 − a) × cos(θ)
```

where θ is the angle between the microphone's aim direction and the direction the sound arrives from. The pattern coefficient `a` selects the shape:

| Pattern       | a    | On-axis (θ=0°) | Side (θ=90°) | Rear (θ=180°) |
|---------------|------|----------------|--------------|---------------|
| Omni          | 1.0  | 1.0            | 1.0          | 1.0           |
| Cardioid      | 0.5  | 1.0            | 0.5          | 0.0           |
| Supercardioid | 0.37 | 1.0            | 0.37         | −0.26 → 0.0  |
| Figure-8      | 0.0  | 1.0            | 0.0          | −1.0 → 0.0   |

(Negative values are clamped to zero — no phase inversion in this model.)

Omni is the simplest: `cos(θ)` cancels out and sensitivity is the same in every direction. Cardioid is the most common studio pattern — it picks up well at the front, attenuates the sides by 6 dB, and rejects the rear completely. Supercardioid has a tighter front lobe but a small rear lobe. Figure-8 captures front and rear equally while rejecting the sides — typical of ribbon microphones.

The aim direction is set in the scene file as azimuth (0° = north/+Y, 90° = east/+X, clockwise) and elevation (0° = horizontal, positive = upward). airpath converts this to a unit vector and computes cos(θ) via the dot product with the direction of each incoming path.

## Algorithms

**Mic polar pattern:** `gain(θ) = a + (1 - a) * cos(θ)`
| Pattern       | a    |
|---------------|------|
| Omni          | 1.0  |
| Cardioid      | 0.5  |
| Supercardioid | 0.37 |
| Figure-8      | 0.0  |

**Reflections:** Image-source method up to configurable order (default: 4).

**Gobo attenuation:** Maekawa barrier approximation, applied per frequency band.

**Late reverb:** Sabine RT60 — `RT60 = 0.161 * V / A` — shapes a noise-based exponential decay tail.

**Air absorption:** `exp(-α(f) * d)` with α ≈ 0.001 dB/m at 1kHz, 0.01 dB/m at 4kHz, 0.03 dB/m at 8kHz.

## Building

```bash
go build ./cmd/airpath
```

Requires Go 1.21+. No CGo, no external C libraries.

## Project Structure

```
airpath/
├── cmd/airpath/         # CLI entry point
├── internal/
│   ├── scene/           # Scene structs, JSON parsing, material library
│   ├── geometry/        # 3D vector math, ray, plane
│   ├── acoustics/       # Image-source, polar patterns, absorption, diffraction, reverb tail, IR assembly
│   ├── output/          # WAV writer, text/JSON report, SVG floor plan
│   └── engine/          # Top-level orchestration
├── materials/           # Default absorption coefficient data
└── examples/            # Example scene files
```
