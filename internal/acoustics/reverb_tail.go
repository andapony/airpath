package acoustics

import (
	"math"
	"math/rand"

	"github.com/andapony/airpath/internal/scene"
)

// SabineRT60 returns the estimated reverberation time (seconds) for room using
// the Sabine equation evaluated at the 1 kHz mid-band: RT60 = 0.161 × V / A,
// where V is the room volume (m³) and A = Σ(surface_area × α_1kHz).
//
// Returns 0 when the total absorption area A is non-positive (e.g. all surfaces
// are unknown materials, which default to α=0 — perfect reflectors).
//
// Limitations:
//   - Single mid-band scalar: real RT60 varies by frequency.
//   - Sabine assumes diffuse-field conditions (uniform absorption distribution).
//     Rooms with strongly uneven absorption are better modelled with Eyring's formula.
//   - Unknown materials are treated as α=0; the scene validator rejects them in
//     practice, so this should only be reached in unit tests with ad-hoc rooms.
func SabineRT60(room scene.Room) float64 {
	V := room.Width * room.Depth * room.Height

	floorCeiling := room.Width * room.Depth
	northSouth := room.Width * room.Height
	eastWest := room.Depth * room.Height

	const band1k = 3 // index of 1000 Hz in the 7-band array

	alpha := func(mat string) float64 {
		if a, ok := scene.KnownMaterials[mat]; ok {
			return a[band1k]
		}
		return 0
	}

	A := floorCeiling*(alpha(room.Surfaces.Floor)+alpha(room.Surfaces.Ceiling)) +
		northSouth*(alpha(room.Surfaces.North)+alpha(room.Surfaces.South)) +
		eastWest*(alpha(room.Surfaces.East)+alpha(room.Surfaces.West))

	if A <= 0 {
		return 0
	}
	return 0.161 * V / A
}

// GenerateReverbTail returns a buffer of lengthSamples with a synthetic diffuse
// reverb tail beginning at tailOnsetSamples. All samples before tailOnsetSamples
// are zero; the caller mixes this buffer into the assembled IR.
//
// The tail is shaped as follows (see inline step comments):
//  1. Gaussian white noise with a fixed seed (1) for reproducibility.
//  2. Exponential decay envelope reaching −60 dB at t=RT60.
//  3. One-pole IIR lowpass (~3 kHz) to simulate faster high-frequency decay.
//  4. 20 ms raised-cosine fade-in to suppress the click at onset.
//  5. RMS normalised to 1.0 over the 20 ms fade-in window so the engine can
//     scale the tail to match the IR energy at the onset point.
//
// Returns an all-zero buffer when tailOnsetSamples >= lengthSamples or when
// SabineRT60 returns 0 (all surfaces unknown — acoustically impossible room).
//
// Limitations:
//   - Single-band decay: a full model would use per-octave-band RT60 values
//     (deferred to M5 alongside per-band FIR filtering).
//   - Monophonic: no decorrelation between source-mic pairs.
func GenerateReverbTail(room scene.Room, sampleRate, lengthSamples, tailOnsetSamples int) []float64 {
	buf := make([]float64, lengthSamples)
	if tailOnsetSamples >= lengthSamples {
		return buf
	}

	rt60 := SabineRT60(room)
	if rt60 <= 0 {
		return buf
	}

	tail := buf[tailOnsetSamples:]

	// Step 1: Gaussian white noise with fixed seed for reproducibility.
	rng := rand.New(rand.NewSource(1))
	for i := range tail {
		tail[i] = rng.NormFloat64()
	}

	// Step 2: Exponential decay envelope — reaches −60 dB at t = RT60.
	decayRate := math.Log(1000) / (rt60 * float64(sampleRate))
	for i := range tail {
		tail[i] *= math.Exp(-decayRate * float64(i))
	}

	// Step 3: One-pole IIR lowpass (~3 kHz) to simulate faster HF decay.
	alpha := 1 - math.Exp(-2*math.Pi*3000/float64(sampleRate))
	var y float64
	for i := range tail {
		y = alpha*tail[i] + (1-alpha)*y
		tail[i] = y
	}

	// Step 4: Raised-cosine fade-in over the first 20 ms to avoid a click at onset.
	fadeIn := sampleRate / 50 // 20 ms
	if fadeIn > len(tail) {
		fadeIn = len(tail)
	}
	for i := 0; i < fadeIn; i++ {
		tail[i] *= 0.5 * (1 - math.Cos(math.Pi*float64(i)/float64(fadeIn)))
	}

	// Step 5: Normalize so the RMS of the fade-in window is 1.0.
	var sumSq float64
	for _, v := range tail[:fadeIn] {
		sumSq += v * v
	}
	if rms := math.Sqrt(sumSq / float64(fadeIn)); rms > 1e-10 {
		for i := range tail {
			tail[i] /= rms
		}
	}

	return buf
}
