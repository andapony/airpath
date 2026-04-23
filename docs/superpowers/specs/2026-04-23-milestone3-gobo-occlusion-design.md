# Milestone 3: Gobo Occlusion and Diffraction — Design Spec

**Date:** 2026-04-23
**Milestone:** M3 of the airpath development plan
**Status:** Approved

---

## Overview

Gobos block or attenuate acoustic paths that intersect them. M3 adds:

1. Ray-gobo segment intersection testing
2. Maekawa barrier diffraction attenuation (mid-band scalar)
3. Mirrored-gobo technique for source-side occlusion on 1st-order reflections

No changes to `PathContribution`, `AssembleIR`, or the output layer. Spectral shaping (per-band diffraction) is deferred to M5 alongside FIR filtering.

---

## Design Decisions

### Frequency treatment: mid-band scalar

The Maekawa formula produces a frequency-dependent attenuation. Rather than extending `PathContribution` to carry per-band gains (which would partially pull forward M5 work), we use the same mid-band scalar approximation as M2 surface absorption: Maekawa N is computed at 1 kHz, and the resulting scalar is applied to the single-amplitude `PathContribution`.

### Diffracted path: over-the-top only

The diffraction point D is placed on the top edge of the gobo, directly above where the blocked path would cross the gobo's footprint. Paths around the vertical end edges are not modelled.

> **TODO:** Add end-edge diffraction. For each blocked path, compute the shortest of three candidate diffracted routes: over the top edge, around the left end, around the right end. The minimum δ wins. This gives more accurate attenuation for paths that graze the ends of a narrow or short gobo.

### Gobos are acoustically opaque

The Maekawa formula models the diffracted path only. Transmission through the gobo material (finite transmission loss) is not modelled.

> **TODO:** Add material-based transmission loss. Real gobos have finite TL (typically 15–30 dB for plywood or rockwool, worse at low frequencies). To implement:
> 1. Add a `TL [7]float64` table to the material library alongside `Absorption`.
> 2. Per blocked path, compute both Maekawa attenuation and material TL per octave band.
> 3. Combine as incoherent energy sum: `sqrt(10^(-TL/10) + Maekawa_linear²)` converted to amplitude scalar.
> This is only meaningful once `PathContribution` carries per-band gains (M5).

---

## Architecture

### Files changed

| File | Change |
|---|---|
| `internal/acoustics/diffraction.go` | **New.** Gobo intersection geometry, Maekawa math, mirroring helpers |
| `internal/acoustics/direct.go` | `ComputeDirect` accepts `[]scene.Gobo` |
| `internal/acoustics/image_source.go` | `ComputeReflections` accepts `[]scene.Gobo`; builds per-image effective gobo list |
| `internal/engine/engine.go` | Passes `s.Gobos` to both compute functions |
| `internal/acoustics/diffraction_test.go` | **New.** Unit tests for intersection, Maekawa, mirroring |
| `examples/small_room.json` | Add a gobo to exercise the new code path end-to-end |

No changes to `PathContribution`, `AssembleIR`, `output/`, or the CLI.

---

## Gobo Intersection Geometry

A gobo is a vertical rectangle. Footprint: `(x1,y1)` to `(x2,y2)` on the floor. Vertical extent: `z ∈ [0, height]`.

**Intersection test for segment A→B:**

1. Compute footprint direction `d = (x2-x1, y2-y1, 0)` and horizontal normal `n = (y2-y1, -(x2-x1), 0)`.
2. Parameterize the segment: `Q(t) = A + t*(B-A)`, `t ∈ [0,1]`.
3. Solve `n·(Q(t) - P1) = 0` for `t` (where `P1 = (x1,y1,0)`). If `n·(B-A) ≈ 0`, the segment is parallel — no hit. If `t ∉ [0,1]`, the intersection is outside the segment.
4. At intersection point Q: project onto footprint direction `s = d·(Q-P1) / |d|²`. If `s ∉ [0,1]`, Q is outside the panel's horizontal extent.
5. Check `Q.z ∈ [0, height]`.

All five conditions must pass for an intersection.

**Diffraction point D** (used to compute δ): the point on the top edge directly above the footprint crossing:

```
D = (1-s)*(x1, y1, height) + s*(x2, y2, height)
```

This is a standard first-order diffraction approximation (the edge point minimizing diffracted path length lies near the geometric shadow boundary).

---

## Maekawa Diffraction

**Path length difference:**

```
δ = |A→D| + |D→B| - |A→B|
```

δ is the extra distance the diffracted wave travels compared to the blocked direct path. It is always ≥ 0.

**Fresnel number:**

```
N = 2δ / λ    where λ = c / f
```

At 1 kHz (mid-band): `λ = 343 / 1000 = 0.343 m`.

**Maekawa attenuation:**

```
attenuation_dB ≈ 10 * log10(3 + 20*N)
```

Representative values:
- N = 0 (grazing, δ = 0): ~4.8 dB
- N = 1 (modest detour): ~13 dB
- N = 10 (significant detour): ~23 dB

**Amplitude scalar:**

```
scalar = 10^(-attenuation_dB / 20)
```

**Multiple gobos:** attenuations are cumulative — scalars multiply.

**References:**

