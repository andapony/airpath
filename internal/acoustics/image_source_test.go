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
