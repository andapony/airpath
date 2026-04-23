# Milestone 3: Gobo Occlusion and Diffraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Gobos block acoustic paths that intersect them, applying Maekawa barrier diffraction attenuation (mid-band scalar) to both direct and reflected paths.

**Architecture:** A new `diffraction.go` in the `acoustics` package provides gobo-segment intersection geometry, Maekawa attenuation math, and mirrored-gobo helpers. `ComputeDirect` and `ComputeReflections` accept `[]scene.Gobo` and multiply each path contribution by the resulting scalar. For 1st-order reflections, gobos are also mirrored across the hit wall to cover the source-side leg of the path.

**Tech Stack:** Go stdlib (`math`). `github.com/stretchr/testify/assert` for tests (already used throughout). No new dependencies.

**Spec:** `docs/superpowers/specs/2026-04-23-milestone3-gobo-occlusion-design.md`

---

## File Structure

| File | Change |
|---|---|
| `internal/acoustics/diffraction.go` | **New.** Intersection geometry, Maekawa math, mirroring helpers |
| `internal/acoustics/diffraction_test.go` | **New.** All tests for the above |
| `internal/acoustics/direct.go` | Add `gobos []scene.Gobo` param; call `DiffractionScalar` |
| `internal/acoustics/image_source.go` | Add `gobos []scene.Gobo` param; call `effectiveGobos` + `DiffractionScalar` |
| `internal/engine/engine.go` | Pass `s.Gobos` to both compute functions |
| `internal/engine/engine_test.go` | Add gobo integration test |
| `examples/small_room.json` | Add a gobo between guitar source and room mic |

---

## Task 1: goboIntersects — segment-rectangle intersection

**Files:**
- Create: `internal/acoustics/diffraction_test.go`
- Create: `internal/acoustics/diffraction.go`

- [ ] **Step 1: Create `diffraction_test.go` with four intersection tests**

```go
package acoustics

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/andapony/airpath/internal/geometry"
	"github.com/andapony/airpath/internal/scene"
)

// testGobo runs along y=2 from x=1 to x=3, height 2 m. Reused across tasks.
var testGobo = scene.Gobo{X1: 1.0, Y1: 2.0, X2: 3.0, Y2: 2.0, Height: 2.0, Material: "plywood"}

func TestGoboIntersects_blocked(t *testing.T) {
	// Segment passes through the gobo at x=2, y=2, z=1 (midpoint of panel).
	a := geometry.Vec3{X: 2.0, Y: 1.0, Z: 1.0}
	b := geometry.Vec3{X: 2.0, Y: 3.0, Z: 1.0}
	hit, s := goboIntersects(a, b, testGobo)
	assert.True(t, hit)
	assert.InDelta(t, 0.5, s, 1e-9)
}

func TestGoboIntersects_overTop(t *testing.T) {
	// Segment passes at z=2.5, above the gobo's height of 2.0.
	a := geometry.Vec3{X: 2.0, Y: 1.0, Z: 2.5}
	b := geometry.Vec3{X: 2.0, Y: 3.0, Z: 2.5}
	hit, _ := goboIntersects(a, b, testGobo)
	assert.False(t, hit, "path above gobo height should not intersect")
}

func TestGoboIntersects_parallel(t *testing.T) {
	// Segment runs along y=1, parallel to the gobo plane (normal is in Y).
	a := geometry.Vec3{X: 1.0, Y: 1.0, Z: 1.0}
	b := geometry.Vec3{X: 3.0, Y: 1.0, Z: 1.0}
	hit, _ := goboIntersects(a, b, testGobo)
	assert.False(t, hit, "segment parallel to gobo plane should not intersect")
}

func TestGoboIntersects_pastEnd(t *testing.T) {
	// Segment crosses the y=2 plane at x=0.5, outside the gobo's x range [1,3].
	a := geometry.Vec3{X: 0.5, Y: 1.0, Z: 1.0}
	b := geometry.Vec3{X: 0.5, Y: 3.0, Z: 1.0}
	hit, _ := goboIntersects(a, b, testGobo)
	assert.False(t, hit, "crossing point outside horizontal extent should not intersect")
}
```

- [ ] **Step 2: Run tests to verify they fail (compile error — goboIntersects not defined)**

```bash
go test ./internal/acoustics/ -run TestGoboIntersects -v
```

Expected: compile error `undefined: goboIntersects`

- [ ] **Step 3: Create `diffraction.go` with `goboIntersects`**

