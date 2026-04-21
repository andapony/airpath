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
