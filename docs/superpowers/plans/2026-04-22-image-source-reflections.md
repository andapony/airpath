# Image-Source Reflections Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add specular reflections to the IR generator using the image-source method for rectangular rooms, with surface absorption as a scalar mid-band (1kHz) approximation.

**Architecture:** Enumerate image source positions via lattice indices (p, q, r) where |p|+|q|+|r| ≤ maxOrder, compute each image's position and wall hit counts in closed form, then accumulate PathContributions alongside the existing direct path. No changes to PathContribution, AssembleIR, output/, or scene/.

**Tech Stack:** Go stdlib only. Testify (assert/require) already used in the test suite. `go test ./...` is the test command throughout.

---

## File Map

| File | Status | Responsibility |
|------|--------|---------------|
| `internal/acoustics/image_source.go` | **Create** | `imageCoord`, `axisHits`, `iabs`, `reflectionScalar`, `absorptionScalar`, `enumerateImageSources`, `ComputeReflections` |
| `internal/acoustics/image_source_test.go` | **Create** | Unit tests for all unexported helpers and `ComputeReflections` |
| `internal/engine/engine.go` | **Modify** | Add `ReflectionOrder int` to `Config`; call `ComputeReflections` in `Run` |
| `internal/engine/engine_test.go` | **Modify** | Add integration test for `ReflectionOrder=1` |
| `cmd/airpath/main.go` | **Modify** | Add `-order` flag, wire into `engine.Config` |

---

## Task 1: Coordinate helpers

Create `image_source.go` with the building-block functions: `iabs`, `imageCoord`, and `axisHits`. All three are unexported.

**Files:**
- Create: `internal/acoustics/image_source.go`
- Create: `internal/acoustics/image_source_test.go`

### Background

**`imageCoord(n int, L, s float64) float64`** computes the image source position along one axis.
- `n` = lattice index (integer, positive or negative)
- `L` = room dimension on this axis
- `s` = real source coordinate on this axis
- Formula: even `n` → `n*L + s`; odd `n` → `n*L + (L - s)`
- In Go: `n%2 == 0` correctly identifies even for negative n (e.g. `-2 % 2 == 0`)

Verification: 10m room, source at 3m:
- `n=0` (real source): `0*10 + 3 = 3`
- `n=1` (east mirror): `1*10 + (10-3) = 17`
- `n=-1` (west mirror): `-1*10 + (10-3) = -3`
- `n=2` (double-bounce): `2*10 + 3 = 23`
- `n=-2` (double-bounce west): `-2*10 + 3 = -17`

**`axisHits(n int) (posWall, negWall int)`** returns the wall hit counts for one axis.
- `n=0`: `(0, 0)`
- `n>0`: posWall = `(|n|+1)/2`, negWall = `|n|/2` (integer division)
- `n<0`: posWall = `|n|/2`, negWall = `(|n|+1)/2`
- Mapping: for x-axis, posWall = east, negWall = west; for y-axis, posWall = north, negWall = south; for z-axis, posWall = ceiling, negWall = floor.

Verification:
- `axisHits(0)` → `(0, 0)` — no reflections
- `axisHits(1)` → `(1, 0)` — one bounce off positive wall
- `axisHits(-1)` → `(0, 1)` — one bounce off negative wall
- `axisHits(2)` → `(1, 1)` — bounce off each wall
- `axisHits(3)` → `(2, 1)` — two off positive, one off negative
- `axisHits(-3)` → `(1, 2)` — one off positive, two off negative

- [ ] **Step 1.1: Write the failing tests**

Create `internal/acoustics/image_source_test.go`:

