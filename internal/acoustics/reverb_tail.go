package acoustics

import (
	"math"
	"math/rand"

	"github.com/andapony/airpath/internal/scene"
)

// SabineRT60 returns the estimated reverberation time (seconds) for room using
// the Sabine equation at the 1 kHz mid-band: RT60 = 0.161 * V / A.
// A = Σ(surface_area × α_1kHz) for all six surfaces.
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

// GenerateReverbTail returns a buffer of lengthSamples with a synthetic reverb
// tail beginning at tailOnsetSamples. Samples before tailOnsetSamples are zero.
//
// The tail is shaped by the room's Sabine RT60 and a one-pole HF lowpass
// (~3 kHz cutoff) to simulate faster high-frequency decay. A 20 ms raised-cosine
// fade-in prevents a click at the onset. The onset window is normalized to RMS 1.0;
// the engine scales each pair's tail by the IR energy at the onset time.
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