- Maekawa, Z. (1968). Noise reduction by screens. *Applied Acoustics*, 1(3), 157–173. (Original empirical formula.)
- Kuttruff, H. (2009). *Room Acoustics* (5th ed.), §4.4. Spon Press. (Derivation and worked examples.)
- Cowan, J.P. (1999). *Architectural Acoustics Design Guide*, §6.3. McGraw-Hill. (Practical application.)

The formula is an empirical fit to the exact diffraction integral for a thin rigid half-plane, accurate to within ~2 dB for N > 0. It is standard practice for barrier insertion loss estimation.

---

## Mirrored Gobos for Reflected Paths

The image-source method replaces a reflected path (source → reflection point → mic) with a straight line from an image source to the mic. Testing this segment against in-room gobos covers only the **mic-side leg** (reflection point → mic).

To also cover the **source-side leg** (source → reflection point), we mirror each gobo across the same wall used to create the image source. The mirrored gobo lands outside the room in image space, and the image-source-to-mic segment intercepts it exactly where the real reflected path would have intercepted the real gobo.

**When mirroring applies:** only for 1st-order reflections (`sum(wallHits) == 1`). The hit wall is unambiguous for these images.

**Mirroring formulas** (applied to both `(x1,y1)` and `(x2,y2)` of the footprint; height is preserved):

| Wall | Mirror |
|---|---|
| West (x = 0) | x′ = −x |
| East (x = W) | x′ = 2W − x |
| South (y = 0) | y′ = −y |
| North (y = D) | y′ = 2D − y |
| Floor / Ceiling | not applicable — `mirrorGoboAcrossWall` returns `ok=false` |

**Limitations and TODOs:**

> **Limitation:** For 2nd-order and higher reflections, only the mic-side leg is tested for gobo occlusion. The source-side and intermediate legs are not covered.
>
> **TODO:** To support full gobo occlusion at all reflection orders, change `imageSource.wallHits` from a `[6]int` count array to a `[]int` ordered wall sequence. With the sequence available: (1) reconstruct each intermediate reflection point by intersecting the image-source-to-mic line with each wall in order; (2) test each sub-segment against appropriately mirrored gobos for that leg. This is a non-trivial refactor of `enumerateImageSources`.
>
> **TODO (lower effort):** For 2nd-order reflections off the same wall twice (e.g. east→east), mirroring the gobo twice across the same axis is well-defined and does not require ordered sequence tracking. The double-mirror places the gobo at an additional offset of `2*(W - gobo_x)` further out. This covers flutter-echo geometry and would be a worthwhile incremental improvement.

---

## Function Signatures

### `internal/acoustics/diffraction.go`

```go
// Exported
func DiffractionScalar(a, b geometry.Vec3, gobos []scene.Gobo) float64

// Unexported
func goboIntersects(a, b geometry.Vec3, g scene.Gobo) (hit bool, s float64)
func diffractionScalarSingle(a, b geometry.Vec3, g scene.Gobo) float64
func mirrorGoboAcrossWall(g scene.Gobo, wall int, room scene.Room) (scene.Gobo, bool)
func effectiveGobos(img imageSource, gobos []scene.Gobo, room scene.Room) []scene.Gobo
// Note: effectiveGobos takes imageSource (defined in image_source.go) directly
// since both files are in the same acoustics package — no export needed.
```

### Updated signatures

```go
// direct.go
func ComputeDirect(src scene.Source, mic scene.Mic, sampleRate int, gobos []scene.Gobo) PathContribution

// image_source.go
func ComputeReflections(src scene.Source, mic scene.Mic, room scene.Room, maxOrder, sampleRate int, gobos []scene.Gobo) []PathContribution
```

### Engine call sites

```go
acoustics.ComputeDirect(src, mic, sampleRate, s.Gobos)
acoustics.ComputeReflections(src, mic, s.Room, cfg.ReflectionOrder, sampleRate, s.Gobos)
```

---

## Testing

### `internal/acoustics/diffraction_test.go`

| Test | Verifies |
|---|---|
| `TestGoboIntersects_blocked` | Segment through gobo → `hit=true` |
| `TestGoboIntersects_overTop` | Segment above gobo height → `hit=false` |
| `TestGoboIntersects_parallel` | Segment parallel to gobo plane → `hit=false` |
| `TestGoboIntersects_pastEnd` | Segment crossing plane outside horizontal extent → `hit=false` |
| `TestDiffractionScalarNone` | No gobos → 1.0 |
| `TestDiffractionScalarBlocked` | Gobo in path → < 1.0 |
| `TestDiffractionScalarMultiple` | Two gobos in path → less than single-gobo case |
| `TestMirrorGoboEastWall` | Gobo at x ∈ [2,3], room width 10 → mirrored to x ∈ [17,18] |
| `TestMirrorGoboFloorWall` | Floor/ceiling → `ok=false` |
| `TestEffectiveGobosFirstOrder` | 1st-order image → original + mirrored gobo |
| `TestEffectiveGobosHigherOrder` | 2nd-order image → original gobos only |

### Existing tests

`direct_test.go` and `image_source_test.go` call sites updated to pass `[]scene.Gobo{}` — existing behaviour unchanged.

### Integration

`engine_test.go`: add a scene with a gobo between a source and mic; assert amplitude is lower than the same scene without the gobo.

### Example scene

`examples/small_room.json`: add a gobo between the guitar source and the room mic.