```go
package acoustics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageCoord(t *testing.T) {
	assert.InDelta(t, 3.0, imageCoord(0, 10, 3), 1e-9)
	assert.InDelta(t, 17.0, imageCoord(1, 10, 3), 1e-9)
	assert.InDelta(t, -3.0, imageCoord(-1, 10, 3), 1e-9)
	assert.InDelta(t, 23.0, imageCoord(2, 10, 3), 1e-9)
	assert.InDelta(t, -17.0, imageCoord(-2, 10, 3), 1e-9)
}

func TestAxisHits(t *testing.T) {
	tests := []struct {
		n       int
		wantPos int
		wantNeg int
	}{
		{0, 0, 0},
		{1, 1, 0},
		{-1, 0, 1},
		{2, 1, 1},
		{-2, 1, 1},
		{3, 2, 1},
		{-3, 1, 2},
	}
	for _, tt := range tests {
		pos, neg := axisHits(tt.n)
		assert.Equal(t, tt.wantPos, pos, "axisHits(%d) posWall", tt.n)
		assert.Equal(t, tt.wantNeg, neg, "axisHits(%d) negWall", tt.n)
	}
}
```

- [ ] **Step 1.2: Run to verify failure**

```
go test ./internal/acoustics/ -run 'TestImageCoord|TestAxisHits' -v
```

Expected: compile error — `imageCoord` and `axisHits` undefined.

- [ ] **Step 1.3: Create `internal/acoustics/image_source.go` with the helpers**

```go
package acoustics

import (
	"math"

	"github.com/andapony/airpath/internal/geometry"
	"github.com/andapony/airpath/internal/scene"
)

// Wall indices for the wallHits array used throughout this file.
const (
	wallWest    = 0
	wallEast    = 1
	wallSouth   = 2
	wallNorth   = 3
	wallFloor   = 4
	wallCeiling = 5
)

// band1kHz is the index of the 1000 Hz octave band in scene.Bands.
// Used as a mid-band approximation for surface absorption scalars;
// M5 replaces this with per-band FIR filtering.
const band1kHz = 3

type imageSource struct {
	pos      geometry.Vec3
	wallHits [6]int // west, east, south, north, floor, ceiling
}

func iabs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// imageCoord computes the image source coordinate for lattice index n,
// room dimension L, and real source coordinate s.
// n=0 returns s (the real source position, not an image).
func imageCoord(n int, L, s float64) float64 {
	if n%2 == 0 {
		return float64(n)*L + s
	}
	return float64(n)*L + L - s
}

// axisHits returns how many times the positive-side and negative-side walls
// are hit for lattice index n on one axis.
// Positive-side: east (x), north (y), ceiling (z).
// Negative-side: west (x), south (y), floor (z).
func axisHits(n int) (posWall, negWall int) {
	if n == 0 {
		return 0, 0
	}
	a := iabs(n)
	if n > 0 {
		return (a + 1) / 2, a / 2
	}
	return a / 2, (a + 1) / 2
}

// reflectionScalar computes ∏ (1−α_i)^hits_i for each wall.
func reflectionScalar(hits [6]int, alphas [6]float64) float64 {
	scalar := 1.0
	for i, alpha := range alphas {
		if hits[i] > 0 {
			scalar *= math.Pow(1.0-alpha, float64(hits[i]))
		}
	}
	return scalar
}

// absorptionScalar looks up the 1kHz absorption coefficient for each wall
// material and calls reflectionScalar. Unknown materials are treated as
// perfect reflectors (absorption 0); the scene validator rejects unknown
// materials before we get here.
func absorptionScalar(hits [6]int, materials [6]string) float64 {
	var alphas [6]float64
	for i, mat := range materials {
		if absorption, ok := scene.KnownMaterials[mat]; ok {
			alphas[i] = absorption[band1kHz]
		}
	}
	return reflectionScalar(hits, alphas)
}

// enumerateImageSources returns all image sources for src within room up to
// maxOrder total reflections, excluding the real source (order 0).
func enumerateImageSources(src scene.Source, room scene.Room, maxOrder int) []imageSource {
	var sources []imageSource
	for p := -maxOrder; p <= maxOrder; p++ {
		for q := -(maxOrder - iabs(p)); q <= maxOrder-iabs(p); q++ {
			rMax := maxOrder - iabs(p) - iabs(q)
			for r := -rMax; r <= rMax; r++ {
				if p == 0 && q == 0 && r == 0 {
					continue
				}
				eastHits, westHits := axisHits(p)
				northHits, southHits := axisHits(q)
				ceilingHits, floorHits := axisHits(r)

				sources = append(sources, imageSource{
					pos: geometry.Vec3{
						X: imageCoord(p, room.Width, src.X),
						Y: imageCoord(q, room.Depth, src.Y),
						Z: imageCoord(r, room.Height, src.Z),
					},
					wallHits: [6]int{
						wallWest:    westHits,
						wallEast:    eastHits,
						wallSouth:   southHits,
						wallNorth:   northHits,
						wallFloor:   floorHits,
						wallCeiling: ceilingHits,
					},
				})
			}
		}
	}
	return sources
}

// ComputeReflections returns PathContributions for all image-source reflections
// up to maxOrder for the given source-mic pair. Returns nil when maxOrder ≤ 0.
//
// TODO: add path-length culling to skip image sources whose travel distance
// exceeds the IR duration, pruning contributions that fall outside the IR buffer.
func ComputeReflections(src scene.Source, mic scene.Mic, room scene.Room, maxOrder, sampleRate int) []PathContribution {
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

		contributions = append(contributions, PathContribution{
			DelaySamples: delaySamples,
			Amplitude:    amplitude * airScalar * polar * absScalar,
		})
	}

	return contributions
}
```

