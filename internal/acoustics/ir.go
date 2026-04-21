package acoustics

// AssembleIR stamps each PathContribution into a zeroed buffer of lengthSamples.
// Contributions with DelaySamples outside [0, lengthSamples) are silently skipped.
func AssembleIR(contributions []PathContribution, lengthSamples int) []float64 {
	ir := make([]float64, lengthSamples)
	for _, c := range contributions {
		if c.DelaySamples >= 0 && c.DelaySamples < lengthSamples {
			ir[c.DelaySamples] += c.Amplitude
		}
	}
	return ir
}
