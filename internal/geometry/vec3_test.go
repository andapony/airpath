package geometry

import (
	"math"
	"testing"
)

func TestVec3Length(t *testing.T) {
	v := Vec3{3, 4, 0}
	if got := v.Length(); math.Abs(got-5.0) > 1e-9 {
		t.Errorf("Length() = %v, want 5.0", got)
	}
}

func TestVec3LengthZero(t *testing.T) {
	if got := (Vec3{}).Length(); got != 0 {
		t.Errorf("zero Length() = %v, want 0", got)
	}
}

func TestVec3Dot(t *testing.T) {
	x := Vec3{1, 0, 0}
	y := Vec3{0, 1, 0}
	if got := x.Dot(y); got != 0 {
		t.Errorf("orthogonal Dot() = %v, want 0", got)
	}
	if got := x.Dot(x); got != 1 {
		t.Errorf("self Dot() = %v, want 1", got)
	}
}

func TestVec3Normalize(t *testing.T) {
	v := Vec3{3, 4, 0}.Normalize()
	if l := v.Length(); math.Abs(l-1.0) > 1e-9 {
		t.Errorf("Normalize().Length() = %v, want 1.0", l)
	}
}

func TestVec3NormalizeZero(t *testing.T) {
	v := Vec3{}.Normalize()
	if v != (Vec3{}) {
		t.Errorf("Normalize(zero) = %v, want zero vector", v)
	}
}

func TestVec3Sub(t *testing.T) {
	a := Vec3{3, 2, 1}
	b := Vec3{1, 1, 1}
	got := a.Sub(b)
	want := Vec3{2, 1, 0}
	if got != want {
		t.Errorf("Sub() = %v, want %v", got, want)
	}
}
