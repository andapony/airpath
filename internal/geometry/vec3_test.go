package geometry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestVec3Length verifies the Pythagorean length of a 3-4-0 vector,
// whose hypotenuse is exactly 5 by the 3-4-5 right triangle identity.
func TestVec3Length(t *testing.T) {
	v := Vec3{3, 4, 0}
	assert.InDelta(t, 5.0, v.Length(), 1e-9)
}

// TestVec3LengthZero verifies that the zero vector has length 0.
func TestVec3LengthZero(t *testing.T) {
	assert.Equal(t, float64(0), (Vec3{}).Length())
}

// TestVec3Dot verifies the dot product of orthogonal and parallel unit vectors.
// Orthogonal vectors must produce 0; a vector dotted with itself must produce 1.
func TestVec3Dot(t *testing.T) {
	x := Vec3{1, 0, 0}
	y := Vec3{0, 1, 0}
	assert.Equal(t, float64(0), x.Dot(y))
	assert.Equal(t, float64(1), x.Dot(x))
}

// TestVec3Normalize verifies that normalising a non-zero vector produces a unit vector.
func TestVec3Normalize(t *testing.T) {
	v := Vec3{3, 4, 0}.Normalize()
	assert.InDelta(t, 1.0, v.Length(), 1e-9)
}

// TestVec3NormalizeZero verifies that normalising the zero vector returns the
// zero vector rather than a NaN or infinity (defensive guard in Normalize).
func TestVec3NormalizeZero(t *testing.T) {
	assert.Equal(t, Vec3{}, Vec3{}.Normalize())
}

// TestVec3Sub verifies component-wise vector subtraction.
func TestVec3Sub(t *testing.T) {
	a := Vec3{3, 2, 1}
	b := Vec3{1, 1, 1}
	assert.Equal(t, Vec3{2, 1, 0}, a.Sub(b))
}
