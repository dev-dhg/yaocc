package skills

import (
	"path/filepath"
	"testing"
)

func TestLoader_Load(t *testing.T) {
	testDataDir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	loader := NewLoader([]string{testDataDir})
	skills, err := loader.Load()
	if err != nil {
		t.Fatalf("Loader.Load() error = %v", err)
	}

	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}

	skill := skills[0]
	if skill.Name != "myskill" {
		t.Errorf("expected name 'myskill', got '%s'", skill.Name)
	}
	if skill.Description != "A test skill for verification." {
		t.Errorf("expected description 'A test skill for verification.', got '%s'", skill.Description)
	}
	if skill.Homepage != "https://example.com" {
		t.Errorf("expected homepage 'https://example.com', got '%s'", skill.Homepage)
	}

	val, ok := skill.Metadata["test"]
	if !ok || val != true {
		t.Errorf("expected metadata test=true, got %v", val)
	}
}
