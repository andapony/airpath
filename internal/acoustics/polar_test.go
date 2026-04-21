package acoustics

import (
	"math"
	"testing"

	"github.com/andapony/airpath/internal/geometry"
)

func TestPolarGainOmni(t *testing.T) {
	dirs := []geometry.Vec3{
		{X: 0, Y: 1, Z: 0}, {X: 0, Y: -1, Z: 0}, {X: 1, Y: 0, Z: 0}, {X: 0, Y: 0, Z: 1},
	}
	for _, d := range dirs {
		if got := PolarGain("omni", 0, 0, d); got != 1.0 {
			t.Errorf("omni dir=%v: got %v, want 1.0", d, got)
		}
	}
}

func TestPolarGainCardioidOnAxis(t *testing.T) {
	// Mic aimed at azimuth=0 (+Y direction).
	// Source directly in front: sourceDir from mic to source = +Y.
	// gain = 0.5 + 0.5*cos(0) = 1.0
	frontDir := geometry.Vec3{X: 0, Y: 1, Z: 0}
	got := PolarGain("cardioid", 0, 0, frontDir)
	if math.Abs(got-1.0) > 1e-9 {
		t.Errorf("cardioid on-axis = %v, want 1.0", got)
	}
}

func TestPolarGainCardioidRear(t *testing.T) {
	// Source directly behind: sourceDir = −Y.
	// gain = 0.5 + 0.5*cos(180°) = 0.5 - 0.5 = 0.0
	rearDir := geometry.Vec3{X: 0, Y: -1, Z: 0}
	got := PolarGain("cardioid", 0, 0, rearDir)
	if math.Abs(got-0.0) > 1e-9 {
		t.Errorf("cardioid rear = %v, want 0.0", got)
	}
}

func TestPolarGainCardioidSide(t *testing.T) {
	// Source 90° off-axis: sourceDir = +X.
	// gain = 0.5 + 0.5*cos(90°) = 0.5
	sideDir := geometry.Vec3{X: 1, Y: 0, Z: 0}
	got := PolarGain("cardioid", 0, 0, sideDir)
	if math.Abs(got-0.5) > 1e-9 {
		t.Errorf("cardioid side = %v, want 0.5", got)
	}
}

func TestPolarGainFigure8Rear(t *testing.T) {
	// Figure-8: a=0. Rear gain = max(0, 0 + 1*cos(180°)) = max(0, -1) = 0
	rearDir := geometry.Vec3{X: 0, Y: -1, Z: 0}
	got := PolarGain("figure8", 0, 0, rearDir)
	if got != 0.0 {
		t.Errorf("figure8 rear = %v, want 0.0 (clamped)", got)
	}
}

func TestPolarGainNonNegative(t *testing.T) {
	dirs := []geometry.Vec3{
		{X: 0, Y: -1, Z: 0}, {X: -1, Y: 0, Z: 0}, {X: 0, Y: 0, Z: -1},
	}
	for _, d := range dirs {
		for _, pattern := range []string{"cardioid", "supercardioid", "figure8"} {
			if got := PolarGain(pattern, 0, 0, d); got < 0 {
				t.Errorf("%s dir=%v: gain %v is negative", pattern, d, got)
			}
		}
	}
}
