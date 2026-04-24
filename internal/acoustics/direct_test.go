package acoustics

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/andapony/airpath/internal/scene"
)

// TestComputeDirectDelay verifies that a source 1 m away at 48000 Hz produces
// a delay of 140 samples: round(1/343×48000) = round(139.94) = 140.
func TestComputeDirectDelay(t *testing.T) {
	src := scene.Source{ID: "s", X: 0, Y: 0, Z: 0}
	mic := scene.Mic{
		ID:      "m",
		X:       1, Y: 0, Z: 0,
		Aim:     scene.Aim{Azimuth: 270, Elevation: 0},
		Pattern: "omni",
	}
	c := ComputeDirect(src, mic, 48000, nil)
	assert.Equal(t, 140, c.DelaySamples)
}

// TestComputeDirectAmplitudeAt1m checks the normalisation point of the inverse-
// distance model: amplitude at 1 m from an omni mic should be ≈1.0, with a
// small deviation from air absorption (airAbsorptionDBPerM × 1 m ≈ 0.003 dB).
func TestComputeDirectAmplitudeAt1m(t *testing.T) {
	src := scene.Source{ID: "s", X: 0, Y: 0, Z: 0}
	mic := scene.Mic{
		ID:      "m",
		X:       1, Y: 0, Z: 0,
		Aim:     scene.Aim{Azimuth: 270, Elevation: 0},
		Pattern: "omni",
	}
	c := ComputeDirect(src, mic, 48000, nil)
	assert.InDelta(t, 1.0, c.Amplitude, 0.01)
}

// TestComputeDirectInverseDistance confirms the 1/distance pressure law: a mic
// at 2 m should produce half the amplitude of one at 1 m (6 dB). The 5%
// tolerance absorbs the small difference in air absorption across the two paths.
func TestComputeDirectInverseDistance(t *testing.T) {
	src := scene.Source{ID: "s", X: 0, Y: 0, Z: 0}
	mic1 := scene.Mic{ID: "m1", X: 1, Y: 0, Z: 0, Aim: scene.Aim{Azimuth: 270}, Pattern: "omni"}
	mic2 := scene.Mic{ID: "m2", X: 2, Y: 0, Z: 0, Aim: scene.Aim{Azimuth: 270}, Pattern: "omni"}
	c1 := ComputeDirect(src, mic1, 48000, nil)
	c2 := ComputeDirect(src, mic2, 48000, nil)
	assert.InDelta(t, 2.0, c1.Amplitude/c2.Amplitude, 0.05)
}

// TestComputeDirectCardioidRear verifies that a cardioid mic aimed east (+X)
// has zero gain for a source arriving from the east (i.e. from its rear).
// The mic aim azimuth=90 points +X; the source is at x=0 so it arrives from −X,
// which is directly behind the mic → PolarGain should return 0.
func TestComputeDirectCardioidRear(t *testing.T) {
	src := scene.Source{ID: "s", X: 0, Y: 0, Z: 0}
	mic := scene.Mic{
		ID:      "m",
		X:       1, Y: 0, Z: 0,
		Aim:     scene.Aim{Azimuth: 90, Elevation: 0},
		Pattern: "cardioid",
	}
	c := ComputeDirect(src, mic, 48000, nil)
	assert.InDelta(t, 0.0, c.Amplitude, 0.01)
}

// TestComputeDirectCoincidentClamp verifies that placing the source and mic at
// the same position does not produce a NaN or zero amplitude. The 1 mm distance
// clamp yields amplitude=1000, delay=0.
func TestComputeDirectCoincidentClamp(t *testing.T) {
	src := scene.Source{ID: "s", X: 1, Y: 1, Z: 1}
	mic := scene.Mic{ID: "m", X: 1, Y: 1, Z: 1, Aim: scene.Aim{}, Pattern: "omni"}
	c := ComputeDirect(src, mic, 48000, nil)
	assert.Equal(t, 0, c.DelaySamples)
	assert.Positive(t, c.Amplitude)
}
