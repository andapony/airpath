package acoustics

// Maekawa barrier diffraction: gobo-segment intersection geometry and
// frequency-independent amplitude scalar computation.
//
// References:
//   Maekawa, Z. (1968). Noise reduction by screens. Applied Acoustics, 1(3), 157–173.
//   Kuttruff, H. (2009). Room Acoustics (5th ed.), §4.4. Spon Press.
//   Cowan, J.P. (1999). Architectural Acoustics Design Guide, §6.3. McGraw-Hill.

import (
	"math"

	"github.com/andapony/airpath/internal/geometry"
	"github.com/andapony/airpath/internal/scene"
)

// goboIntersects tests whether segment A→B passes through the gobo panel.
// Returns hit=true and footprint parameter s ∈ [0,1] when it does.
// s locates the crossing point along the base edge and is used to place
// the diffraction point on the top edge.
//
// Algorithm:
//  1. Compute the horizontal normal n to the footprint direction d.
//  2. Find where the segment crosses the gobo's plane: t = n·(P1-A) / n·(B-A).
//  3. Check t ∈ [0,1] (within segment), Q.Z ∈ [0,height] (vertical bounds),
//     and s ∈ [0,1] (horizontal bounds along the footprint).
func goboIntersects(a, b geometry.Vec3, g scene.Gobo) (hit bool, s float64) {
	dx := g.X2 - g.X1
	dy := g.Y2 - g.Y1
	// Horizontal normal to the footprint direction: n = (dy, -dx, 0).
	nx, ny := dy, -dx

	denom := nx*(b.X-a.X) + ny*(b.Y-a.Y)
	if math.Abs(denom) < 1e-12 {
		return false, 0 // segment parallel to gobo plane
	}
	t := (nx*(g.X1-a.X) + ny*(g.Y1-a.Y)) / denom
	if t < 0 || t > 1 {
		return false, 0 // intersection outside segment
	}
	q := geometry.Vec3{
		X: a.X + t*(b.X-a.X),
		Y: a.Y + t*(b.Y-a.Y),
		Z: a.Z + t*(b.Z-a.Z),
	}
	if q.Z < 0 || q.Z > g.Height {
		return false, 0 // outside vertical extent
	}
	dLen2 := dx*dx + dy*dy
	if dLen2 < 1e-12 {
		return false, 0 // degenerate (zero-length) gobo
	}
	s = ((q.X-g.X1)*dx + (q.Y-g.Y1)*dy) / dLen2
	if s < 0 || s > 1 {
		return false, 0 // outside horizontal extent
	}
	return true, s
}

// diffractionScalarSingle returns the Maekawa amplitude scalar [0,1] for a
// single gobo blocking segment A→B. Returns 1.0 if the gobo does not
// block the path.
//
// The diffraction point D is placed on the top edge of the gobo directly
// above the footprint crossing (standard first-order diffraction approximation).
// The Fresnel number N is evaluated at 1 kHz — a mid-band scalar that defers
// full per-octave-band treatment to M5 alongside per-band FIR filtering.
//
// Maekawa formula: attenuation_dB ≈ 10·log10(3 + 20·N), N = 2δ/λ
// where δ = |A→D| + |D→B| - |A→B| (extra path length of the diffracted route)
// and λ = c/f is the wavelength at 1 kHz.
//
// Characteristic values: N=0 (grazing) → 4.8 dB; N=1 → 13 dB; N=10 → 23 dB.
//
// References:
//   Maekawa (1968) Applied Acoustics 1(3):157–173 (original empirical formula).
//   Kuttruff, Room Acoustics 5th ed. §4.4 (derivation and worked examples).
//   Cowan, Architectural Acoustics Design Guide §6.3 (practical application).
// Accurate to ~2 dB for N > 0; standard for barrier insertion-loss estimation.
//
// TODO: Only over-the-top diffraction is modelled (D on the top edge). For
// better accuracy on paths that graze the vertical end edges, compute δ for
// the left end vertex, the right end vertex, and the top edge, then take the
// minimum across all three. This covers narrow gobos and low-angle paths.
//
// TODO: Gobos are currently treated as acoustically opaque (transmission
// loss = ∞). Real gobos have finite TL (typically 15–30 dB for plywood or
// rockwool, worse at low frequencies). To add material transparency:
//   1. Add a TL [7]float64 table to the material library alongside Absorption.
//   2. Per blocked path, compute Maekawa attenuation and TL per octave band.
//   3. Combine as incoherent energy sum: sqrt(10^(-TL/10) + Maekawa_linear²),
//      converted to an amplitude scalar.
// Deferred until PathContribution carries per-band gains (M5).
func diffractionScalarSingle(a, b geometry.Vec3, g scene.Gobo) float64 {
	hit, s := goboIntersects(a, b, g)
	if !hit {
		return 1.0
	}
	// Diffraction point D: top edge at footprint parameter s.
	d := geometry.Vec3{
		X: g.X1 + s*(g.X2-g.X1),
		Y: g.Y1 + s*(g.Y2-g.Y1),
		Z: g.Height,
	}
	// Path length difference δ = detour over the top edge vs. direct blocked path.
	delta := d.Sub(a).Length() + b.Sub(d).Length() - b.Sub(a).Length()
	if delta < 0 {
		delta = 0 // guard against floating-point underflow
	}
	lambda := speedOfSound / 1000.0 // wavelength at 1 kHz (mid-band)
	N := 2 * delta / lambda
	attenDB := 10 * math.Log10(3+20*N)
	return math.Pow(10, -attenDB/20)
}

