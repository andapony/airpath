package acoustics

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/andapony/airpath/internal/geometry"
)

// TestPolarGainOmni verifies that the omni pattern returns 1.0 regardless of
// the incoming direction, confirming the a=1.0 special-case path.
func TestPolarGainOmni(t *testing.T) {
	dirs := []geometry.Vec3{
		{X: 0, Y: 1, Z: 0}, {X: 0, Y: -1, Z: 0}, {X: 1, Y: 0, Z: 0}, {X: 0, Y: 0, Z: 1},
	}
	for _, d := range dirs {
		assert.Equal(t, 1.0, PolarGain("omni", 0, 0, d), "omni dir=%v", d)
	}
}

// TestPolarGainCardioidOnAxis verifies maximum gain (1.0) when the source
// arrives from directly in front of the mic (azimuth=0 → aim=+Y, sourceDir=+Y).
func TestPolarGainCardioidOnAxis(t *testing.T) {
	frontDir := geometry.Vec3{X: 0, Y: 1, Z: 0}
	assert.InDelta(t, 1.0, PolarGain("cardioid", 0, 0, frontDir), 1e-9)
}

// TestPolarGainCardioidRear verifies zero gain when the source arrives from
// directly behind the mic (sourceDir=−Y, aim=+Y → θ=180° → gain=0.5+(1-0.5)×(−1)=0).
func TestPolarGainCardioidRear(t *testing.T) {
	rearDir := geometry.Vec3{X: 0, Y: -1, Z: 0}
	assert.InDelta(t, 0.0, PolarGain("cardioid", 0, 0, rearDir), 1e-9)
}

// TestPolarGainCardioidSide verifies the 90° (side) gain for a cardioid: with
// a=0.5, gain = 0.5 + 0.5×cos(90°) = 0.5.
func TestPolarGainCardioidSide(t *testing.T) {
	sideDir := geometry.Vec3{X: 1, Y: 0, Z: 0}
	assert.InDelta(t, 0.5, PolarGain("cardioid", 0, 0, sideDir), 1e-9)
}

// TestPolarGainFigure8Rear verifies that the figure-8 pattern clamps to 0 at
// the rear null. Without clamping, a=0 would yield 0+1×cos(180°)=−1; the
// clamp ensures the returned value is non-negative.
func TestPolarGainFigure8Rear(t *testing.T) {
	rearDir := geometry.Vec3{X: 0, Y: -1, Z: 0}
	assert.Equal(t, 0.0, PolarGain("figure8", 0, 0, rearDir))
}

// TestPolarGainNonNegative checks that PolarGain never returns a negative value
// for any directional pattern when the source arrives from a null or rear angle.
// Negative gain would indicate a phase inversion, which the current model does
// not represent.
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