- [ ] **Step 1.4: Run tests to verify they pass**

```
go test ./internal/acoustics/ -run 'TestImageCoord|TestAxisHits' -v
```

Expected: PASS for both tests.

- [ ] **Step 1.5: Run full test suite to check for regressions**

```
go test ./...
```

Expected: all tests pass.

- [ ] **Step 1.6: Commit**

```bash
git add internal/acoustics/image_source.go internal/acoustics/image_source_test.go
git commit -m "feat: add image-source coordinate helpers (imageCoord, axisHits)"
```

---

## Task 2: Reflection scalar

Add tests for `reflectionScalar` and `enumerateImageSources`. The implementation is already in `image_source.go` from Task 1 — this task just validates it thoroughly.

**Files:**
- Modify: `internal/acoustics/image_source_test.go`

- [ ] **Step 2.1: Add reflectionScalar tests to `image_source_test.go`**

Append to the existing test file:

```go
func TestReflectionScalar(t *testing.T) {
	var noHits [6]int
	var noAlpha [6]float64

	// no hits → scalar is 1 regardless of alphas
	assert.InDelta(t, 1.0, reflectionScalar(noHits, noAlpha), 1e-9)

	var hits [6]int
	hits[0] = 1
	var alphas [6]float64

	// perfect absorber: (1 - 1.0)^1 = 0
	alphas[0] = 1.0
	assert.InDelta(t, 0.0, reflectionScalar(hits, alphas), 1e-9)

	// perfect reflector: (1 - 0.0)^1 = 1
	alphas[0] = 0.0
	assert.InDelta(t, 1.0, reflectionScalar(hits, alphas), 1e-9)

	// two hits at 0.5 absorption: (0.5)^2 = 0.25
	hits[0] = 2
	alphas[0] = 0.5
	assert.InDelta(t, 0.25, reflectionScalar(hits, alphas), 1e-9)

	// two walls: (0.5)^1 * (0.75)^1 = 0.375
	hits[0] = 1
	alphas[0] = 0.5
	hits[1] = 1
	alphas[1] = 0.25
	assert.InDelta(t, 0.375, reflectionScalar(hits, alphas), 1e-9)
}
```

- [ ] **Step 2.2: Run to verify the tests pass**

```
go test ./internal/acoustics/ -run TestReflectionScalar -v
```

Expected: PASS. The implementation was written in Task 1.

- [ ] **Step 2.3: Commit**

```bash
git add internal/acoustics/image_source_test.go
git commit -m "test: add reflectionScalar unit tests"
```

---

## Task 3: Image source enumeration

Add tests for `enumerateImageSources`.

**Files:**
- Modify: `internal/acoustics/image_source_test.go`

### How many image sources at each order?

At order 1 (`|p|+|q|+|r|=1`): 6 images — one per face: `(±1,0,0)`, `(0,±1,0)`, `(0,0,±1)`.

The loop correctly skips `(0,0,0)` via the `continue` guard.

