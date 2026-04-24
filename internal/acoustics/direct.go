package acoustics

import (
	"math"

	"github.com/andapony/airpath/internal/geometry"
	"github.com/andapony/airpath/internal/scene"
)

const (
	// speedOfSound is the assumed speed of sound in air at ~20 °C, in m/s.
	// Used to convert path distances to sample delays. Temperature and humidity
	// dependence is not modelled.
	speedOfSound = 343.0

	// airAbsorptionDBPerM is a single mid-band scalar approximation for air
	// absorption, in dB/m. Real air absorption is strongly frequency-dependent
	// (negligible at 125 Hz, severe above 4 kHz) and also depends on temperature
	// and relative humidity. This constant will be replaced by per-band FIR
	// filtering in M5.
	airAbsorptionDBPerM = 0.003
)

// ComputeDirect computes the PathContribution for the direct line-of-sight path
// from src to mic, accounting for distance attenuation, air absorption, microphone
// polar pattern gain, and gobo diffraction.
//
// The amplitude model assumes a point source radiating omnidirectionally into
// free space. Amplitude falls off as 1/distance (pressure domain, normalised
// to 1.0 at 1 m). This is the far-field approximation; it over-estimates
// amplitude for sources very close to the microphone.
//
// When src and mic are coincident (distance < 1 mm), distance is clamped to
// 1 mm to prevent division by zero. The resulting amplitude of 1000.0 is
// physically unrealistic but avoids NaN propagation.
//
// Limitations:
//   - Air absorption uses a single mid-band scalar (see airAbsorptionDBPerM).
//   - No near-field correction for very short distances.
//   - Sources are assumed omnidirectional (no source directivity).
func ComputeDirect(src scene.Source, mic scene.Mic, sampleRate int, gobos []scene.Gobo) PathContribution {
	srcPos := geometry.Vec3{X: src.X, Y: src.Y, Z: src.Z}
	micPos := geometry.Vec3{X: mic.X, Y: mic.Y, Z: mic.Z}

	dist := micPos.Sub(srcPos).Length()
	if dist < 0.001 {
		dist = 0.001 // clamp to avoid division by zero at coincident positions
	}

	delaySamples := int(math.Round(dist / speedOfSound * float64(sampleRate)))

	// Inverse-distance attenuation (pressure domain), normalised to 1.0 at 1 m.
	amplitude := 1.0 / dist

	// Air absorption: single mid-band scalar approximation. M5 replaces this with per-band FIR.
	airScalar := math.Pow(10, -airAbsorptionDBPerM*dist/20)

	// Polar pattern gain: depends on the angle between the mic's aim and the
	// direction from which sound arrives (i.e. from src toward mic).
	sourceDir := srcPos.Sub(micPos).Normalize()
	polar := PolarGain(mic.Pattern, mic.Aim.Azimuth, mic.Aim.Elevation, sourceDir)

	// Gobo diffraction: product of Maekawa scalars for all intersecting gobos.
	diffraction := DiffractionScalar(srcPos, micPos, gobos)

	return PathContribution{
		DelaySamples: delaySamples,
		Amplitude:    amplitude * airScalar * polar * diffraction,
	}
}
