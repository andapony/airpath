package acoustics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/andapony/airpath/internal/scene"
)

func TestSabineRT60_KnownRoom(t *testing.T) {
	// 5×4×2.8 m room matching examples/small_room.json
	room := scene.Room{
		Width: 5.0, Depth: 4.0, Height: 2.8,
		Surfaces: scene.Surfaces{
			Floor:   "hardwood_floor", // α_1kHz = 0.06
			Ceiling: "acoustic_tile",  // α_1kHz = 0.72
			North:   "drywall",        // α_1kHz = 0.03
			South:   "drywall",
			East:    "drywall",
			West:    "glass_window", // α_1kHz = 0.12
		},
	}
	// V = 5×4×2.8 = 56 m³
	// A = (5×4)×(0.06+0.72) + (5×2.8)×(0.03+0.03) + (4×2.8)×(0.03+0.12)
	//   = 20×0.78 + 14×0.06 + 11.2×0.15
	//   = 15.6 + 0.84 + 1.68 = 18.12 m²
	// RT60 = 0.161×56/18.12 ≈ 0.4976 s
	expected := 0.161 * 56.0 / 18.12
	got := SabineRT60(room)
	assert.InDelta(t, expected, got, 0.001)
}

func TestSabineRT60_HighAbsorption(t *testing.T) {
	room := scene.Room{Width: 4.0, Depth: 5.0, Height: 3.0}
	// acoustic_foam α_1kHz = 0.80; brick α_1kHz = 0.04
	room.Surfaces = scene.Surfaces{
		Floor: "acoustic_foam", Ceiling: "acoustic_foam",
		North: "acoustic_foam", South: "acoustic_foam",
		East: "acoustic_foam", West: "acoustic_foam",
	}
	rt60Foam := SabineRT60(room)

	room.Surfaces = scene.Surfaces{
		Floor: "brick", Ceiling: "brick",
		North: "brick", South: "brick",
		East: "brick", West: "brick",
	}
	rt60Brick := SabineRT60(room)

	assert.Greater(t, rt60Brick, rt60Foam,
		"low-absorption room should have longer RT60 than high-absorption room")
	require.Positive(t, rt60Foam)
	require.Positive(t, rt60Brick)
}