```go
package acoustics

// Maekawa barrier diffraction: gobo-segment intersection geometry and
// frequency-independent amplitude scalar computation.
//
// References:
//   Maekawa, Z. (1968). Noise reduction by screens. Applied Acoustics, 1(3), 157–173.
//   Kuttruff, H. (2009). Room Acoustics (5th ed.), §4.4. Spon Press.
//   Cowan, J.P. (1999). Architectural Acoustics Design Guide, §6.3. McGraw-Hill.

import (
	"math"

	"github.com/andapony/airpath/internal/geometry"
	"github.com/andapony/airpath/internal/scene"
)

// goboIntersects tests whether segment A→B passes through the gobo panel.
// Returns hit=true and footprint parameter s ∈ [0,1] when it does.
// s locates the crossing point along the base edge and is used to place
// the diffraction point on the top edge.
//
// Algorithm:
//  1. Compute the horizontal normal n to the footprint direction d.
//  2. Find where the segment crosses the gobo's plane: t = n·(P1-A) / n·(B-A).
//  3. Check t ∈ [0,1] (within segment), Q.Z ∈ [0,height] (vertical bounds),
//     and s ∈ [0,1] (horizontal bounds along the footprint).
func goboIntersects(a, b geometry.Vec3, g scene.Gobo) (hit bool, s float64) {
	dx := g.X2 - g.X1
	dy := g.Y2 - g.Y1
	// Horizontal normal to the footprint direction: n = (dy, -dx, 0).
	nx, ny := dy, -dx

	denom := nx*(b.X-a.X) + ny*(b.Y-a.Y)
	if math.Abs(denom) < 1e-12 {
		return false, 0 // segment parallel to gobo plane
	}
	t := (nx*(g.X1-a.X) + ny*(g.Y1-a.Y)) / denom
	if t < 0 || t > 1 {
		return false, 0 // intersection outside segment
	}
	q := geometry.Vec3{
		X: a.X + t*(b.X-a.X),
		Y: a.Y + t*(b.Y-a.Y),
		Z: a.Z + t*(b.Z-a.Z),
	}
	if q.Z < 0 || q.Z > g.Height {
		return false, 0 // outside vertical extent
	}
	dLen2 := dx*dx + dy*dy
	if dLen2 < 1e-12 {
		return false, 0 // degenerate (zero-length) gobo
	}
	s = ((q.X-g.X1)*dx + (q.Y-g.Y1)*dy) / dLen2
	if s < 0 || s > 1 {
		return false, 0 // outside horizontal extent
	}
	return true, s
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/acoustics/ -run TestGoboIntersects -v
```

Expected:
```
--- PASS: TestGoboIntersects_blocked (0.00s)
--- PASS: TestGoboIntersects_overTop (0.00s)
--- PASS: TestGoboIntersects_parallel (0.00s)
--- PASS: TestGoboIntersects_pastEnd (0.00s)
PASS
```

- [ ] **Step 5: Commit**

```bash
git add internal/acoustics/diffraction.go internal/acoustics/diffraction_test.go
git commit -m "$(cat <<'EOF'
feat(acoustics): add gobo-segment intersection geometry
EOF
)"
```

---

## Task 2: Maekawa diffraction scalar

**Files:**
- Modify: `internal/acoustics/diffraction_test.go`
- Modify: `internal/acoustics/diffraction.go`

- [ ] **Step 1: Add three scalar tests to `diffraction_test.go`**

Append these functions after the existing tests:

```go
func TestDiffractionScalarNone(t *testing.T) {
	a := geometry.Vec3{X: 0, Y: 0, Z: 1}
	b := geometry.Vec3{X: 5, Y: 5, Z: 1}
	assert.Equal(t, 1.0, DiffractionScalar(a, b, nil))
}

func TestDiffractionScalarBlocked(t *testing.T) {
	// testGobo blocks the path from (2,1,1) to (2,3,1).
	a := geometry.Vec3{X: 2.0, Y: 1.0, Z: 1.0}
	b := geometry.Vec3{X: 2.0, Y: 3.0, Z: 1.0}
	got := DiffractionScalar(a, b, []scene.Gobo{testGobo})
	assert.Greater(t, got, 0.0, "scalar must be positive")
	assert.Less(t, got, 1.0, "blocked path must be attenuated")
}

func TestDiffractionScalarMultiple(t *testing.T) {
	// Two gobos in the path: one at y=2 (testGobo) and one at y=3.
	gobo2 := scene.Gobo{X1: 1.0, Y1: 3.0, X2: 3.0, Y2: 3.0, Height: 2.0, Material: "plywood"}
	a := geometry.Vec3{X: 2.0, Y: 0.0, Z: 1.0}
	b := geometry.Vec3{X: 2.0, Y: 5.0, Z: 1.0}
	one := DiffractionScalar(a, b, []scene.Gobo{testGobo})
	two := DiffractionScalar(a, b, []scene.Gobo{testGobo, gobo2})
	assert.Less(t, two, one, "two blocking gobos must attenuate more than one")
}
```

