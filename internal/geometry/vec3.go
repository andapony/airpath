package geometry

import "math"

// Vec3 is a 3D vector with float64 components.
// All spatial coordinates in airpath are in metres. Vec3 is used for
// source/mic positions, image-source positions, direction vectors, and
// the intermediate geometry of path-length calculations.
type Vec3 struct {
	X, Y, Z float64
}

// Add returns the vector sum v + u.
func (v Vec3) Add(u Vec3) Vec3 {
	return Vec3{v.X + u.X, v.Y + u.Y, v.Z + u.Z}
}

// Sub returns the vector difference v − u.
func (v Vec3) Sub(u Vec3) Vec3 {
	return Vec3{v.X - u.X, v.Y - u.Y, v.Z - u.Z}
}

// Scale returns the vector v scaled by scalar s.
func (v Vec3) Scale(s float64) Vec3 {
	return Vec3{v.X * s, v.Y * s, v.Z * s}
}

// Dot returns the dot product of v and u.
// When both vectors are unit length, Dot returns cos(θ) where θ is
// the angle between them — used throughout polar pattern and diffraction
// calculations.
func (v Vec3) Dot(u Vec3) float64 {
	return v.X*u.X + v.Y*u.Y + v.Z*u.Z
}

// Length returns the Euclidean length of v.
func (v Vec3) Length() float64 {
	return math.Sqrt(v.Dot(v))
}

// Normalize returns a unit vector in the direction of v.
// Returns the zero vector when v has zero length (i.e. when the caller
// passes coincident points); callers that depend on a valid direction
// must guard against this before calling.
func (v Vec3) Normalize() Vec3 {
	l := v.Length()
	if l == 0 {
		return Vec3{}
	}
	return v.Scale(1 / l)
}
