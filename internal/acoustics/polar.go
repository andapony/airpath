package acoustics

import (
	"math"

	"github.com/andapony/airpath/internal/geometry"
)

var patternCoeff = map[string]float64{
	"omni":          1.0,
	"cardioid":      0.5,
	"supercardioid": 0.37,
	"figure8":       0.0,
}

// PolarGain returns the microphone gain [0,1] for a source arriving from sourceDir,
// given the mic's polar pattern and aim direction (azimuth/elevation in degrees).
// sourceDir must be a unit vector (length 1).
//
// Azimuth 0 = +Y (north), 90 = +X (east), 180 = −Y, 270 = −X.
// Elevation 0 = horizontal, positive = upward (+Z).
func PolarGain(pattern string, azimuthDeg, elevationDeg float64, sourceDir geometry.Vec3) float64 {
	a, ok := patternCoeff[pattern]
	if !ok {
		return 1.0
	}
	if a == 1.0 {
		return 1.0
	}
	aim := aimToVec3(azimuthDeg, elevationDeg)
	cosTheta := aim.Dot(sourceDir)
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

// aimToVec3 converts azimuth and elevation angles (in degrees) to a unit direction vector.
func aimToVec3(azimuthDeg, elevationDeg float64) geometry.Vec3 {
	az := azimuthDeg * math.Pi / 180
	el := elevationDeg * math.Pi / 180
	return geometry.Vec3{
		X: math.Sin(az) * math.Cos(el),
		Y: math.Cos(az) * math.Cos(el),
		Z: math.Sin(el),
	}.Normalize()
}
