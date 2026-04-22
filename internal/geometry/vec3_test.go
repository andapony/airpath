package geometry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVec3Length(t *testing.T) {
	v := Vec3{3, 4, 0}
	assert.InDelta(t, 5.0, v.Length(), 1e-9)
}

func TestVec3LengthZero(t *testing.T) {
	assert.Equal(t, float64(0), (Vec3{}).Length())
}

func TestVec3Dot(t *testing.T) {
	x := Vec3{1, 0, 0}
	y := Vec3{0, 1, 0}
	assert.Equal(t, float64(0), x.Dot(y))
	assert.Equal(t, float64(1), x.Dot(x))
}

func TestVec3Normalize(t *testing.T) {
	v := Vec3{3, 4, 0}.Normalize()
	assert.InDelta(t, 1.0, v.Length(), 1e-9)
}

func TestVec3NormalizeZero(t *testing.T) {
	assert.Equal(t, Vec3{}, Vec3{}.Normalize())
}

func TestVec3Sub(t *testing.T) {
	a := Vec3{3, 2, 1}
	b := Vec3{1, 1, 1}
	assert.Equal(t, Vec3{2, 1, 0}, a.Sub(b))
}
