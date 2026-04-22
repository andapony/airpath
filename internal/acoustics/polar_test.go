package acoustics

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/andapony/airpath/internal/geometry"
)

func TestPolarGainOmni(t *testing.T) {
	dirs := []geometry.Vec3{
		{X: 0, Y: 1, Z: 0}, {X: 0, Y: -1, Z: 0}, {X: 1, Y: 0, Z: 0}, {X: 0, Y: 0, Z: 1},
	}
	for _, d := range dirs {
		assert.Equal(t, 1.0, PolarGain("omni", 0, 0, d), "omni dir=%v", d)
	}
}

func TestPolarGainCardioidOnAxis(t *testing.T) {
	frontDir := geometry.Vec3{X: 0, Y: 1, Z: 0}
	assert.InDelta(t, 1.0, PolarGain("cardioid", 0, 0, frontDir), 1e-9)
}

func TestPolarGainCardioidRear(t *testing.T) {
	rearDir := geometry.Vec3{X: 0, Y: -1, Z: 0}
	assert.InDelta(t, 0.0, PolarGain("cardioid", 0, 0, rearDir), 1e-9)
}

func TestPolarGainCardioidSide(t *testing.T) {
	sideDir := geometry.Vec3{X: 1, Y: 0, Z: 0}
	assert.InDelta(t, 0.5, PolarGain("cardioid", 0, 0, sideDir), 1e-9)
}

func TestPolarGainFigure8Rear(t *testing.T) {
	rearDir := geometry.Vec3{X: 0, Y: -1, Z: 0}
	assert.Equal(t, 0.0, PolarGain("figure8", 0, 0, rearDir))
}

func TestPolarGainNonNegative(t *testing.T) {
	dirs := []geometry.Vec3{
		{X: 0, Y: -1, Z: 0}, {X: -1, Y: 0, Z: 0}, {X: 0, Y: 0, Z: -1},
	}
	for _, d := range dirs {
		for _, pattern := range []string{"cardioid", "supercardioid", "figure8"} {
			assert.GreaterOrEqual(t, PolarGain(pattern, 0, 0, d), float64(0), "%s dir=%v", pattern, d)
		}
	}
}
