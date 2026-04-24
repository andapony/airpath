package scene

import (
	"encoding/json"
	"fmt"
	"os"
)

// validPatterns is the set of microphone polar pattern names accepted by the
// scene validator. Any mic.Pattern value not in this map causes validation to fail.
var validPatterns = map[string]bool{
	"omni": true, "cardioid": true, "supercardioid": true, "figure8": true,
}

// Parse reads a scene JSON file from path, unmarshals it into a Scene, and
// validates the result. Returns an error if the file cannot be read, if the
// JSON is malformed, or if the scene fails validation (unknown materials,
// unknown mic patterns, missing required fields, etc.).
//
// Assumptions: path is a readable file on the local filesystem. The JSON
// must conform to version 1 of the scene format (see docs/ for the spec).
func Parse(path string) (*Scene, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading scene file: %w", err)
	}
	var s Scene
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing scene JSON: %w", err)
	}
	if err := validate(&s); err != nil {
		return nil, fmt.Errorf("invalid scene: %w", err)
	}
	return &s, nil
}

// validate checks that s satisfies all constraints required by the acoustic
// engine. It does not check geometric constraints (e.g. sources outside the
// room) — those would silently produce correct but potentially surprising
// results (image sources outside the physical room are valid in the
// image-source method).
//
// Validated constraints:
//   - Version == 1
//   - SampleRate > 0
//   - Room dimensions all positive
//   - All surface materials in KnownMaterials
//   - At least one source and one mic
//   - All sources have non-empty IDs
//   - All mics have non-empty IDs and recognised polar patterns
//   - All gobo materials in KnownMaterials
func validate(s *Scene) error {
	if s.Version != 1 {
		return fmt.Errorf("unsupported version %d", s.Version)
	}
	if s.SampleRate <= 0 {
		return fmt.Errorf("sample_rate must be positive")
	}
	if s.Room.Width <= 0 || s.Room.Depth <= 0 || s.Room.Height <= 0 {
		return fmt.Errorf("room dimensions must be positive")
	}
	for _, mat := range []string{
		s.Room.Surfaces.Floor, s.Room.Surfaces.Ceiling,
		s.Room.Surfaces.North, s.Room.Surfaces.South,
		s.Room.Surfaces.East, s.Room.Surfaces.West,
	} {
		if _, ok := KnownMaterials[mat]; !ok {
			return fmt.Errorf("unknown material %q", mat)
		}
	}
	if len(s.Sources) == 0 {
		return fmt.Errorf("scene must have at least one source")
	}
	if len(s.Mics) == 0 {
		return fmt.Errorf("scene must have at least one mic")
	}
	for _, src := range s.Sources {
		if src.ID == "" {
			return fmt.Errorf("source missing id")
		}
	}
	for _, mic := range s.Mics {
		if mic.ID == "" {
			return fmt.Errorf("mic missing id")
		}
		if !validPatterns[mic.Pattern] {
			return fmt.Errorf("mic %q: unknown pattern %q", mic.ID, mic.Pattern)
		}
	}
	for _, gobo := range s.Gobos {
		if _, ok := KnownMaterials[gobo.Material]; !ok {
			return fmt.Errorf("gobo %q: unknown material %q", gobo.ID, gobo.Material)
		}
	}
	return nil
}
