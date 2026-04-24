package acoustics

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/andapony/airpath/internal/geometry"
	"github.com/andapony/airpath/internal/scene"
)

// testGobo runs along y=2 from x=1 to x=3, height 2 m. Reused across tasks.
var testGobo = scene.Gobo{X1: 1.0, Y1: 2.0, X2: 3.0, Y2: 2.0, Height: 2.0, Material: "plywood"}

// TestGoboIntersects_blocked verifies that a segment passing directly through
// the centre of testGobo is detected as a hit with footprint parameter s=0.5.
func TestGoboIntersects_blocked(t *testing.T) {
	// Segment passes through the gobo at x=2, y=2, z=1 (midpoint of panel).
	a := geometry.Vec3{X: 2.0, Y: 1.0, Z: 1.0}
	b := geometry.Vec3{X: 2.0, Y: 3.0, Z: 1.0}
	hit, s := goboIntersects(a, b, testGobo)
	assert.True(t, hit)
	assert.InDelta(t, 0.5, s, 1e-9)
}

// TestGoboIntersects_overTop confirms that a path passing above the gobo's
// height (z=2.5 > height=2.0) is not classified as blocked.
func TestGoboIntersects_overTop(t *testing.T) {
	// Segment passes at z=2.5, above the gobo's height of 2.0.
	a := geometry.Vec3{X: 2.0, Y: 1.0, Z: 2.5}
	b := geometry.Vec3{X: 2.0, Y: 3.0, Z: 2.5}
	hit, _ := goboIntersects(a, b, testGobo)
	assert.False(t, hit, "path above gobo height should not intersect")
}

// TestGoboIntersects_parallel confirms that a segment running parallel to the
// gobo plane (denominator ≈ 0) returns hit=false without division-by-zero.
func TestGoboIntersects_parallel(t *testing.T) {
	// Segment runs along y=1, parallel to the gobo plane (normal is in Y).
	a := geometry.Vec3{X: 1.0, Y: 1.0, Z: 1.0}
	b := geometry.Vec3{X: 3.0, Y: 1.0, Z: 1.0}
	hit, _ := goboIntersects(a, b, testGobo)
	assert.False(t, hit, "segment parallel to gobo plane should not intersect")
}

// TestGoboIntersects_pastEnd confirms that a path crossing the gobo plane
// outside the gobo's horizontal extent (x=0.5 < x1=1.0) is not blocked.
func TestGoboIntersects_pastEnd(t *testing.T) {
	// Segment crosses the y=2 plane at x=0.5, outside the gobo's x range [1,3].
	a := geometry.Vec3{X: 0.5, Y: 1.0, Z: 1.0}
	b := geometry.Vec3{X: 0.5, Y: 3.0, Z: 1.0}
	hit, _ := goboIntersects(a, b, testGobo)
	assert.False(t, hit, "crossing point outside horizontal extent should not intersect")
}

// TestDiffractionScalarNone verifies that DiffractionScalar returns 1.0 (no
// attenuation) when the gobos slice is nil — no barriers in the scene.
func TestDiffractionScalarNone(t *testing.T) {
	a := geometry.Vec3{X: 0, Y: 0, Z: 1}
	b := geometry.Vec3{X: 5, Y: 5, Z: 1}
	assert.Equal(t, 1.0, DiffractionScalar(a, b, nil))
}

// TestDiffractionScalarBlocked checks the Maekawa scalar for the testGobo
// geometry: path from (2,1,1) to (2,3,1), gobo from y=2 with height=2.
// The expected value 0.1003 is derived from the formula at 1 kHz.
func TestDiffractionScalarBlocked(t *testing.T) {
	// testGobo blocks the path from (2,1,1) to (2,3,1).
	a := geometry.Vec3{X: 2.0, Y: 1.0, Z: 1.0}
	b := geometry.Vec3{X: 2.0, Y: 3.0, Z: 1.0}
	got := DiffractionScalar(a, b, []scene.Gobo{testGobo})
	assert.InDelta(t, 0.1003, got, 0.001, "expected Maekawa scalar for testGobo geometry")
}

// TestDiffractionScalarMultiple verifies that two gobos in the path produce
// greater attenuation than one alone, confirming scalar multiplication.
func TestDiffractionScalarMultiple(t *testing.T) {
	// Two gobos in the path: one at y=2 (testGobo) and one at y=3.
	gobo2 := scene.Gobo{X1: 1.0, Y1: 3.0, X2: 3.0, Y2: 3.0, Height: 2.0, Material: "plywood"}
	a := geometry.Vec3{X: 2.0, Y: 0.0, Z: 1.0}
	b := geometry.Vec3{X: 2.0, Y: 5.0, Z: 1.0}
	one := DiffractionScalar(a, b, []scene.Gobo{testGobo})
	two := DiffractionScalar(a, b, []scene.Gobo{testGobo, gobo2})
	assert.Less(t, two, one, "two blocking gobos must attenuate more than one")
}

// TestMirrorGoboEastWall verifies that a gobo at x∈[2,3] is correctly reflected
// to x∈[17,18] across the east wall at x=10 (formula: 2×W − original).
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

