package output

import (
	"encoding/binary"
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteWAVHeader(t *testing.T) {
	samples := []float64{0.5, -0.5, 1.0, 0.0}
	path := t.TempDir() + "/test.wav"

	require.NoError(t, WriteWAV(path, samples, 48000))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Equal(t, "RIFF", string(data[0:4]))
	assert.Equal(t, "WAVE", string(data[8:12]))
	assert.Equal(t, "fmt ", string(data[12:16]))
	assert.Equal(t, uint16(3), binary.LittleEndian.Uint16(data[20:22]), "AudioFormat should be IEEE float")
	assert.Equal(t, uint16(1), binary.LittleEndian.Uint16(data[22:24]), "NumChannels should be mono")
	assert.Equal(t, uint32(48000), binary.LittleEndian.Uint32(data[24:28]))
	assert.Equal(t, uint16(32), binary.LittleEndian.Uint16(data[34:36]), "BitsPerSample should be 32")
}

func TestWriteWAVSamples(t *testing.T) {
	samples := []float64{0.5, -0.25}
	path := t.TempDir() + "/samples.wav"

	require.NoError(t, WriteWAV(path, samples, 44100))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	s0 := math.Float32frombits(binary.LittleEndian.Uint32(data[44:48]))
	assert.InDelta(t, 0.5, float64(s0), 1e-6)

	s1 := math.Float32frombits(binary.LittleEndian.Uint32(data[48:52]))
	assert.InDelta(t, -0.25, float64(s1), 1e-6)
}

func TestWriteWAVFileSize(t *testing.T) {
	samples := make([]float64, 100)
	path := t.TempDir() + "/size.wav"

	require.NoError(t, WriteWAV(path, samples, 48000))

	info, err := os.Stat(path)
	require.NoError(t, err)
	// 44 header bytes + 100 samples * 4 bytes each = 444
	assert.Equal(t, int64(444), info.Size())
}
