package acoustics

// AssembleIR stamps each PathContribution into a zeroed float64 buffer of
// lengthSamples and returns it. Contributions with the same DelaySamples
// value accumulate — their amplitudes are added, modelling the superposition
// of simultaneous path arrivals.
//
// Contributions whose DelaySamples is outside [0, lengthSamples) are
// silently skipped; they represent paths whose travel time exceeds the IR
// duration. This is expected at high reflection orders or short durations.
//
// Assumption: contributions may have any non-negative amplitude; no clipping
// or normalisation is applied. The buffer is returned as-is for the engine
// to optionally mix a reverb tail into before writing to WAV.
func AssembleIR(contributions []PathContribution, lengthSamples int) []float64 {
	ir := make([]float64, lengthSamples)
	for _, c := range contributions {
		if c.DelaySamples >= 0 && c.DelaySamples < lengthSamples {
			ir[c.DelaySamples] += c.Amplitude
		}
	}
	return ir
}
