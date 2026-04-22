package acoustics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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

func TestAssembleIRSkipsOutOfBounds(t *testing.T) {
	contribs := []PathContribution{
		{DelaySamples: 200, Amplitude: 1.0},
	}
	ir := AssembleIR(contribs, 100)
	for i, v := range ir {
		assert.Zero(t, v, "ir[%d] should be zero (out-of-bounds skipped)", i)
	}
}

func TestAssembleIRAccumulates(t *testing.T) {
	contribs := []PathContribution{
		{DelaySamples: 5, Amplitude: 0.4},
		{DelaySamples: 5, Amplitude: 0.3},
	}
	ir := AssembleIR(contribs, 100)
	assert.InDelta(t, 0.7, ir[5], 1e-9)
}

func TestAssembleIREmptyContributions(t *testing.T) {
	ir := AssembleIR(nil, 100)
	assert.Len(t, ir, 100)
}
