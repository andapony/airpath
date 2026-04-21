package acoustics

import "testing"

func TestAssembleIRStampsContributions(t *testing.T) {
	contribs := []PathContribution{
		{DelaySamples: 10, Amplitude: 0.5},
		{DelaySamples: 20, Amplitude: 0.3},
	}
	ir := AssembleIR(contribs, 100)
	if ir[10] != 0.5 {
		t.Errorf("ir[10] = %v, want 0.5", ir[10])
	}
	if ir[20] != 0.3 {
		t.Errorf("ir[20] = %v, want 0.3", ir[20])
	}
	if ir[0] != 0 {
		t.Errorf("ir[0] = %v, want 0.0 (empty)", ir[0])
	}
}

func TestAssembleIRSkipsOutOfBounds(t *testing.T) {
	contribs := []PathContribution{
		{DelaySamples: 200, Amplitude: 1.0},
	}
	ir := AssembleIR(contribs, 100)
	for i, v := range ir {
		if v != 0 {
			t.Errorf("ir[%d] = %v, want 0 (out-of-bounds skipped)", i, v)
		}
	}
}

func TestAssembleIRAccumulates(t *testing.T) {
	// Two contributions at the same sample should sum.
	contribs := []PathContribution{
		{DelaySamples: 5, Amplitude: 0.4},
		{DelaySamples: 5, Amplitude: 0.3},
	}
	ir := AssembleIR(contribs, 100)
	if ir[5] != 0.7 {
		t.Errorf("ir[5] = %v, want 0.7 (accumulated)", ir[5])
	}
}

func TestAssembleIREmptyContributions(t *testing.T) {
	ir := AssembleIR(nil, 100)
	if len(ir) != 100 {
		t.Errorf("len(ir) = %d, want 100", len(ir))
	}
}
