package acoustics

import (
	"github.com/andapony/airpath/internal/scene"
)

// SabineRT60 returns the estimated reverberation time (seconds) for room using
// the Sabine equation at the 1 kHz mid-band: RT60 = 0.161 * V / A.
// A = Σ(surface_area × α_1kHz) for all six surfaces.
func SabineRT60(room scene.Room) float64 {
	V := room.Width * room.Depth * room.Height

	floorCeiling := room.Width * room.Depth
	northSouth := room.Width * room.Height
	eastWest := room.Depth * room.Height

	const band1k = 3 // index of 1000 Hz in the 7-band array

	alpha := func(mat string) float64 {
		if a, ok := scene.KnownMaterials[mat]; ok {
			return a[band1k]
		}
		return 0
	}

	A := floorCeiling*(alpha(room.Surfaces.Floor)+alpha(room.Surfaces.Ceiling)) +
		northSouth*(alpha(room.Surfaces.North)+alpha(room.Surfaces.South)) +
		eastWest*(alpha(room.Surfaces.East)+alpha(room.Surfaces.West))

	if A <= 0 {
		return 0
	}
	return 0.161 * V / A
}
