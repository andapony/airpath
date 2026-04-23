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