// DiffractionScalar returns the cumulative Maekawa amplitude attenuation [0,1]
// for segment A→B due to all intersecting gobos. Returns 1.0 when gobos is
// nil or empty, or when no gobo blocks the path.
func DiffractionScalar(a, b geometry.Vec3, gobos []scene.Gobo) float64 {
	scalar := 1.0
	for _, g := range gobos {
		scalar *= diffractionScalarSingle(a, b, g)
	}
	return scalar
}

// mirrorGoboAcrossWall returns a copy of g with its footprint coordinates
// reflected across the given wall plane. Returns ok=false for floor and
// ceiling walls, which don't interact meaningfully with vertical gobo geometry.
func mirrorGoboAcrossWall(g scene.Gobo, wall int, room scene.Room) (scene.Gobo, bool) {
	m := g
	switch wall {
	case wallWest:
		m.X1, m.X2 = -g.X1, -g.X2
	case wallEast:
		m.X1, m.X2 = 2*room.Width-g.X1, 2*room.Width-g.X2
	case wallSouth:
		m.Y1, m.Y2 = -g.Y1, -g.Y2
	case wallNorth:
		m.Y1, m.Y2 = 2*room.Depth-g.Y1, 2*room.Depth-g.Y2
	default:
		return scene.Gobo{}, false
	}
	return m, true
}

// effectiveGobos returns the gobos to test against the image-source-to-mic
// segment for img.
//
// For 1st-order reflections (sum(wallHits) == 1), each gobo is also mirrored
// across the hit wall. The mirrored copy lands in image space (outside the
// room) and intercepts the image-source-to-mic segment exactly where the real
// reflected path would be blocked on the source-to-reflection-point leg.
// For higher-order reflections, only the original gobos are tested (covering
// the reflection-point-to-mic leg only).
//
// Limitations:
//   - 2nd-order and higher: only the mic-side leg is tested. Full coverage
//     requires tracking the ordered wall sequence, not just per-wall counts.
//   - Floor/ceiling reflections: excluded from mirroring.
//
// TODO: To support full gobo occlusion at all reflection orders, change
// imageSource.wallHits from a [6]int count array to a []int ordered wall
// sequence. With the sequence: (1) reconstruct each intermediate reflection
// point by intersecting the image-source-to-mic line with each wall in order;
// (2) test each sub-segment against appropriately mirrored gobos for that leg.
//
// TODO: For 2nd-order same-wall double-bounce (e.g. east→east), mirroring
// the gobo twice across the same axis is well-defined without ordered sequence
// tracking. The double-mirror places the gobo at an additional offset of
// 2*(W - gobo_x) further out, covering flutter-echo geometry.
func effectiveGobos(img imageSource, gobos []scene.Gobo, room scene.Room) []scene.Gobo {
	total := 0
	for _, h := range img.wallHits {
		total += h
	}
	if total != 1 {
		return gobos
	}
	hitWall := -1
	for i, h := range img.wallHits {
		if h > 0 {
			hitWall = i
			break
		}
	}
	result := make([]scene.Gobo, len(gobos), len(gobos)*2)
	copy(result, gobos)
	for _, g := range gobos {
		if mirrored, ok := mirrorGoboAcrossWall(g, hitWall, room); ok {
			result = append(result, mirrored)
		}
	}
	return result
}
