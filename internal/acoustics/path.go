package acoustics

// PathContribution represents a single acoustic path's contribution to an IR.
// M1 produces one per source-mic pair (direct path only).
// Later milestones append more contributions (reflections, diffraction, reverb tail)
// without changing the IR assembler.
type PathContribution struct {
	DelaySamples int
	Amplitude    float64
}
