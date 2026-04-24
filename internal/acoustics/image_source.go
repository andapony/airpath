package acoustics

import (
	"math"

	"github.com/andapony/airpath/internal/geometry"
	"github.com/andapony/airpath/internal/scene"
)

// Wall index constants for the wallHits array used throughout this file.
// The array has six entries, one per room face, in this fixed order.
const (
	wallWest    = 0
	wallEast    = 1
	wallSouth   = 2
	wallNorth   = 3
	wallFloor   = 4
	wallCeiling = 5
)

// band1kHz is the index of the 1000 Hz octave band in scene.Bands.
// Used as a mid-band approximation for surface absorption scalars.
// M5 replaces this single-band scalar with per-band FIR filtering.
const band1kHz = 3

// imageSource represents a virtual copy of a source placed outside the room by
// the image-source method. pos is its position in 3D space (which may be far
// outside the physical room for high-order reflections). wallHits records how
// many times each room face was struck on the path from the real source to
// this image, which drives the surface absorption calculation.
type imageSource struct {
	pos      geometry.Vec3
	wallHits [6]int // indexed by wallWest…wallCeiling
}

// iabs returns the absolute value of n.
// math.Abs operates on float64; this integer version avoids a type conversion
// in the hot loop of enumerateImageSources where it is called thousands of times.
func iabs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// imageCoord computes the position of an image source along one axis.
// n is the lattice index (0 = real source, ±1 = first-order images, etc.),
// L is the room dimension on that axis, and s is the real source coordinate.
//
// For even n the image is at n·L + s (the source position repeats).
// For odd n the image is at n·L + (L−s) (the source position is mirrored),
// corresponding to a reflection off a wall on the positive side of the axis.
// n=0 returns s, the real source position.
func imageCoord(n int, L, s float64) float64 {
	if n%2 == 0 {
		return float64(n)*L + s
	}
	return float64(n)*L + L - s
}

// axisHits returns the number of times the positive-side and negative-side
// walls are struck for lattice index n on a single axis.
//
// Positive side: east (x-axis), north (y-axis), ceiling (z-axis).
// Negative side: west (x-axis), south (y-axis), floor (z-axis).
//
// For n=0 there are no reflections. For n>0, ⌈|n|/2⌉ bounces hit the
// positive wall and ⌊|n|/2⌋ hit the negative wall; for n<0 the counts swap.
// This formula is derived from the alternating reflection pattern in the
// image-source lattice.
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

// reflectionScalar computes the cumulative surface absorption scalar for a
// single reflection path: ∏ (1 − αᵢ)^hitsᵢ for each of the six walls.
//
// A wall with zero hits contributes a factor of 1.0 (no absorption).
// A wall with alpha=1.0 (perfect absorber) reduces the scalar to zero
// regardless of other walls.
func reflectionScalar(hits [6]int, alphas [6]float64) float64 {
	scalar := 1.0
	for i, alpha := range alphas {
		if hits[i] > 0 {
			scalar *= math.Pow(1.0-alpha, float64(hits[i]))
		}
	}
	return scalar
}

// absorptionScalar looks up the 1 kHz absorption coefficient for each wall's
// material and returns the cumulative reflection scalar for the given wall-hit
// counts. Unknown materials default to alpha=0 (perfect reflector), but the
// scene validator rejects unknown materials before the engine runs, so this
// fallback is a safety net rather than an expected code path.
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
// maxOrder total reflections, excluding the real source (order 0, lattice
// index (0,0,0)).
//
// The image-source lattice is indexed by integer triples (p, q, r) where
// |p|+|q|+|r| ≤ maxOrder. The nested loops constrain q and r ranges to stay
// within the order budget after fixing p, avoiding redundant rejection.
//
// The returned slice grows with O(maxOrder³) — order 4 yields 258 sources,
// order 8 yields ~2500. Memory is allocated once per call.
func enumerateImageSources(src scene.Source, room scene.Room, maxOrder int) []imageSource {
	var sources []imageSource
	for p := -maxOrder; p <= maxOrder; p++ {
		for q := -(maxOrder - iabs(p)); q <= maxOrder-iabs(p); q++ {
			rMax := maxOrder - iabs(p) - iabs(q)
			for r := -rMax; r <= rMax; r++ {
				if p == 0 && q == 0 && r == 0 {
					continue // skip the real source
				}
				// axisHits axis convention: p → east/west, q → north/south, r → ceiling/floor
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
// Each image source represents a unique sequence of wall bounces. The
// contribution's amplitude is the product of: 1/distance (inverse-square),
// air absorption, polar pattern gain, surface absorption, and gobo diffraction.
// All scalars use 1 kHz mid-band approximations; per-band filtering is deferred to M5.
//
// Limitation: contributions are not culled by IR duration. Image sources whose
// travel distance exceeds duration×343 m/s will produce DelaySamples values
// beyond the IR buffer length and will be silently discarded by AssembleIR.
// Adding a distance-based cull here would avoid computing those contributions.
//
// TODO: add path-length culling — skip image sources whose travel distance
// exceeds cfg.Duration × speedOfSound, pruning out-of-range contributions
// before computing amplitude.
func ComputeReflections(src scene.Source, mic scene.Mic, room scene.Room, maxOrder, sampleRate int, gobos []scene.Gobo) []PathContribution {
	if maxOrder <= 0 {
		return nil
	}

	micPos := geometry.Vec3{X: mic.X, Y: mic.Y, Z: mic.Z}
	// Arrange wall materials in the same order as wallHits indices.
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
			dist = 0.001 // same coincidence guard as ComputeDirect
		}
		delaySamples := int(math.Round(dist / speedOfSound * float64(sampleRate)))
		amplitude := 1.0 / dist
		airScalar := math.Pow(10, -airAbsorptionDBPerM*dist/20)
		// Direction is from the image source toward the mic — equivalent to the
		// direction from which the reflected sound arrives at the mic.
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