- [ ] **Step 2: Run tests to verify they fail (compile error — DiffractionScalar not defined)**

```bash
go test ./internal/acoustics/ -run TestDiffractionScalar -v
```

Expected: compile error `undefined: DiffractionScalar`

- [ ] **Step 3: Add `diffractionScalarSingle` and `DiffractionScalar` to `diffraction.go`**

Append after `goboIntersects`:

```go
// diffractionScalarSingle returns the Maekawa amplitude scalar [0,1] for a
// single gobo blocking segment A→B. Returns 1.0 if the gobo does not
// block the path.
//
// The diffraction point D is placed on the top edge of the gobo directly
// above the footprint crossing (standard first-order diffraction approximation).
// The Fresnel number N is evaluated at 1 kHz — a mid-band scalar that defers
// full per-octave-band treatment to M5 alongside per-band FIR filtering.
//
// Maekawa formula: attenuation_dB ≈ 10·log10(3 + 20·N), N = 2δ/λ
// where δ = |A→D| + |D→B| - |A→B| (extra path length of the diffracted route)
// and λ = c/f is the wavelength at 1 kHz.
//
// Characteristic values: N=0 (grazing) → 4.8 dB; N=1 → 13 dB; N=10 → 23 dB.
//
// References:
//   Maekawa (1968) Applied Acoustics 1(3):157–173 (original empirical formula).
//   Kuttruff, Room Acoustics 5th ed. §4.4 (derivation and worked examples).
//   Cowan, Architectural Acoustics Design Guide §6.3 (practical application).
// Accurate to ~2 dB for N > 0; standard for barrier insertion-loss estimation.
//
// TODO: Only over-the-top diffraction is modelled (D on the top edge). For
// better accuracy on paths that graze the vertical end edges, compute δ for
// the left end vertex, the right end vertex, and the top edge, then take the
// minimum across all three. This covers narrow gobos and low-angle paths.
//
// TODO: Gobos are currently treated as acoustically opaque (transmission
// loss = ∞). Real gobos have finite TL (typically 15–30 dB for plywood or
// rockwool, worse at low frequencies). To add material transparency:
//   1. Add a TL [7]float64 table to the material library alongside Absorption.
//   2. Per blocked path, compute Maekawa attenuation and TL per octave band.
//   3. Combine as incoherent energy sum: sqrt(10^(-TL/10) + Maekawa_linear²),
//      converted to an amplitude scalar.
// Deferred until PathContribution carries per-band gains (M5).
func diffractionScalarSingle(a, b geometry.Vec3, g scene.Gobo) float64 {
	hit, s := goboIntersects(a, b, g)
	if !hit {
		return 1.0
	}
	// Diffraction point D: top edge at footprint parameter s.
	d := geometry.Vec3{
		X: g.X1 + s*(g.X2-g.X1),
		Y: g.Y1 + s*(g.Y2-g.Y1),
		Z: g.Height,
	}
	// Path length difference δ = detour over the top edge vs. direct blocked path.
	delta := d.Sub(a).Length() + b.Sub(d).Length() - b.Sub(a).Length()
	if delta < 0 {
		delta = 0 // guard against floating-point underflow
	}
	lambda := speedOfSound / 1000.0 // wavelength at 1 kHz (mid-band)
	N := 2 * delta / lambda
	attenDB := 10 * math.Log10(3+20*N)
	return math.Pow(10, -attenDB/20)
}

// DiffractionScalar returns the cumulative Maekawa amplitude attenuation [0,1]
// for segment A→B due to all intersecting gobos. Returns 1.0 when gobos is
// nil or empty, or when no gobo blocks the path.
func DiffractionScalar(a, b geometry.Vec3, gobos []scene.Gobo) float64 {
	scalar := 1.0
	for _, g := range gobos {
		scalar *= diffractionScalarSingle(a, b, g)
	}
	return scalar
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/acoustics/ -run 'TestGoboIntersects|TestDiffractionScalar' -v
```

