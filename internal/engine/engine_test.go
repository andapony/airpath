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
	outDirDirect := t.TempDir()
	require.NoError(t, Run(Config{
		ScenePath: "../../examples/small_room.json",
		OutputDir: outDirDirect,
		Duration:  1.0,
	}))

	outDirReflected := t.TempDir()
	require.NoError(t, Run(Config{
		ScenePath:       "../../examples/small_room.json",
		OutputDir:       outDirReflected,
		Duration:        1.0,
		ReflectionOrder: 1,
	}))

	expected := []string{
		"guitar_to_guitar_close.wav",
		"guitar_to_room.wav",
		"vocal_to_guitar_close.wav",
		"vocal_to_room.wav",
	}
	for _, name := range expected {
		info, err := os.Stat(filepath.Join(outDirReflected, name))
		if !assert.NoError(t, err, "expected output file %s", name) {
			continue
		}
		assert.Positive(t, info.Size(), "output file %s should not be empty", name)
	}

	// Verify reflections actually change the output for at least one file.
	directBytes, err := os.ReadFile(filepath.Join(outDirDirect, "guitar_to_room.wav"))
	require.NoError(t, err)
	reflectedBytes, err := os.ReadFile(filepath.Join(outDirReflected, "guitar_to_room.wav"))
	require.NoError(t, err)
	assert.NotEqual(t, directBytes, reflectedBytes, "guitar_to_room.wav should differ with reflections")
}

func TestRunSmallRoom_GoboChangesOutput(t *testing.T) {
	// Run the same scene with and without the gobo to verify the gobo actually
	// changes the output bytes for the blocked guitar→room pair.
	// small_room.json contains a gobo blocking guitar→room.

	outWithGobo := t.TempDir()
	require.NoError(t, Run(Config{
		ScenePath: "../../examples/small_room.json",
		OutputDir: outWithGobo,
		Duration:  1.0,
	}))

	// Write a temporary scene file with gobos removed.
	noGoboScene := `{
  "version": 1,
  "sample_rate": 48000,
  "room": {
    "width": 5.0,
    "depth": 4.0,
    "height": 2.8,
    "surfaces": {
      "floor":   "hardwood_floor",
      "ceiling": "acoustic_tile",
      "north":   "drywall",
      "south":   "drywall",
      "east":    "drywall",
      "west":    "glass_window"
    }
  },
  "sources": [
    { "id": "guitar", "x": 1.5, "y": 1.0, "z": 1.2 },
    { "id": "vocal",  "x": 3.5, "y": 2.0, "z": 1.5 }
  ],
  "mics": [
    {
      "id": "guitar_close",
      "x": 1.7, "y": 1.0, "z": 1.2,
      "aim": { "azimuth": 270, "elevation": 0 },
      "pattern": "cardioid"
    },
    {
      "id": "room",
      "x": 2.5, "y": 3.5, "z": 1.8,
      "aim": { "azimuth": 180, "elevation": -10 },
      "pattern": "omni"
    }
  ],
  "gobos": []
}`
	sceneFile := filepath.Join(t.TempDir(), "no_gobo.json")
	require.NoError(t, os.WriteFile(sceneFile, []byte(noGoboScene), 0644))

	outNoGobo := t.TempDir()
	require.NoError(t, Run(Config{
		ScenePath: sceneFile,
		OutputDir: outNoGobo,
		Duration:  1.0,
	}))

	withBytes, err := os.ReadFile(filepath.Join(outWithGobo, "guitar_to_room.wav"))
	require.NoError(t, err)
	withoutBytes, err := os.ReadFile(filepath.Join(outNoGobo, "guitar_to_room.wav"))
	require.NoError(t, err)
	assert.NotEqual(t, withBytes, withoutBytes, "guitar_to_room.wav should differ when gobo is present")
}

func TestRunSmallRoom_TailChangesOutput(t *testing.T) {
	outWithTail := t.TempDir()
	require.NoError(t, Run(Config{
		ScenePath:       "../../examples/small_room.json",
		OutputDir:       outWithTail,
		Duration:        1.0,
		ReflectionOrder: 1,
		TailEnabled:     true,
		TailOnset:       0.08,
	}))

	outNoTail := t.TempDir()
	require.NoError(t, Run(Config{
		ScenePath:       "../../examples/small_room.json",
		OutputDir:       outNoTail,
		Duration:        1.0,
		ReflectionOrder: 1,
		TailEnabled:     false,
	}))

	withBytes, err := os.ReadFile(filepath.Join(outWithTail, "guitar_to_room.wav"))
	require.NoError(t, err)
	withoutBytes, err := os.ReadFile(filepath.Join(outNoTail, "guitar_to_room.wav"))
	require.NoError(t, err)
	assert.NotEqual(t, withBytes, withoutBytes,
		"guitar_to_room.wav should differ when tail is enabled")
}
