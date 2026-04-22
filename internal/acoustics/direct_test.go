package acoustics

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/andapony/airpath/internal/scene"
)

func TestComputeDirectDelay(t *testing.T) {
	src := scene.Source{ID: "s", X: 0, Y: 0, Z: 0}
	mic := scene.Mic{
		ID:      "m",
		X:       1, Y: 0, Z: 0,
		Aim:     scene.Aim{Azimuth: 270, Elevation: 0},
		Pattern: "omni",
	}
	c := ComputeDirect(src, mic, 48000)
	assert.Equal(t, 140, c.DelaySamples)
}

func TestComputeDirectAmplitudeAt1m(t *testing.T) {
	src := scene.Source{ID: "s", X: 0, Y: 0, Z: 0}
	mic := scene.Mic{
		ID:      "m",
		X:       1, Y: 0, Z: 0,
		Aim:     scene.Aim{Azimuth: 270, Elevation: 0},
		Pattern: "omni",
	}
	c := ComputeDirect(src, mic, 48000)
	assert.InDelta(t, 1.0, c.Amplitude, 0.01)
}

func TestComputeDirectInverseDistance(t *testing.T) {
	src := scene.Source{ID: "s", X: 0, Y: 0, Z: 0}
	mic1 := scene.Mic{ID: "m1", X: 1, Y: 0, Z: 0, Aim: scene.Aim{Azimuth: 270}, Pattern: "omni"}
	mic2 := scene.Mic{ID: "m2", X: 2, Y: 0, Z: 0, Aim: scene.Aim{Azimuth: 270}, Pattern: "omni"}
	c1 := ComputeDirect(src, mic1, 48000)
	c2 := ComputeDirect(src, mic2, 48000)
	assert.InDelta(t, 2.0, c1.Amplitude/c2.Amplitude, 0.05)
}

func TestComputeDirectCardioidRear(t *testing.T) {
	src := scene.Source{ID: "s", X: 0, Y: 0, Z: 0}
	mic := scene.Mic{
		ID:      "m",
		X:       1, Y: 0, Z: 0,
		Aim:     scene.Aim{Azimuth: 90, Elevation: 0},
		Pattern: "cardioid",
	}
	c := ComputeDirect(src, mic, 48000)
	assert.InDelta(t, 0.0, c.Amplitude, 0.01)
}

func TestComputeDirectCoincidentClamp(t *testing.T) {
	src := scene.Source{ID: "s", X: 1, Y: 1, Z: 1}
	mic := scene.Mic{ID: "m", X: 1, Y: 1, Z: 1, Aim: scene.Aim{}, Pattern: "omni"}
	c := ComputeDirect(src, mic, 48000)
	assert.Equal(t, 0, c.DelaySamples)
	assert.Positive(t, c.Amplitude)
}