Expected: all 7 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/acoustics/diffraction.go internal/acoustics/diffraction_test.go
git commit -m "$(cat <<'EOF'
feat(acoustics): add Maekawa barrier diffraction scalar
EOF
)"
```

---

## Task 3: Mirroring helpers

**Files:**
- Modify: `internal/acoustics/diffraction_test.go`
- Modify: `internal/acoustics/diffraction.go`

- [ ] **Step 1: Add four mirroring tests to `diffraction_test.go`**

Append after the existing scalar tests. Wall index constants (`wallEast`, `wallFloor`, etc.) are defined in `image_source.go` and accessible here since both files are in `package acoustics`.

```go
func TestMirrorGoboEastWall(t *testing.T) {
	// Gobo at x ∈ [2,3] mirrored across east wall x=10: x ∈ [17,18].
	g := scene.Gobo{X1: 2.0, Y1: 1.0, X2: 3.0, Y2: 1.0, Height: 2.0}
	room := scene.Room{Width: 10.0, Depth: 8.0, Height: 3.0}
	m, ok := mirrorGoboAcrossWall(g, wallEast, room)
	assert.True(t, ok)
	assert.InDelta(t, 18.0, m.X1, 1e-9) // 2*10 - 2
	assert.InDelta(t, 17.0, m.X2, 1e-9) // 2*10 - 3
	assert.Equal(t, g.Y1, m.Y1)
	assert.Equal(t, g.Y2, m.Y2)
	assert.Equal(t, g.Height, m.Height)
}

func TestMirrorGoboNorthWall(t *testing.T) {
	// Gobo at y ∈ [1,2] mirrored across north wall y=8: y ∈ [14,15].
	g := scene.Gobo{X1: 1.0, Y1: 1.0, X2: 1.0, Y2: 2.0, Height: 2.0}
	room := scene.Room{Width: 10.0, Depth: 8.0, Height: 3.0}
	m, ok := mirrorGoboAcrossWall(g, wallNorth, room)
	assert.True(t, ok)
	assert.InDelta(t, 15.0, m.Y1, 1e-9) // 2*8 - 1
	assert.InDelta(t, 14.0, m.Y2, 1e-9) // 2*8 - 2
}

func TestMirrorGoboFloorCeiling(t *testing.T) {
	// Floor and ceiling walls return ok=false — gobos are vertical panels
	// that don't interact meaningfully with floor/ceiling image geometry.
	g := scene.Gobo{X1: 1.0, Y1: 1.0, X2: 3.0, Y2: 1.0, Height: 2.0}
	room := scene.Room{Width: 10.0, Depth: 8.0, Height: 3.0}
	_, ok := mirrorGoboAcrossWall(g, wallFloor, room)
	assert.False(t, ok, "floor wall should return ok=false")
	_, ok = mirrorGoboAcrossWall(g, wallCeiling, room)
	assert.False(t, ok, "ceiling wall should return ok=false")
}

func TestEffectiveGobos(t *testing.T) {
	room := scene.Room{Width: 10.0, Depth: 8.0, Height: 3.0}
	gobos := []scene.Gobo{testGobo}

	t.Run("first order returns original plus mirrored", func(t *testing.T) {
		// wallHits[wallEast]=1 → sum=1 → 1st order east reflection
		img := imageSource{wallHits: [6]int{wallEast: 1}}
		effective := effectiveGobos(img, gobos, room)
		assert.Len(t, effective, 2, "expected original + mirrored gobo")
	})

	t.Run("higher order returns originals only", func(t *testing.T) {
		// wallHits[wallEast]=1, wallHits[wallNorth]=1 → sum=2 → 2nd order
		img := imageSource{}
		img.wallHits[wallEast] = 1
		img.wallHits[wallNorth] = 1
		effective := effectiveGobos(img, gobos, room)
		assert.Len(t, effective, 1, "expected original gobos only for higher-order reflection")
	})
}
```

- [ ] **Step 2: Run tests to verify they fail (compile error)**

```bash
go test ./internal/acoustics/ -run 'TestMirrorGobo|TestEffectiveGobos' -v
```

Expected: compile error `undefined: mirrorGoboAcrossWall`

- [ ] **Step 3: Add `mirrorGoboAcrossWall` and `effectiveGobos` to `diffraction.go`**

Append after `DiffractionScalar`:

```go
// mirrorGoboAcrossWall returns a copy of g with its footprint coordinates
// reflected across the given wall plane. Returns ok=false for floor and
// ceiling walls, which don't interact meaningfully with vertical gobo geometry.
func mirrorGoboAcrossWall(g scene.Gobo, wall int, room scene.Room) (scene.Gobo, bool) {
	m := g
	switch wall {
	case wallWest:
		m.X1, m.X2 = -g.X1, -g.X2
	case wallEast:
		m.X1, m.X2 = 2*room.Width-g.X1, 2*room.Width-g.X2
	case wallSouth:
		m.Y1, m.Y2 = -g.Y1, -g.Y2
	case wallNorth:
		m.Y1, m.Y2 = 2*room.Depth-g.Y1, 2*room.Depth-g.Y2
	default:
		return scene.Gobo{}, false
	}
	return m, true
}

