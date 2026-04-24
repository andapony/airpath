package acoustics

import (
	"math"

	"github.com/andapony/airpath/internal/geometry"
)

// patternCoeff maps polar pattern names to their cardioid-family coefficient a.
// The gain formula is: gain(θ) = a + (1−a)·cos(θ), where θ is the angle
// between the mic's aim direction and the incoming sound direction.
// a=1.0 → omni (constant gain), a=0.5 → cardioid, a=0.37 → supercardioid,
// a=0.0 → figure-8 (bidirectional).
var patternCoeff = map[string]float64{
	"omni":          1.0,
	"cardioid":      0.5,
	"supercardioid": 0.37,
	"figure8":       0.0,
}

// PolarGain returns the microphone gain in [0, 1] for a sound arriving from
// sourceDir, given the mic's polar pattern name and aim direction.
//
// The gain formula is the standard cardioid family: gain(θ) = a + (1−a)·cos(θ),
// where a is the pattern coefficient and θ is the angle between the mic's aim
// direction and sourceDir. Negative values (which arise for figure-8 and
// supercardioid patterns at extreme rear angles) are clamped to 0 — this
// model does not represent phase inversion.
//
// Assumptions:
//   - sourceDir must be a unit vector; the caller is responsible for normalising it.
//   - Azimuth 0° = north (+Y), 90° = east (+X), increasing clockwise from above.
//   - Elevation 0° = horizontal, positive = upward (+Z).
//   - Unknown pattern names are treated as omni (gain = 1.0).
//
// Limitation: the cardioid family is a single-parameter approximation.
// Real microphones deviate from this model, especially at high frequencies
// where the pattern narrows due to capsule diffraction. Per-frequency polar
// patterns are not modelled.
func PolarGain(pattern string, azimuthDeg, elevationDeg float64, sourceDir geometry.Vec3) float64 {
	a, ok := patternCoeff[pattern]
	if !ok {
		return 1.0
	}
	// Omni is a special case: a=1 makes (1−a)·cos(θ) = 0 regardless of direction.
	if a == 1.0 {
		return 1.0
	}
	aim := aimToVec3(azimuthDeg, elevationDeg)
	cosTheta := aim.Dot(sourceDir)
	// Clamp to [−1, 1] to guard against floating-point rounding errors on
	// unit vectors that push the dot product slightly outside this range.
	if cosTheta > 1 {
		cosTheta = 1
	} else if cosTheta < -1 {
		cosTheta = -1
	}
	gain := a + (1-a)*cosTheta
	if gain < 0 {
		gain = 0
	}
	return gain
}

// aimToVec3 converts a mic aim direction from azimuth/elevation angles
// (in degrees) to a normalised unit direction vector in the room coordinate system.
//
// Azimuth is measured clockwise from north (+Y) when viewed from above:
//   - 0°  → +Y (north)
//   - 90° → +X (east)
//
// Elevation is measured from the horizontal plane: positive = upward (+Z).
// The result is normalised to unit length.
func aimToVec3(azimuthDeg, elevationDeg float64) geometry.Vec3 {
	az := azimuthDeg * math.Pi / 180
	el := elevationDeg * math.Pi / 180
	return geometry.Vec3{
		X: math.Sin(az) * math.Cos(el),
		Y: math.Cos(az) * math.Cos(el),
		Z: math.Sin(el),
	}.Normalize()
}
