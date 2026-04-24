package output

import (
	"encoding/binary"
	"fmt"
	"os"
)

// WriteWAV writes samples as a mono 32-bit IEEE float WAV file at path.
// The output format is: RIFF/WAVE container, PCM format code 3 (IEEE float),
// 1 channel, 32 bits per sample, little-endian byte order.
//
// Samples are truncated from float64 to float32 — values outside [−1, 1]
// are preserved (no clipping); DAWs handle out-of-range IR values without
// distortion in offline convolution. The caller is responsible for ensuring
// the output path is writable and that parent directories exist.
//
// Limitation: only mono output is supported. Stereo or multi-channel IRs
// would require multiple files or a format change.
func WriteWAV(path string, samples []float64, sampleRate int) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating WAV file: %w", err)
	}
	defer f.Close()

	numSamples := len(samples)
	dataSize := uint32(numSamples * 4) // 4 bytes per float32 sample

	// write and writeBytes are local closures that accumulate the first error
	// and skip subsequent writes, avoiding repetitive error checks per field.
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

	// RIFF chunk descriptor: 4-byte chunk ID, 4-byte chunk size, 4-byte format.
	writeBytes([]byte("RIFF"))
	write(uint32(36 + dataSize)) // total file size minus the 8-byte RIFF header
	writeBytes([]byte("WAVE"))

	// fmt sub-chunk: describes the audio encoding.
	writeBytes([]byte("fmt "))
	write(uint32(16))             // sub-chunk size: always 16 for PCM/float
	write(uint16(3))              // AudioFormat: 3 = IEEE float (not 1 = PCM int)
	write(uint16(1))              // NumChannels: 1 = mono
	write(uint32(sampleRate))     // SampleRate in Hz
	write(uint32(sampleRate * 4)) // ByteRate = SampleRate × BlockAlign
	write(uint16(4))              // BlockAlign = NumChannels × BitsPerSample/8
	write(uint16(32))             // BitsPerSample: 32-bit float

	// data sub-chunk: raw sample data.
	writeBytes([]byte("data"))
	write(dataSize)

	for _, s := range samples {
		write(float32(s)) // narrowing float64→float32; ~7 decimal digits of precision
		if writeErr != nil {
			break
		}
	}

	if writeErr != nil {
		return fmt.Errorf("writing WAV data: %w", writeErr)
	}
	return nil
}
