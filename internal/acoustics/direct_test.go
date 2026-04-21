package acoustics

import (
	"math"
	"testing"

	"github.com/andapony/airpath/internal/scene"
)

func TestComputeDirectDelay(t *testing.T) {
	// Source at origin, mic 1m away along +X. Speed of sound = 343 m/s.
	// delay = round(1.0 / 343.0 * 48000) = round(139.94) = 140 samples
	src := scene.Source{ID: "s", X: 0, Y: 0, Z: 0}
	mic := scene.Mic{
		ID:      "m",
		X:       1, Y: 0, Z: 0,
		Aim:     scene.Aim{Azimuth: 270, Elevation: 0}, // aimed −X toward source
		Pattern: "omni",
	}
	c := ComputeDirect(src, mic, 48000)
	if c.DelaySamples != 140 {
		t.Errorf("DelaySamples = %d, want 140", c.DelaySamples)
	}
}

func TestComputeDirectAmplitudeAt1m(t *testing.T) {
	// At 1m with omni: amplitude = 1/dist * polar(1.0) * airScalar(≈1.0) ≈ 1.0
	src := scene.Source{ID: "s", X: 0, Y: 0, Z: 0}
	mic := scene.Mic{
		ID:      "m",
		X:       1, Y: 0, Z: 0,
		Aim:     scene.Aim{Azimuth: 270, Elevation: 0},
		Pattern: "omni",
	}
	c := ComputeDirect(src, mic, 48000)
	if math.Abs(c.Amplitude-1.0) > 0.01 {
		t.Errorf("Amplitude at 1m = %v, want ≈1.0", c.Amplitude)
	}
}

func TestComputeDirectInverseDistance(t *testing.T) {
	// At 2m amplitude should be roughly half of 1m amplitude (inverse-distance law).
	src := scene.Source{ID: "s", X: 0, Y: 0, Z: 0}
	mic1 := scene.Mic{ID: "m1", X: 1, Y: 0, Z: 0, Aim: scene.Aim{Azimuth: 270}, Pattern: "omni"}
	mic2 := scene.Mic{ID: "m2", X: 2, Y: 0, Z: 0, Aim: scene.Aim{Azimuth: 270}, Pattern: "omni"}
	c1 := ComputeDirect(src, mic1, 48000)
	c2 := ComputeDirect(src, mic2, 48000)
	ratio := c1.Amplitude / c2.Amplitude
	if math.Abs(ratio-2.0) > 0.05 {
		t.Errorf("amplitude ratio 1m/2m = %v, want ≈2.0 (inverse-distance)", ratio)
	}
}

func TestComputeDirectCardioidRear(t *testing.T) {
	// Mic aimed +X (away from source at origin), cardioid: rear gain ≈ 0.
	src := scene.Source{ID: "s", X: 0, Y: 0, Z: 0}
	mic := scene.Mic{
		ID:      "m",
		X:       1, Y: 0, Z: 0,
		Aim:     scene.Aim{Azimuth: 90, Elevation: 0}, // aimed +X, away from source
		Pattern: "cardioid",
	}
	c := ComputeDirect(src, mic, 48000)
	if c.Amplitude > 0.01 {
		t.Errorf("cardioid rear amplitude = %v, want ≈0", c.Amplitude)
	}
}

func TestComputeDirectCoincidentClamp(t *testing.T) {
	// Source and mic at same position: should not panic, delay = 0.
	src := scene.Source{ID: "s", X: 1, Y: 1, Z: 1}
	mic := scene.Mic{ID: "m", X: 1, Y: 1, Z: 1, Aim: scene.Aim{}, Pattern: "omni"}
	c := ComputeDirect(src, mic, 48000)
	if c.DelaySamples != 0 {
		t.Errorf("coincident DelaySamples = %d, want 0", c.DelaySamples)
	}
	if c.Amplitude <= 0 {
		t.Errorf("coincident Amplitude = %v, want > 0", c.Amplitude)
	}
}