// TestMirrorGoboNorthWall verifies that a gobo at y∈[1,2] is reflected to
// y∈[14,15] across the north wall at y=8 (formula: 2×Depth − original).
func TestMirrorGoboNorthWall(t *testing.T) {
	// Gobo at y ∈ [1,2] mirrored across north wall y=8: y ∈ [14,15].
	g := scene.Gobo{X1: 1.0, Y1: 1.0, X2: 1.0, Y2: 2.0, Height: 2.0}
	room := scene.Room{Width: 10.0, Depth: 8.0, Height: 3.0}
	m, ok := mirrorGoboAcrossWall(g, wallNorth, room)
	assert.True(t, ok)
	assert.InDelta(t, 15.0, m.Y1, 1e-9) // 2*8 - 1
	assert.InDelta(t, 14.0, m.Y2, 1e-9) // 2*8 - 2
}

// TestMirrorGoboFloorCeiling confirms that floor and ceiling walls return
// ok=false, since gobos are vertical panels and don't interact meaningfully
// with floor/ceiling image geometry.
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

// TestMirrorGoboWestWall verifies reflection across the west wall at x=0:
// a gobo at x∈[2,3] maps to x∈[−2,−3] (formula: negate original x).
func TestMirrorGoboWestWall(t *testing.T) {
	// Gobo at x ∈ [2,3] mirrored across west wall x=0: x ∈ [-3,-2].
	g := scene.Gobo{X1: 2.0, Y1: 1.0, X2: 3.0, Y2: 1.0, Height: 2.0}
	room := scene.Room{Width: 10.0, Depth: 8.0, Height: 3.0}
	m, ok := mirrorGoboAcrossWall(g, wallWest, room)
	assert.True(t, ok)
	assert.InDelta(t, -2.0, m.X1, 1e-9)
	assert.InDelta(t, -3.0, m.X2, 1e-9)
	assert.Equal(t, g.Y1, m.Y1)
}

// TestMirrorGoboSouthWall verifies reflection across the south wall at y=0:
// a gobo at y∈[1,2] maps to y∈[−1,−2] (formula: negate original y).
func TestMirrorGoboSouthWall(t *testing.T) {
	// Gobo at y ∈ [1,2] mirrored across south wall y=0: y ∈ [-2,-1].
	g := scene.Gobo{X1: 1.0, Y1: 1.0, X2: 1.0, Y2: 2.0, Height: 2.0}
	room := scene.Room{Width: 10.0, Depth: 8.0, Height: 3.0}
	m, ok := mirrorGoboAcrossWall(g, wallSouth, room)
	assert.True(t, ok)
	assert.InDelta(t, -1.0, m.Y1, 1e-9)
	assert.InDelta(t, -2.0, m.Y2, 1e-9)
}

// TestEffectiveGobos checks that first-order reflections produce both the
// original and mirrored gobo copies, while higher-order reflections return
// only the originals (mirroring is not defined without wall-sequence tracking).
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

// TestGoboAttenuatesDirect is an integration test confirming that a gobo
// blocking the direct path reduces the ComputeDirect amplitude without
// changing the delay. The guitar→room path crosses y=2 inside testGobo bounds.
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

// TestGoboAttenuatesReflection is an integration test confirming that a gobo
// near the east wall reduces the amplitude of the first-order east-wall
// reflection returned by ComputeReflections, both in aggregate and for the
// specific east-wall contribution identified by its expected delay.
func TestGoboAttenuatesReflection(t *testing.T) {
	// Room 10×8×4m, source at (2,2,1), mic at (7,4,1).
	// Gobo at east wall (x=8) blocking the east-wall 1st-order reflection.
	src := scene.Source{ID: "s", X: 2, Y: 2, Z: 1}
	mic := scene.Mic{
		ID:      "m",
		X:       7, Y: 4, Z: 1,
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

	// Compute reflections without gobo.
	contribsWithout := ComputeReflections(src, mic, room, 1, 48000, nil)
	ampWithout := 0.0
	for _, c := range contribsWithout {
		ampWithout += c.Amplitude
	}

	// Compute reflections with gobo.
	contribsWith := ComputeReflections(src, mic, room, 1, 48000, []scene.Gobo{gobo})
	ampWith := 0.0
	for _, c := range contribsWith {
		ampWith += c.Amplitude
	}

	assert.Less(t, ampWith, ampWithout, "gobo should reduce total amplitude of reflections")

	// Also verify the east-wall reflection is individually attenuated.
	// The east-wall image source (source X=2, room W=10) is at X=18.
	// Its distance to mic at (7,4,1) ≈ 11.18m → delay ≈ 1565 samples.
	// Find the contribution whose delay is closest to 1565 in the no-gobo set.
	targetDelay := 1565
	bestDelay := -1
	for _, c := range contribsWithout {
		if bestDelay < 0 || iabs(c.DelaySamples-targetDelay) < iabs(bestDelay-targetDelay) {
			bestDelay = c.DelaySamples
		}
	}
	var ampWithoutEast, ampWithEast float64
	for _, c := range contribsWithout {
		if c.DelaySamples == bestDelay {
			ampWithoutEast += c.Amplitude
		}
	}
	for _, c := range contribsWith {
		if c.DelaySamples == bestDelay {
			ampWithEast += c.Amplitude
		}
	}
	assert.Less(t, ampWithEast, ampWithoutEast, "east-wall reflection should be individually attenuated")
}
