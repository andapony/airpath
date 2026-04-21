package scene

import (
	"encoding/json"
	"fmt"
	"os"
)

var validPatterns = map[string]bool{
	"omni": true, "cardioid": true, "supercardioid": true, "figure8": true,
}

// Parse reads and validates a scene JSON file.
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