// effectiveGobos returns the gobos to test against the image-source-to-mic
// segment for img.
//
// For 1st-order reflections (sum(wallHits) == 1), each gobo is also mirrored
// across the hit wall. The mirrored copy lands in image space (outside the
// room) and intercepts the image-source-to-mic segment exactly where the real
// reflected path would be blocked on the source-to-reflection-point leg.
// For higher-order reflections, only the original gobos are tested (covering
// the reflection-point-to-mic leg only).
//
// Limitations:
//   - 2nd-order and higher: only the mic-side leg is tested. Full coverage
//     requires tracking the ordered wall sequence, not just per-wall counts.
//   - Floor/ceiling reflections: excluded from mirroring.
//
// TODO: To support full gobo occlusion at all reflection orders, change
// imageSource.wallHits from a [6]int count array to a []int ordered wall
// sequence. With the sequence: (1) reconstruct each intermediate reflection
// point by intersecting the image-source-to-mic line with each wall in order;
// (2) test each sub-segment against appropriately mirrored gobos for that leg.
//
// TODO: For 2nd-order same-wall double-bounce (e.g. east→east), mirroring
// the gobo twice across the same axis is well-defined without ordered sequence
// tracking. The double-mirror places the gobo at an additional offset of
// 2*(W - gobo_x) further out, covering flutter-echo geometry.
func effectiveGobos(img imageSource, gobos []scene.Gobo, room scene.Room) []scene.Gobo {
	total := 0
	for _, h := range img.wallHits {
		total += h
	}
	if total != 1 {
		return gobos
	}
	hitWall := -1
	for i, h := range img.wallHits {
		if h > 0 {
			hitWall = i
			break
		}
	}
	result := make([]scene.Gobo, len(gobos), len(gobos)*2)
	copy(result, gobos)
	for _, g := range gobos {
		if mirrored, ok := mirrorGoboAcrossWall(g, hitWall, room); ok {
			result = append(result, mirrored)
		}
	}
	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/acoustics/ -run 'TestGoboIntersects|TestDiffractionScalar|TestMirrorGobo|TestEffectiveGobos' -v
```

Expected: all 12 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/acoustics/diffraction.go internal/acoustics/diffraction_test.go
git commit -m "$(cat <<'EOF'
feat(acoustics): add gobo mirroring helpers for reflected-path occlusion
EOF
)"
```

---

## Task 4: Wire gobos into ComputeDirect

**Files:**
- Modify: `internal/acoustics/diffraction_test.go`
- Modify: `internal/acoustics/direct.go`
- Modify: `internal/acoustics/direct_test.go`

- [ ] **Step 1: Add `TestGoboAttenuatesDirect` to `diffraction_test.go`**

Append after the existing mirroring tests:

```go
func TestGoboAttenuatesDirect(t *testing.T) {
	// Guitar source at (1.5,1.0,1.2), room mic at (2.5,3.5,1.8).
	// The direct path crosses y=2 at x≈1.9, z≈1.44 — within testGobo bounds.
	src := scene.Source{ID: "s", X: 1.5, Y: 1.0, Z: 1.2}
	mic := scene.Mic{
		ID:      "m",
		X:       2.5, Y: 3.5, Z: 1.8,
		Aim:     scene.Aim{Azimuth: 180, Elevation: 0},
		Pattern: "omni",
	}
	without := ComputeDirect(src, mic, 48000, nil)
	with := ComputeDirect(src, mic, 48000, []scene.Gobo{testGobo})

	assert.Less(t, with.Amplitude, without.Amplitude, "gobo should reduce amplitude")
	assert.Equal(t, without.DelaySamples, with.DelaySamples, "gobo should not affect delay")
}
```

- [ ] **Step 2: Update `ComputeDirect` signature in `direct.go` (without calling DiffractionScalar yet)**

Replace the function signature line and add the unused parameter (the test must compile but fail):

```go
func ComputeDirect(src scene.Source, mic scene.Mic, sampleRate int, gobos []scene.Gobo) PathContribution {
	srcPos := geometry.Vec3{X: src.X, Y: src.Y, Z: src.Z}
	micPos := geometry.Vec3{X: mic.X, Y: mic.Y, Z: mic.Z}

	dist := micPos.Sub(srcPos).Length()
	if dist < 0.001 {
		dist = 0.001
	}

	delaySamples := int(math.Round(dist / speedOfSound * float64(sampleRate)))
	amplitude := 1.0 / dist
	airScalar := math.Pow(10, -airAbsorptionDBPerM*dist/20)
	sourceDir := srcPos.Sub(micPos).Normalize()
	polar := PolarGain(mic.Pattern, mic.Aim.Azimuth, mic.Aim.Elevation, sourceDir)

	return PathContribution{
		DelaySamples: delaySamples,
		Amplitude:    amplitude * airScalar * polar,
	}
}
```

Note: `gobos` is accepted but not yet used — the test will fail because amplitude is unchanged.

- [ ] **Step 3: Update all call sites in `direct_test.go` to pass `nil` as the fourth argument**

There are 5 calls to `ComputeDirect` in `direct_test.go`. Change each one:

```go
// Before:
c := ComputeDirect(src, mic, 48000)

// After:
c := ComputeDirect(src, mic, 48000, nil)
```

Apply to all five tests: `TestComputeDirectDelay`, `TestComputeDirectAmplitudeAt1m`, `TestComputeDirectInverseDistance`, `TestComputeDirectCardioidRear`, `TestComputeDirectCoincidentClamp`.

- [ ] **Step 4: Run tests to verify `TestGoboAttenuatesDirect` fails**

```bash
go test ./internal/acoustics/ -run TestGoboAttenuatesDirect -v
```

Expected: FAIL — `with.Amplitude` equals `without.Amplitude` (gobos param not used yet).

- [ ] **Step 5: Add `DiffractionScalar` call to `ComputeDirect`**

In `direct.go`, add the diffraction computation and include it in the returned amplitude:

```go
func ComputeDirect(src scene.Source, mic scene.Mic, sampleRate int, gobos []scene.Gobo) PathContribution {
	srcPos := geometry.Vec3{X: src.X, Y: src.Y, Z: src.Z}
	micPos := geometry.Vec3{X: mic.X, Y: mic.Y, Z: mic.Z}

	dist := micPos.Sub(srcPos).Length()
	if dist < 0.001 {
		dist = 0.001
	}

	delaySamples := int(math.Round(dist / speedOfSound * float64(sampleRate)))
	amplitude := 1.0 / dist
	airScalar := math.Pow(10, -airAbsorptionDBPerM*dist/20)
	sourceDir := srcPos.Sub(micPos).Normalize()
	polar := PolarGain(mic.Pattern, mic.Aim.Azimuth, mic.Aim.Elevation, sourceDir)
	diffraction := DiffractionScalar(srcPos, micPos, gobos)

	return PathContribution{
		DelaySamples: delaySamples,
		Amplitude:    amplitude * airScalar * polar * diffraction,
	}
}
```

- [ ] **Step 6: Run all acoustics tests to verify they pass**

```bash
go test ./internal/acoustics/ -v
```

Expected: all tests PASS, including the five existing `TestComputeDirect*` tests and the new `TestGoboAttenuatesDirect`.

- [ ] **Step 7: Commit**

```bash
git add internal/acoustics/direct.go internal/acoustics/direct_test.go internal/acoustics/diffraction_test.go
git commit -m "$(cat <<'EOF'
feat(acoustics): apply gobo diffraction attenuation to direct path
EOF
)"
```

---

## Task 5: Wire gobos into ComputeReflections

**Files:**
- Modify: `internal/acoustics/diffraction_test.go`
- Modify: `internal/acoustics/image_source.go`
- Modify: `internal/acoustics/image_source_test.go`

- [ ] **Step 1: Add `TestGoboAttenuatesReflection` to `diffraction_test.go`**

Append after `TestGoboAttenuatesDirect`:

```go
func TestGoboAttenuatesReflection(t *testing.T) {
	// Source at (2,2,1), mic at (7,4,1), 10×8×4 room, all concrete.
	// A gobo at x=8 (panel running y=2 to y=4) blocks both legs of the
	// east-wall 1st-order reflection:
	//   - mic-side leg: reflection point (≈10, 3.45, 1) → mic (7,4,1) passes x=8
	//   - source-side leg: source (2,2,1) → reflection point passes x=8
	//     (caught via mirrored gobo at x=12 in image space)
	src := scene.Source{X: 2, Y: 2, Z: 1}
	mic := scene.Mic{
		X: 7, Y: 4, Z: 1,
		Pattern: "omni",
		Aim:     scene.Aim{Azimuth: 0, Elevation: 0},
	}
	room := scene.Room{
		Width: 10, Depth: 8, Height: 4,
		Surfaces: scene.Surfaces{
			West: "concrete", East: "concrete",
			South: "concrete", North: "concrete",
			Floor: "concrete", Ceiling: "concrete",
		},
	}
	gobo := scene.Gobo{X1: 8, Y1: 2, X2: 8, Y2: 4, Height: 2, Material: "plywood"}

	without := ComputeReflections(src, mic, room, 1, 48000, nil)
	with := ComputeReflections(src, mic, room, 1, 48000, []scene.Gobo{gobo})

	var sumWithout, sumWith float64
	for _, c := range without {
		sumWithout += c.Amplitude
	}
	for _, c := range with {
		sumWith += c.Amplitude
	}
	assert.Less(t, sumWith, sumWithout, "gobo should reduce total reflection amplitude")
}
```

- [ ] **Step 2: Update `ComputeReflections` signature in `image_source.go` (without using gobos yet)**

Change only the function signature line:

```go
func ComputeReflections(src scene.Source, mic scene.Mic, room scene.Room, maxOrder, sampleRate int, gobos []scene.Gobo) []PathContribution {
```

Leave the body unchanged. The test will compile but fail because amplitudes are unaffected.

- [ ] **Step 3: Update call sites in `image_source_test.go` to pass `nil` as the final argument**

There are three calls to `ComputeReflections` in `image_source_test.go`. Add `nil` as the last argument to each:

```go
// Before:
contribs := ComputeReflections(src, mic, room, 0, 48000)
contribs := ComputeReflections(src, mic, room, 1, 48000)

// After:
contribs := ComputeReflections(src, mic, room, 0, 48000, nil)
contribs := ComputeReflections(src, mic, room, 1, 48000, nil)
```

Apply to `TestComputeReflections_Order0`, `TestComputeReflections_Order1Count`, and `TestComputeReflections_FirstOrderDelay`.

- [ ] **Step 4: Run tests to verify `TestGoboAttenuatesReflection` fails**

```bash
go test ./internal/acoustics/ -run TestGoboAttenuatesReflection -v
```

Expected: FAIL — `sumWith` equals `sumWithout`.

- [ ] **Step 5: Add `effectiveGobos` + `DiffractionScalar` to `ComputeReflections` in `image_source.go`**

Replace the inner loop body. The complete updated `ComputeReflections` function:

```go
func ComputeReflections(src scene.Source, mic scene.Mic, room scene.Room, maxOrder, sampleRate int, gobos []scene.Gobo) []PathContribution {
	if maxOrder <= 0 {
		return nil
	}

	micPos := geometry.Vec3{X: mic.X, Y: mic.Y, Z: mic.Z}
	materials := [6]string{
		wallWest:    room.Surfaces.West,
		wallEast:    room.Surfaces.East,
		wallSouth:   room.Surfaces.South,
		wallNorth:   room.Surfaces.North,
		wallFloor:   room.Surfaces.Floor,
		wallCeiling: room.Surfaces.Ceiling,
	}

	images := enumerateImageSources(src, room, maxOrder)
	contributions := make([]PathContribution, 0, len(images))

	for _, img := range images {
		dist := micPos.Sub(img.pos).Length()
		if dist < 0.001 {
			dist = 0.001
		}
		delaySamples := int(math.Round(dist / speedOfSound * float64(sampleRate)))
		amplitude := 1.0 / dist
		airScalar := math.Pow(10, -airAbsorptionDBPerM*dist/20)
		sourceDir := img.pos.Sub(micPos).Normalize()
		polar := PolarGain(mic.Pattern, mic.Aim.Azimuth, mic.Aim.Elevation, sourceDir)
		absScalar := absorptionScalar(img.wallHits, materials)
		effGobos := effectiveGobos(img, gobos, room)
		diffractionScalar := DiffractionScalar(img.pos, micPos, effGobos)

		contributions = append(contributions, PathContribution{
			DelaySamples: delaySamples,
			Amplitude:    amplitude * airScalar * polar * absScalar * diffractionScalar,
		})
	}

	return contributions
}
```

- [ ] **Step 6: Run all acoustics tests to verify they pass**

```bash
go test ./internal/acoustics/ -v
```

Expected: all tests PASS, including the three existing `TestComputeReflections_*` tests and the new `TestGoboAttenuatesReflection`.

- [ ] **Step 7: Commit**

```bash
git add internal/acoustics/image_source.go internal/acoustics/image_source_test.go internal/acoustics/diffraction_test.go
git commit -m "$(cat <<'EOF'
feat(acoustics): apply gobo diffraction attenuation to reflected paths
EOF
)"
```

---

## Task 6: Engine wiring, example scene, integration test

**Files:**
- Modify: `internal/engine/engine.go`
- Modify: `internal/engine/engine_test.go`
- Modify: `examples/small_room.json`

- [ ] **Step 1: Update `engine.go` to pass `s.Gobos` to both compute functions**

In `engine.go`, replace the two compute calls inside the source-mic loop:

```go
contributions := []acoustics.PathContribution{acoustics.ComputeDirect(src, mic, sampleRate, s.Gobos)}
if cfg.ReflectionOrder > 0 {
    contributions = append(contributions,
        acoustics.ComputeReflections(src, mic, s.Room, cfg.ReflectionOrder, sampleRate, s.Gobos)...)
}
```

- [ ] **Step 2: Run all tests to verify engine still compiles and existing tests pass**

```bash
go test ./...
```

Expected: all tests PASS (`small_room.json` has empty `gobos: []` so behaviour is unchanged for existing tests).

- [ ] **Step 3: Add gobo to `examples/small_room.json`**

Replace the `"gobos": []` line with:

```json
  "gobos": [
    {
      "id": "guitar_screen",
      "x1": 1.0, "y1": 2.0,
      "x2": 3.0, "y2": 2.0,
      "height": 2.0,
      "material": "plywood"
    }
  ]
```

This gobo runs along y=2 from x=1 to x=3 at height 2 m. It sits between the guitar source (y=1.0) and the room mic (y=3.5), blocking that direct path. The guitar close mic is at y=1.0 (same side as the source) and is not blocked.

- [ ] **Step 4: Add `TestRunSmallRoom_GoboChangesOutput` to `engine_test.go`**

Append after the existing tests:

```go
func TestRunSmallRoom_GoboChangesOutput(t *testing.T) {
	// Run the same scene with and without the gobo to verify the gobo actually
	// changes the output bytes for the blocked guitar→room pair.
	// small_room.json contains a gobo blocking guitar→room.

	outWithGobo := t.TempDir()
	require.NoError(t, Run(Config{
		ScenePath: "../../examples/small_room.json",
		OutputDir: outWithGobo,
		Duration:  1.0,
	}))

	// Write a temporary scene file with gobos removed.
	noGoboScene := `{
  "version": 1,
  "sample_rate": 48000,
  "room": {
    "width": 5.0,
    "depth": 4.0,
    "height": 2.8,
    "surfaces": {
      "floor":   "hardwood_floor",
      "ceiling": "acoustic_tile",
      "north":   "drywall",
      "south":   "drywall",
      "east":    "drywall",
      "west":    "glass_window"
    }
  },
  "sources": [
    { "id": "guitar", "x": 1.5, "y": 1.0, "z": 1.2 },
    { "id": "vocal",  "x": 3.5, "y": 2.0, "z": 1.5 }
  ],
  "mics": [
    {
      "id": "guitar_close",
      "x": 1.7, "y": 1.0, "z": 1.2,
      "aim": { "azimuth": 270, "elevation": 0 },
      "pattern": "cardioid"
    },
    {
      "id": "room",
      "x": 2.5, "y": 3.5, "z": 1.8,
      "aim": { "azimuth": 180, "elevation": -10 },
      "pattern": "omni"
    }
  ],
  "gobos": []
}`
	sceneFile := filepath.Join(t.TempDir(), "no_gobo.json")
	require.NoError(t, os.WriteFile(sceneFile, []byte(noGoboScene), 0644))

	outNoGobo := t.TempDir()
	require.NoError(t, Run(Config{
		ScenePath: sceneFile,
		OutputDir: outNoGobo,
		Duration:  1.0,
	}))

	withBytes, err := os.ReadFile(filepath.Join(outWithGobo, "guitar_to_room.wav"))
	require.NoError(t, err)
	withoutBytes, err := os.ReadFile(filepath.Join(outNoGobo, "guitar_to_room.wav"))
	require.NoError(t, err)
	assert.NotEqual(t, withBytes, withoutBytes, "guitar_to_room.wav should differ when gobo is present")
}
```

- [ ] **Step 5: Run all tests to verify they pass**

```bash
go test ./...
```

Expected: all tests PASS. The new integration test confirms the gobo changes the WAV output for the blocked pair.

- [ ] **Step 6: Commit**

```bash
git add internal/engine/engine.go internal/engine/engine_test.go examples/small_room.json
git commit -m "$(cat <<'EOF'
feat: wire gobo occlusion through engine; add gobo to small_room example
EOF
)"
```

---

## Verification

After all tasks are complete, run the full test suite and a manual generate:

```bash
go test ./...
go build ./cmd/airpath && ./airpath generate -scene examples/small_room.json -output ./output/ -duration 1.0
```

The output should include `guitar_to_room.wav` with a reduced amplitude direct impulse compared to running without the gobo in the scene.
