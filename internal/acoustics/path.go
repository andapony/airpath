package acoustics

// PathContribution represents a single acoustic path's contribution to an IR.
// A path is fully described by its travel time (in samples) and its amplitude
// at the microphone after accounting for distance, surface absorption, air
// absorption, polar pattern gain, and gobo diffraction.
//
// Multiple contributions can share the same DelaySamples value — their
// amplitudes are summed by AssembleIR, modelling the physical superposition
// of simultaneous arrivals. Negative amplitude values are valid in principle
// but do not arise in the current model (no phase inversion is modelled).
//
// Contributions whose DelaySamples falls outside [0, IR length) are
// discarded by AssembleIR. The engine does not pre-filter by IR duration,
// so very long IRs or low reflection orders can produce unreachable samples.
type PathContribution struct {
	DelaySamples int
	Amplitude    float64
}
