# Milestone 2: Image-Source Reflections — Design

## Overview

Extends the airpath IR generator to include specular reflections using the image-source method for rectangular rooms. Reflections are accumulated as additional `PathContribution` values alongside the existing direct path, with surface absorption applied as a scalar mid-band (1kHz) approximation. Proper frequency-domain filtering is deferred to Milestone 5.

## Scope

**In scope:**
- Image-source generation via lattice enumeration for rectangular rooms
- Surface absorption as a scalar (1kHz mid-band) per-bounce multiplier
- `-order N` CLI flag (default 4); order 0 reproduces M1 direct-path-only behaviour
- TODO comment for future path-length culling

**Out of scope:**
- Per-octave-band amplitude tracking (M5)
- Path-length culling (future optimisation)
- Non-rectangular geometry
- Gobo occlusion of reflection paths (M3)

## Architecture

One new file; three modified files. No changes to `PathContribution`, `AssembleIR`, `output/`, or `scene/`.

```
internal/acoustics/image_source.go   NEW — image-source logic
internal/engine/engine.go            add ReflectionOrder to Config; call ComputeReflections
cmd/airpath/main.go                  add -order flag
internal/acoustics/path.go           add TODO comment
```

The IR pipeline is additive: M1's direct contribution is unchanged; M2 appends reflection contributions to the same `[]PathContribution` slice before calling `AssembleIR`.

## Room Coordinate Convention

The room spans `[0, W] × [0, D] × [0, H]` where W = `room.Width`, D = `room.Depth`, H = `room.Height`.

Wall mapping:

| Wall    | Plane  | `scene.Surfaces` field |
|---------|--------|------------------------|
| West    | x = 0  | `West`                 |
| East    | x = W  | `East`                 |
| South   | y = 0  | `South`                |
| North   | y = D  | `North`                |
| Floor   | z = 0  | `Floor`                |
| Ceiling | z = H  | `Ceiling`              |

## Image Source Generation

### Lattice Enumeration

Enumerate all integer triples `(p, q, r)` where `1 ≤ |p|+|q|+|r| ≤ maxOrder`. For each triple, compute the image source position using the closed-form per-axis formula:

```
imageCoord(n, L, s):
    if n is even: n*L + s
    if n is odd:  n*L + (L - s)
```

where `n` is the axis index (p, q, or r), `L` is the room dimension, and `s` is the source coordinate on that axis. In Go, `n%2 == 0` correctly identifies even indices for both positive and negative n.

### Surface Hit Counts

For axis index `n` (with `|n|` total reflections on that axis):

```
positive-wall hits = ceil(|n|/2)  if n > 0,  else floor(|n|/2)
negative-wall hits = floor(|n|/2) if n > 0,  else ceil(|n|/2)
```

Applied per axis:
- x: east = positive-wall, west = negative-wall
- y: north = positive-wall, south = negative-wall
- z: ceiling = positive-wall, floor = negative-wall

### Internal Type

```go
type imageSource struct {
    pos      geometry.Vec3
    wallHits [6]int  // west, east, south, north, floor, ceiling
}
```

Unexported — implementation detail of `image_source.go`.

## Contribution Computation

Public function:

```go
func ComputeReflections(
    src scene.Source,
    mic scene.Mic,
    room scene.Room,
    maxOrder int,
    sampleRate int,
) []PathContribution
```

For each image source → mic pair, the `PathContribution` is computed as:

```
delay        = distance / speedOfSound * sampleRate   (same as ComputeDirect)
amplitude    = (1/distance) × airScalar × polarGain × absorptionScalar
airScalar    = exp(-airAbsorptionDBPerM × distance / 20)  (same as ComputeDirect)
polarGain    = PolarGain(mic, imageSource.pos)             (same as ComputeDirect)
absorptionScalar = ∏ (1 - α_1kHz[wall])^wallHits[wall]  over all 6 walls
```

`α_1kHz` is band index 3 in `scene.Bands` (`[7]int{125, 250, 500, 1000, 2000, 4000, 8000}`). This is the same mid-band approximation used by `airAbsorptionDBPerM` in `direct.go`; M5 replaces both with proper per-band FIR filtering.

Walls with unknown materials use absorption 0.0 (perfect reflector). The scene validator already rejects unknown materials, so this path is unreachable in normal operation.

## Engine Integration

`engine.Config` gains one field:

```go
type Config struct {
    ScenePath       string
    OutputDir       string
    SampleRate      int
    Duration        float64
    ReflectionOrder int  // 0 = direct path only
}
```

`Run` assembles contributions per source-mic pair:

```go
contributions := []PathContribution{ComputeDirect(src, mic, sampleRate)}
if cfg.ReflectionOrder > 0 {
    contributions = append(contributions,
        ComputeReflections(src, mic, s.Room, cfg.ReflectionOrder, sampleRate)...)
}
ir := AssembleIR(contributions, lengthSamples)
```

## CLI

New flag in `cmd/airpath/main.go`:

```
-order int   maximum reflection order (default 4, 0 = direct path only)
```

`-order 0` reproduces M1 output exactly.

## Testing

### Unit tests — `internal/acoustics/image_source_test.go`

| Test | What it checks |
|------|---------------|
| `imageCoord` values | 10m room, source at 3m: n=1→17m, n=-1→-3m, n=2→23m |
| Wall hit counts | p=3: east=2, west=1; p=-2: east=1, west=1 |
| First-order delay | Image (1,0,0) arrives at correct sample delay for known geometry |
| Perfect absorber | All surfaces α=1.0 → absorptionScalar=0, amplitude=0 |
| Perfect reflector | All surfaces α=0.0 → absorptionScalar=1, amplitude = 1/distance only |
| Empty at order 0 | `ComputeReflections(..., 0, ...)` returns empty slice |

### Integration tests — `internal/engine/engine_test.go`

| Test | What it checks |
|------|---------------|
| Order 0 regression | `ReflectionOrder=0` output matches M1 direct-path-only WAV |
| Order 1 additive | `ReflectionOrder=1` produces more contributions than order 0 for same pair |

## Validation (from dev plan)

Load a reflection IR into a DAW. Verify:
- Early reflections visible as impulse cluster following the direct arrival
- Bare concrete room has stronger, longer-decaying reflections than a heavily treated room
- First reflection arrives at the correct time for the room geometry (verifiable by hand)
