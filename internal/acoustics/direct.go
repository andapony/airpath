package acoustics

import (
	"math"

	"github.com/andapony/airpath/internal/geometry"
	"github.com/andapony/airpath/internal/scene"
)

const (
	speedOfSound        = 343.0 // m/s
	airAbsorptionDBPerM = 0.003 // dB/m, mid-band approximation; replaced by FIR filters in M5
)

// ComputeDirect computes the direct line-of-sight path contribution from src to mic.
func ComputeDirect(src scene.Source, mic scene.Mic, sampleRate int) PathContribution {
	srcPos := geometry.Vec3{X: src.X, Y: src.Y, Z: src.Z}
	micPos := geometry.Vec3{X: mic.X, Y: mic.Y, Z: mic.Z}

	dist := micPos.Sub(srcPos).Length()
	if dist < 0.001 {
		dist = 0.001
	}

	delaySamples := int(math.Round(dist / speedOfSound * float64(sampleRate)))

	// Inverse-distance attenuation (pressure domain), normalized to 1.0 at 1m.
	amplitude := 1.0 / dist

	// Air absorption: scalar approximation. M5 replaces this with per-band FIR.
	airScalar := math.Pow(10, -airAbsorptionDBPerM*dist/20)

	// Polar pattern gain.
	sourceDir := srcPos.Sub(micPos).Normalize()
	polar := PolarGain(mic.Pattern, mic.Aim.Azimuth, mic.Aim.Elevation, sourceDir)

	return PathContribution{
		DelaySamples: delaySamples,
		Amplitude:    amplitude * airScalar * polar,
	}
}
