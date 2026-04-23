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
	assert.InDelta(t, 0.1003, got, 0.001, "expected Maekawa scalar for testGobo geometry")
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
