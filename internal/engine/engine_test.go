package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunSmallRoom(t *testing.T) {
	outDir := t.TempDir()

	if err := Run(Config{
		ScenePath: "../../examples/small_room.json",
		OutputDir: outDir,
		Duration:  1.0,
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// 2 sources × 2 mics = 4 output files
	expected := []string{
		"guitar_to_guitar_close.wav",
		"guitar_to_room.wav",
		"vocal_to_guitar_close.wav",
		"vocal_to_room.wav",
	}
	for _, name := range expected {
		path := filepath.Join(outDir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected output file missing: %s", name)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("output file is empty: %s", name)
		}
	}
}

func TestRunMissingScene(t *testing.T) {
	err := Run(Config{
		ScenePath: "nonexistent.json",
		OutputDir: t.TempDir(),
		Duration:  1.0,
	})
	if err == nil {
		t.Error("expected error for missing scene file, got nil")
	}
}
