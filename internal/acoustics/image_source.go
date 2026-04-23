package acoustics

import (
	"math"

	"github.com/andapony/airpath/internal/geometry"
	"github.com/andapony/airpath/internal/scene"
)

// Wall indices for the wallHits array used throughout this file.
const (
	wallWest    = 0
	wallEast    = 1
	wallSouth   = 2
	wallNorth   = 3
	wallFloor   = 4
	wallCeiling = 5
)

// band1kHz is the index of the 1000 Hz octave band in scene.Bands.
// Used as a mid-band approximation for surface absorption scalars;
// M5 replaces this with per-band FIR filtering.
const band1kHz = 3

type imageSource struct {
	pos      geometry.Vec3
	wallHits [6]int // west, east, south, north, floor, ceiling
}

// iabs returns the absolute value of n. math.Abs operates on float64; this
// avoids an int→float64 conversion in the hot path.
func iabs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// imageCoord computes the image source coordinate for lattice index n,
// room dimension L, and real source coordinate s.
// n=0 returns s (the real source position, not an image).
func imageCoord(n int, L, s float64) float64 {
	if n%2 == 0 {
		return float64(n)*L + s
	}
	return float64(n)*L + L - s
}

// axisHits returns how many times the positive-side and negative-side walls
// are hit for lattice index n on one axis.
// Positive-side: east (x), north (y), ceiling (z).
// Negative-side: west (x), south (y), floor (z).
func axisHits(n int) (posWall, negWall int) {
	if n == 0 {
		return 0, 0
	}
	a := iabs(n)
	if n > 0 {
		return (a + 1) / 2, a / 2
	}
	return a / 2, (a + 1) / 2
}

// reflectionScalar computes ∏ (1−α_i)^hits_i for each wall.
func reflectionScalar(hits [6]int, alphas [6]float64) float64 {
	scalar := 1.0
	for i, alpha := range alphas {
		if hits[i] > 0 {
			scalar *= math.Pow(1.0-alpha, float64(hits[i]))
		}
	}
	return scalar
}

// absorptionScalar looks up the 1kHz absorption coefficient for each wall
// material and calls reflectionScalar. Unknown materials are treated as
// perfect reflectors (absorption 0); the scene validator rejects unknown
// materials before we get here.
func absorptionScalar(hits [6]int, materials [6]string) float64 {
	var alphas [6]float64
	for i, mat := range materials {
		if absorption, ok := scene.KnownMaterials[mat]; ok {
			alphas[i] = absorption[band1kHz]
		}
	}
	return reflectionScalar(hits, alphas)
}

// enumerateImageSources returns all image sources for src within room up to
// maxOrder total reflections, excluding the real source (order 0).
func enumerateImageSources(src scene.Source, room scene.Room, maxOrder int) []imageSource {
	var sources []imageSource
	for p := -maxOrder; p <= maxOrder; p++ {
		for q := -(maxOrder - iabs(p)); q <= maxOrder-iabs(p); q++ {
			rMax := maxOrder - iabs(p) - iabs(q)
			for r := -rMax; r <= rMax; r++ {
				if p == 0 && q == 0 && r == 0 {
					continue
				}
				eastHits, westHits := axisHits(p)
				northHits, southHits := axisHits(q)
				ceilingHits, floorHits := axisHits(r)

				sources = append(sources, imageSource{
					pos: geometry.Vec3{
						X: imageCoord(p, room.Width, src.X),
						Y: imageCoord(q, room.Depth, src.Y),
						Z: imageCoord(r, room.Height, src.Z),
					},
					wallHits: [6]int{
						wallWest:    westHits,
						wallEast:    eastHits,
						wallSouth:   southHits,
						wallNorth:   northHits,
						wallFloor:   floorHits,
						wallCeiling: ceilingHits,
					},
				})
			}
		}
	}
	return sources
}

// ComputeReflections returns PathContributions for all image-source reflections
// up to maxOrder for the given source-mic pair. Returns nil when maxOrder <= 0.
//
// TODO: add path-length culling to skip image sources whose travel distance
// exceeds the IR duration, pruning contributions that fall outside the IR buffer.
func ComputeReflections(src scene.Source, mic scene.Mic, room scene.Room, maxOrder, sampleRate int, gobos []scene.Gobo) []PathContribution {
	if maxOrder <= 0 {
		return nil
	}

	micPos := geometry.Vec3{X: mic.X, Y: mic.Y, Z: mic.Z}
	materials := [6]string{
		wallWest:    room.Surfaces.West,
		wallEast:    room.Surfaces.East,
		wallSouth:   room.Surfaces.South,
		wallNorth:   room.Surfaces.North,
		wallFloor:   room.Surfaces.Floor,
		wallCeiling: room.Surfaces.Ceiling,
	}

	images := enumerateImageSources(src, room, maxOrder)
	contributions := make([]PathContribution, 0, len(images))

	for _, img := range images {
		dist := micPos.Sub(img.pos).Length()
		if dist < 0.001 {
			dist = 0.001
		}
		delaySamples := int(math.Round(dist / speedOfSound * float64(sampleRate)))
		amplitude := 1.0 / dist
		airScalar := math.Pow(10, -airAbsorptionDBPerM*dist/20)
		sourceDir := img.pos.Sub(micPos).Normalize()
		polar := PolarGain(mic.Pattern, mic.Aim.Azimuth, mic.Aim.Elevation, sourceDir)
		absScalar := absorptionScalar(img.wallHits, materials)
		effGobos := effectiveGobos(img, gobos, room)
		diffractionScalar := DiffractionScalar(img.pos, micPos, effGobos)

		contributions = append(contributions, PathContribution{
			DelaySamples: delaySamples,
			Amplitude:    amplitude * airScalar * polar * absScalar * diffractionScalar,
		})
	}

	return contributions
}