### Spot-check: image `(p=1, q=0, r=0)` in a room of W=10, D=8, H=4 with source at `(2, 2, 1)`:
- `imageCoord(1, 10, 2)` = `1*10 + (10-2)` = `18`
- `imageCoord(0, 8, 2)` = `0*8 + 2` = `2`
- `imageCoord(0, 4, 1)` = `0*4 + 1` = `1`
- pos = `(18, 2, 1)`
- `axisHits(1)` → `posWall=1, negWall=0` → east=1, west=0
- All other walls: 0 hits

- [ ] **Step 3.1: Add enumeration tests to `image_source_test.go`**

Append to the existing test file:

```go
func TestEnumerateImageSources_Order0(t *testing.T) {
	src := scene.Source{X: 2, Y: 2, Z: 1}
	room := scene.Room{Width: 10, Depth: 8, Height: 4}
	imgs := enumerateImageSources(src, room, 0)
	assert.Empty(t, imgs)
}

func TestEnumerateImageSources_Order1Count(t *testing.T) {
	src := scene.Source{X: 2, Y: 2, Z: 1}
	room := scene.Room{Width: 10, Depth: 8, Height: 4}
	imgs := enumerateImageSources(src, room, 1)
	assert.Len(t, imgs, 6)
}

func TestEnumerateImageSources_EastWallImage(t *testing.T) {
	// image (p=1, q=0, r=0): source at (2,2,1) in 10×8×4 room
	// imageCoord(1, 10, 2) = 18; y and z unchanged
	// axisHits(1) → east=1, west=0; all others 0
	src := scene.Source{X: 2, Y: 2, Z: 1}
	room := scene.Room{Width: 10, Depth: 8, Height: 4}
	imgs := enumerateImageSources(src, room, 1)

	var found bool
	for _, img := range imgs {
		if math.Abs(img.pos.X-18) < 1e-9 &&
			math.Abs(img.pos.Y-2) < 1e-9 &&
			math.Abs(img.pos.Z-1) < 1e-9 {
			assert.Equal(t, 0, img.wallHits[wallWest], "west hits")
			assert.Equal(t, 1, img.wallHits[wallEast], "east hits")
			assert.Equal(t, 0, img.wallHits[wallSouth], "south hits")
			assert.Equal(t, 0, img.wallHits[wallNorth], "north hits")
			assert.Equal(t, 0, img.wallHits[wallFloor], "floor hits")
			assert.Equal(t, 0, img.wallHits[wallCeiling], "ceiling hits")
			found = true
			break
		}
	}
	assert.True(t, found, "expected image source at (18, 2, 1)")
}
```

Note: the test file imports `math` — add it to the import block if not already present:

```go
import (
	"math"
	"testing"

	"github.com/andapony/airpath/internal/scene"
	"github.com/stretchr/testify/assert"
)
```

- [ ] **Step 3.2: Run to verify the tests pass**

```
go test ./internal/acoustics/ -run 'TestEnumerateImageSources' -v
```

Expected: all three PASS.

- [ ] **Step 3.3: Commit**

```bash
git add internal/acoustics/image_source_test.go
git commit -m "test: add enumerateImageSources unit tests"
```

---

## Task 4: ComputeReflections tests

Add tests for the public `ComputeReflections` function.

**Files:**
- Modify: `internal/acoustics/image_source_test.go`

### Delay verification

Room: W=10, D=8, H=4. Source: `(2, 2, 1)`. Mic: `(7, 2, 1)` omni. Sample rate: 48000.

Image `(p=1, q=0, r=0)`: pos = `(18, 2, 1)`, dist = `|7-18| = 11m`
- delay = `round(11 / 343 * 48000)` = `round(1539.07)` = **1539 samples**

Image `(p=-1, q=0, r=0)`: `imageCoord(-1, 10, 2)` = `-1*10 + (10-2)` = `-2`, pos = `(-2, 2, 1)`, dist = `|7-(-2)| = 9m`
- delay = `round(9 / 343 * 48000)` = `round(1259.48)` = **1259 samples**

- [ ] **Step 4.1: Add ComputeReflections tests to `image_source_test.go`**

Append to the existing test file:

```go
func TestComputeReflections_Order0(t *testing.T) {
	src := scene.Source{X: 2, Y: 2, Z: 1}
	mic := scene.Mic{
		X: 7, Y: 2, Z: 1,
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
	contribs := ComputeReflections(src, mic, room, 0, 48000)
	assert.Empty(t, contribs)
}

func TestComputeReflections_Order1Count(t *testing.T) {
	src := scene.Source{X: 2, Y: 2, Z: 1}
	mic := scene.Mic{
		X: 7, Y: 2, Z: 1,
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
	contribs := ComputeReflections(src, mic, room, 1, 48000)
	assert.Len(t, contribs, 6)
}

func TestComputeReflections_FirstOrderDelay(t *testing.T) {
	// src=(2,2,1), mic=(7,2,1), room 10×8×4 m, 48000 Hz, all walls concrete.
	// East-wall image (p=1): x=18, dist=11m → delay=round(11/343*48000)=1539
	// West-wall image (p=-1): x=-2,  dist=9m  → delay=round(9/343*48000)=1259
	src := scene.Source{X: 2, Y: 2, Z: 1}
	mic := scene.Mic{
		X: 7, Y: 2, Z: 1,
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
	contribs := ComputeReflections(src, mic, room, 1, 48000)

	delays := make(map[int]bool, len(contribs))
	for _, c := range contribs {
		delays[c.DelaySamples] = true
	}
	assert.True(t, delays[1539], "expected east-wall reflection at 1539 samples (11m)")
	assert.True(t, delays[1259], "expected west-wall reflection at 1259 samples (9m)")
}
```

- [ ] **Step 4.2: Run to verify the tests pass**

```
go test ./internal/acoustics/ -run 'TestComputeReflections' -v
```

Expected: all three PASS.

- [ ] **Step 4.3: Run full test suite**

```
go test ./...
```

Expected: all tests pass.

- [ ] **Step 4.4: Commit**

```bash
git add internal/acoustics/image_source_test.go
git commit -m "test: add ComputeReflections unit tests"
```

---

## Task 5: Engine integration

Wire `ComputeReflections` into the engine and add an integration test.

**Files:**
- Modify: `internal/engine/engine.go`
- Modify: `internal/engine/engine_test.go`

### What changes in `engine.go`

1. Add `ReflectionOrder int` to `Config` (zero value = direct path only, preserving M1 behaviour).
2. In `Run`, replace the single-contribution slice with an accumulator that optionally appends reflections.

Current loop (lines 44–53):
```go
for _, src := range s.Sources {
    for _, mic := range s.Mics {
        contrib := acoustics.ComputeDirect(src, mic, sampleRate)
        ir := acoustics.AssembleIR([]acoustics.PathContribution{contrib}, lengthSamples)
        ...
    }
}
```

- [ ] **Step 5.1: Write the failing engine integration test first**

Append to `internal/engine/engine_test.go`:

```go
func TestRunSmallRoom_WithReflections(t *testing.T) {
	outDir := t.TempDir()
	err := Run(Config{
		ScenePath:       "../../examples/small_room.json",
		OutputDir:       outDir,
		Duration:        1.0,
		ReflectionOrder: 1,
	})
	require.NoError(t, err)

	for _, name := range []string{
		"guitar_to_guitar_close.wav",
		"guitar_to_room.wav",
		"vocal_to_guitar_close.wav",
		"vocal_to_room.wav",
	} {
		info, err := os.Stat(filepath.Join(outDir, name))
		require.NoError(t, err, "expected output file %s", name)
		assert.Positive(t, info.Size(), "output file %s should not be empty", name)
	}
}
```

- [ ] **Step 5.2: Run to verify the test currently fails to compile**

```
go test ./internal/engine/ -run TestRunSmallRoom_WithReflections -v
```

Expected: compile error — `ReflectionOrder` unknown field in `Config`.

- [ ] **Step 5.3: Update `engine.go`**

Replace the entire file content:

