package acoustics

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/andapony/airpath/internal/scene"
)

// TestSabineRT60_KnownRoom computes the Sabine RT60 for the small_room.json
// dimensions and surface materials, then checks the result against the
// hand-calculated value (V=56 m³, A=18.12 m² → RT60≈0.4976 s).
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

// TestSabineRT60_HighAbsorption verifies the direction of the RT60 response:
// a room lined with highly absorptive foam (α=0.80) must have a shorter RT60
// than the same room lined with brick (α=0.04).
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

// TestGenerateReverbTail_Length verifies that the returned buffer has exactly
// lengthSamples entries and that all samples before tailOnsetSamples are zero
// (the tail generator does not modify the pre-onset region).
func TestGenerateReverbTail_Length(t *testing.T) {
	room := scene.Room{
		Width: 5.0, Depth: 4.0, Height: 2.8,
		Surfaces: scene.Surfaces{
			Floor: "hardwood_floor", Ceiling: "acoustic_tile",
			North: "drywall", South: "drywall",
			East: "drywall", West: "glass_window",
		},
	}
	const sampleRate = 48000
	const length = 48000  // 1 second
	const onset = 3840    // 80 ms

	buf := GenerateReverbTail(room, sampleRate, length, onset)

	assert.Len(t, buf, length, "output length must equal lengthSamples")
	for i := 0; i < onset; i++ {
		assert.Equal(t, 0.0, buf[i], "sample %d before onset should be zero", i)
	}
}

// TestGenerateReverbTail_OnsetRMS verifies that the RMS of the 20 ms fade-in
// window (the normalisation target) is approximately 1.0. The engine relies on
// this invariant to correctly scale the tail by the IR energy at onset.
func TestGenerateReverbTail_OnsetRMS(t *testing.T) {
	room := scene.Room{
		Width: 5.0, Depth: 4.0, Height: 2.8,
		Surfaces: scene.Surfaces{
			Floor: "hardwood_floor", Ceiling: "acoustic_tile",
			North: "drywall", South: "drywall",
			East: "drywall", West: "glass_window",
		},
	}
	const sampleRate = 48000
	const onset = 3840 // 80 ms

	buf := GenerateReverbTail(room, sampleRate, sampleRate*2, onset)

	// RMS of the first 20 ms of the tail (the fade-in window) should be ≈ 1.0.
	fadeIn := sampleRate / 50 // 960 samples = 20 ms
	var sumSq float64
	for _, v := range buf[onset : onset+fadeIn] {
		sumSq += v * v
	}
	rms := math.Sqrt(sumSq / float64(fadeIn))
	assert.InDelta(t, 1.0, rms, 0.1, "onset RMS should be approximately 1.0")
}

// TestGenerateReverbTail_Decays verifies that the tail level at t=RT60 is at
// least 55 dB below the post-ramp reference level (design target: −60 dB;
// 5 dB allowance for IIR filter transients). Uses an all-plaster room
// (α_1kHz=0.04) so RT60≈2.567 s — long enough that the 20 ms fade-in is
// negligible when comparing the two measurement windows.
func TestGenerateReverbTail_Decays(t *testing.T) {
	// Use an all-plaster room: RT60 ≈ 2.57 s — long enough that the 20 ms
	// fade-in is negligible when measuring the RT60 decay ratio.
	// plaster α_1kHz = 0.04; 4×5×3 room: V=60, A=94×0.04=3.76, RT60=2.567 s.
	room := scene.Room{
		Width: 4.0, Depth: 5.0, Height: 3.0,
		Surfaces: scene.Surfaces{
			Floor: "plaster", Ceiling: "plaster",
			North: "plaster", South: "plaster",
			East: "plaster", West: "plaster",
		},
	}
	const sampleRate = 48000
	const onset = 3840 // 80 ms

	rt60 := SabineRT60(room) // ≈ 2.567 s
	length := onset + int(rt60*float64(sampleRate)) + sampleRate/50 + sampleRate
	buf := GenerateReverbTail(room, sampleRate, length, onset)

	fadeIn := sampleRate / 50 // 960 samples = 20 ms
	window := fadeIn

	// Reference: RMS of the 20 ms window just after the fade-in completes.
	refStart := onset + fadeIn
	var sumRef float64
	for _, v := range buf[refStart : refStart+window] {
		sumRef += v * v
	}
	rmsRef := math.Sqrt(sumRef / float64(window))
	require.Positive(t, rmsRef)

	// Decay window: 20 ms starting at onset + RT60.
	rt60Start := onset + int(rt60*float64(sampleRate))
	require.LessOrEqualf(t, rt60Start+window, len(buf),
		"buffer too short: need %d samples, have %d", rt60Start+window, len(buf))
	var sumDecay float64
	for _, v := range buf[rt60Start : rt60Start+window] {
		sumDecay += v * v
	}
	rmsDecay := math.Sqrt(sumDecay / float64(window))

	// At t=RT60 from onset, level should be well below the post-ramp reference.
	// The design target is −60 dB; we allow −55 dB to account for filter transients.
	threshold := rmsRef * math.Pow(10, -55.0/20.0)
	assert.LessOrEqual(t, rmsDecay, threshold,
		"RMS at t=RT60 should be ≤ −55 dB of post-ramp reference (design: −60 dB)")
}
