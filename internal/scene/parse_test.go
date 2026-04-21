package scene

import (
	"os"
	"path/filepath"
	"testing"
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
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseValid(t *testing.T) {
	s, err := Parse(writeTemp(t, validSceneJSON))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(s.Sources) != 1 || s.Sources[0].ID != "src" {
		t.Errorf("unexpected sources: %v", s.Sources)
	}
	if s.Room.Width != 5.0 {
		t.Errorf("Room.Width = %v, want 5.0", s.Room.Width)
	}
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
	if err == nil {
		t.Error("expected error for unknown material, got nil")
	}
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
	if err == nil {
		t.Error("expected error for unknown pattern, got nil")
	}
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
	if err == nil {
		t.Error("expected error for empty sources, got nil")
	}
}
