package output

import (
	"encoding/binary"
	"fmt"
	"os"
)

// WriteWAV writes samples as a mono 32-bit IEEE float WAV file.
func WriteWAV(path string, samples []float64, sampleRate int) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating WAV file: %w", err)
	}
	defer f.Close()

	numSamples := len(samples)
	dataSize := uint32(numSamples * 4) // 4 bytes per float32

	var writeErr error
	write := func(v any) {
		if writeErr != nil {
			return
		}
		writeErr = binary.Write(f, binary.LittleEndian, v)
	}
	writeBytes := func(b []byte) {
		if writeErr != nil {
			return
		}
		_, writeErr = f.Write(b)
	}

	// RIFF chunk descriptor
	writeBytes([]byte("RIFF"))
	write(uint32(36 + dataSize))
	writeBytes([]byte("WAVE"))

	// fmt sub-chunk
	writeBytes([]byte("fmt "))
	write(uint32(16))
	write(uint16(3))              // AudioFormat: 3 = IEEE float
	write(uint16(1))              // NumChannels: 1 = mono
	write(uint32(sampleRate))
	write(uint32(sampleRate * 4)) // ByteRate
	write(uint16(4))              // BlockAlign
	write(uint16(32))             // BitsPerSample

	// data sub-chunk
	writeBytes([]byte("data"))
	write(dataSize)

	for _, s := range samples {
		write(float32(s))
		if writeErr != nil {
			break
		}
	}

	if writeErr != nil {
		return fmt.Errorf("writing WAV data: %w", writeErr)
	}
	return nil
}
