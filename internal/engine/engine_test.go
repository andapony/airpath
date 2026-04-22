package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSmallRoom(t *testing.T) {
	outDir := t.TempDir()

	err := Run(Config{
		ScenePath: "../../examples/small_room.json",
		OutputDir: outDir,
		Duration:  1.0,
	})
	require.NoError(t, err)

	expected := []string{
		"guitar_to_guitar_close.wav",
		"guitar_to_room.wav",
		"vocal_to_guitar_close.wav",
		"vocal_to_room.wav",
	}
	for _, name := range expected {
		info, err := os.Stat(filepath.Join(outDir, name))
		if !assert.NoError(t, err, "expected output file %s", name) {
			continue
		}
		assert.Positive(t, info.Size(), "output file %s should not be empty", name)
	}
}

func TestRunMissingScene(t *testing.T) {
	err := Run(Config{
		ScenePath: "nonexistent.json",
		OutputDir: t.TempDir(),
		Duration:  1.0,
	})
	assert.Error(t, err)
}

func TestRunSmallRoom_WithReflections(t *testing.T) {
	outDir := t.TempDir()
	err := Run(Config{
		ScenePath:       "../../examples/small_room.json",
		OutputDir:       outDir,
		Duration:        1.0,
		ReflectionOrder: 1,
	})
	require.NoError(t, err)

	for _, name := range []string{
		"guitar_to_guitar_close.wav",
		"guitar_to_room.wav",
		"vocal_to_guitar_close.wav",
		"vocal_to_room.wav",
	} {
		info, err := os.Stat(filepath.Join(outDir, name))
		require.NoError(t, err, "expected output file %s", name)
		assert.Positive(t, info.Size(), "output file %s should not be empty", name)
	}
}
