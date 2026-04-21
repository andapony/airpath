package output

import (
	"encoding/binary"
	"math"
	"os"
	"testing"
)

func TestWriteWAVHeader(t *testing.T) {
	samples := []float64{0.5, -0.5, 1.0, 0.0}
	path := t.TempDir() + "/test.wav"

	if err := WriteWAV(path, samples, 48000); err != nil {
		t.Fatalf("WriteWAV failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading WAV: %v", err)
	}

	if string(data[0:4]) != "RIFF" {
		t.Errorf("chunk ID: got %q, want RIFF", data[0:4])
	}
	if string(data[8:12]) != "WAVE" {
		t.Errorf("format: got %q, want WAVE", data[8:12])
	}
	if string(data[12:16]) != "fmt " {
		t.Errorf("subchunk1 ID: got %q, want 'fmt '", data[12:16])
	}

	audioFormat := binary.LittleEndian.Uint16(data[20:22])
	if audioFormat != 3 {
		t.Errorf("AudioFormat = %d, want 3 (IEEE float)", audioFormat)
	}
	channels := binary.LittleEndian.Uint16(data[22:24])
	if channels != 1 {
		t.Errorf("NumChannels = %d, want 1 (mono)", channels)
	}
	sr := binary.LittleEndian.Uint32(data[24:28])
	if sr != 48000 {
		t.Errorf("SampleRate = %d, want 48000", sr)
	}
	bps := binary.LittleEndian.Uint16(data[34:36])
	if bps != 32 {
		t.Errorf("BitsPerSample = %d, want 32", bps)
	}
}

func TestWriteWAVSamples(t *testing.T) {
	samples := []float64{0.5, -0.25}
	path := t.TempDir() + "/samples.wav"

	if err := WriteWAV(path, samples, 44100); err != nil {
		t.Fatalf("WriteWAV failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading WAV: %v", err)
	}

	// First sample starts at byte 44.
	s0 := math.Float32frombits(binary.LittleEndian.Uint32(data[44:48]))
	if math.Abs(float64(s0)-0.5) > 1e-6 {
		t.Errorf("sample[0] = %v, want 0.5", s0)
	}
	s1 := math.Float32frombits(binary.LittleEndian.Uint32(data[48:52]))
	if math.Abs(float64(s1)-(-0.25)) > 1e-6 {
		t.Errorf("sample[1] = %v, want -0.25", s1)
	}
}

func TestWriteWAVFileSize(t *testing.T) {
	samples := make([]float64, 100)
	path := t.TempDir() + "/size.wav"

	if err := WriteWAV(path, samples, 48000); err != nil {
		t.Fatalf("WriteWAV failed: %v", err)
	}

	info, _ := os.Stat(path)
	// 44 header bytes + 100 samples * 4 bytes each = 444
	if info.Size() != 444 {
		t.Errorf("file size = %d, want 444", info.Size())
	}
}
