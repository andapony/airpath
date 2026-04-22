package scene

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validSceneJSON = `{
	"version": 1,
	"sample_rate": 48000,
	"room": {
		"width": 5.0, "depth": 4.0, "height": 3.0,
		"surfaces": {
			"floor": "concrete", "ceiling": "acoustic_tile",
			"north": "drywall",  "south": "drywall",
			"east":  "brick",    "west":  "glass_window"
		}
	},
	"sources": [{"id": "src", "x": 1.0, "y": 1.0, "z": 1.0}],
	"mics": [{
		"id": "mic", "x": 3.0, "y": 3.0, "z": 1.5,
		"aim": {"azimuth": 180, "elevation": 0},
		"pattern": "cardioid"
	}]
}`

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scene.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

func TestParseValid(t *testing.T) {
	s, err := Parse(writeTemp(t, validSceneJSON))
	require.NoError(t, err)
	require.Len(t, s.Sources, 1)
	assert.Equal(t, "src", s.Sources[0].ID)
	assert.Equal(t, 5.0, s.Room.Width)
}

func TestParseUnknownMaterial(t *testing.T) {
	bad := `{
		"version": 1, "sample_rate": 48000,
		"room": {
			"width": 5.0, "depth": 4.0, "height": 3.0,
			"surfaces": {
				"floor": "moon_rock", "ceiling": "acoustic_tile",
				"north": "drywall", "south": "drywall",
				"east": "brick", "west": "glass_window"
			}
		},
		"sources": [{"id": "s", "x": 1, "y": 1, "z": 1}],
		"mics": [{"id": "m", "x": 2, "y": 2, "z": 1, "aim": {"azimuth": 0, "elevation": 0}, "pattern": "omni"}]
	}`
	_, err := Parse(writeTemp(t, bad))
	assert.Error(t, err)
}

func TestParseUnknownPattern(t *testing.T) {
	bad := `{
		"version": 1, "sample_rate": 48000,
		"room": {
			"width": 5.0, "depth": 4.0, "height": 3.0,
			"surfaces": {
				"floor": "concrete", "ceiling": "acoustic_tile",
				"north": "drywall", "south": "drywall",
				"east": "brick", "west": "glass_window"
			}
		},
		"sources": [{"id": "s", "x": 1, "y": 1, "z": 1}],
		"mics": [{"id": "m", "x": 2, "y": 2, "z": 1, "aim": {"azimuth": 0, "elevation": 0}, "pattern": "bidirectional"}]
	}`
	_, err := Parse(writeTemp(t, bad))
	assert.Error(t, err)
}

func TestParseNoSources(t *testing.T) {
	bad := `{
		"version": 1, "sample_rate": 48000,
		"room": {
			"width": 5.0, "depth": 4.0, "height": 3.0,
			"surfaces": {
				"floor": "concrete", "ceiling": "acoustic_tile",
				"north": "drywall", "south": "drywall",
				"east": "brick", "west": "glass_window"
			}
		},
		"sources": [],
		"mics": [{"id": "m", "x": 2, "y": 2, "z": 1, "aim": {"azimuth": 0, "elevation": 0}, "pattern": "omni"}]
	}`
	_, err := Parse(writeTemp(t, bad))
	assert.Error(t, err)
}
