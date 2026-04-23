package acoustics

import (
	"math"
	"testing"

	"github.com/andapony/airpath/internal/scene"
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
	contribs := ComputeReflections(src, mic, room, 0, 48000, nil)
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
	contribs := ComputeReflections(src, mic, room, 1, 48000, nil)
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
	contribs := ComputeReflections(src, mic, room, 1, 48000, nil)

	delays := make(map[int]bool, len(contribs))
	for _, c := range contribs {
		delays[c.DelaySamples] = true
	}
	assert.True(t, delays[1539], "expected east-wall reflection at 1539 samples (11m)")
	assert.True(t, delays[1259], "expected west-wall reflection at 1259 samples (9m)")
}
