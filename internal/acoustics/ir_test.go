package acoustics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAssembleIRStampsContributions verifies that each PathContribution's
// Amplitude lands at the correct sample index and that all other samples
// remain zero.
func TestAssembleIRStampsContributions(t *testing.T) {
	contribs := []PathContribution{
		{DelaySamples: 10, Amplitude: 0.5},
		{DelaySamples: 20, Amplitude: 0.3},
	}
	ir := AssembleIR(contribs, 100)
	assert.Equal(t, 0.5, ir[10])
	assert.Equal(t, 0.3, ir[20])
	assert.Zero(t, ir[0])
}

// TestAssembleIRSkipsOutOfBounds confirms that contributions whose DelaySamples
// exceeds the buffer length are silently discarded, leaving all samples zero.
func TestAssembleIRSkipsOutOfBounds(t *testing.T) {
	contribs := []PathContribution{
		{DelaySamples: 200, Amplitude: 1.0},
	}
	ir := AssembleIR(contribs, 100)
	for i, v := range ir {
		assert.Zero(t, v, "ir[%d] should be zero (out-of-bounds skipped)", i)
	}
}

// TestAssembleIRAccumulates checks that multiple contributions at the same
// sample index are summed (superposition of simultaneous arrivals).
func TestAssembleIRAccumulates(t *testing.T) {
	contribs := []PathContribution{
		{DelaySamples: 5, Amplitude: 0.4},
		{DelaySamples: 5, Amplitude: 0.3},
	}
	ir := AssembleIR(contribs, 100)
	assert.InDelta(t, 0.7, ir[5], 1e-9)
}

// TestAssembleIREmptyContributions verifies that a nil contributions slice
// returns an all-zero buffer of the requested length.
func TestAssembleIREmptyContributions(t *testing.T) {
	ir := AssembleIR(nil, 100)
	assert.Len(t, ir, 100)
}