```go
package engine

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/andapony/airpath/internal/acoustics"
	"github.com/andapony/airpath/internal/output"
	"github.com/andapony/airpath/internal/scene"
)

// Config holds runtime parameters for a generate run.
type Config struct {
	ScenePath       string
	OutputDir       string
	SampleRate      int     // overrides scene sample_rate when > 0
	Duration        float64 // IR duration in seconds
	ReflectionOrder int     // maximum reflection order; 0 = direct path only
}

// Run loads the scene, computes IRs for all source-mic pairs, and writes WAV
// files to OutputDir. With ReflectionOrder > 0, image-source reflections are
// accumulated alongside the direct path.
func Run(cfg Config) error {
	s, err := scene.Parse(cfg.ScenePath)
	if err != nil {
		return fmt.Errorf("loading scene: %w", err)
	}

	sampleRate := s.SampleRate
	if cfg.SampleRate > 0 {
		sampleRate = cfg.SampleRate
	}

	if cfg.Duration <= 0 {
		return fmt.Errorf("duration must be positive, got %v seconds", cfg.Duration)
	}

	lengthSamples := int(cfg.Duration * float64(sampleRate))

	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	for _, src := range s.Sources {
		for _, mic := range s.Mics {
			contributions := []acoustics.PathContribution{acoustics.ComputeDirect(src, mic, sampleRate)}
			if cfg.ReflectionOrder > 0 {
				contributions = append(contributions,
					acoustics.ComputeReflections(src, mic, s.Room, cfg.ReflectionOrder, sampleRate)...)
			}
			ir := acoustics.AssembleIR(contributions, lengthSamples)

			filename := fmt.Sprintf("%s_to_%s.wav", src.ID, mic.ID)
			if err := output.WriteWAV(filepath.Join(cfg.OutputDir, filename), ir, sampleRate); err != nil {
				return fmt.Errorf("writing %s: %w", filename, err)
			}
		}
	}

	return nil
}
```

- [ ] **Step 5.4: Run all engine tests**

```
go test ./internal/engine/ -v
```

Expected: `TestRunSmallRoom`, `TestRunMissingScene`, and `TestRunSmallRoom_WithReflections` all PASS.

- [ ] **Step 5.5: Run full test suite**

```
go test ./...
```

Expected: all tests pass.

- [ ] **Step 5.6: Commit**

```bash
git add internal/engine/engine.go internal/engine/engine_test.go
git commit -m "feat: add ReflectionOrder to engine Config; accumulate image-source contributions"
```

---

## Task 6: CLI `-order` flag

Expose `ReflectionOrder` as a command-line flag.

**Files:**
- Modify: `cmd/airpath/main.go`

- [ ] **Step 6.1: Update `runGenerate` in `main.go`**

Replace the `runGenerate` function:

```go
func runGenerate(args []string) {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	scenePath := fs.String("scene", "", "path to scene JSON file (required)")
	outputDir := fs.String("output", "./output", "output directory")
	sampleRate := fs.Int("samplerate", 0, "sample rate override in Hz (default: from scene file)")
	duration := fs.Float64("duration", 1.0, "IR duration in seconds")
	order := fs.Int("order", 4, "maximum reflection order (0 = direct path only)")
	fs.Parse(args)

	if *scenePath == "" {
		fmt.Fprintln(os.Stderr, "error: -scene is required")
		os.Exit(1)
	}

	if err := engine.Run(engine.Config{
		ScenePath:       *scenePath,
		OutputDir:       *outputDir,
		SampleRate:      *sampleRate,
		Duration:        *duration,
		ReflectionOrder: *order,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Done. Output written to %s\n", *outputDir)
}
```

- [ ] **Step 6.2: Build**

```
go build ./cmd/airpath
```

Expected: no errors, binary produced.

- [ ] **Step 6.3: Smoke test with order 1**

```
./airpath generate -scene examples/small_room.json -output ./output/ -order 1
```

Expected output:
```
Done. Output written to ./output/
```

Check that WAV files in `./output/` are non-zero:
```
ls -lh ./output/*.wav
```

- [ ] **Step 6.4: Smoke test with order 0 (direct path only)**

```
./airpath generate -scene examples/small_room.json -output ./output/ -order 0
```

Expected: same files produced, success message printed.

- [ ] **Step 6.5: Run full test suite one final time**

```
go test ./...
```

Expected: all tests pass.

- [ ] **Step 6.6: Commit**

```bash
git add cmd/airpath/main.go
git commit -m "feat: add -order flag to generate subcommand (default 4)"
```
