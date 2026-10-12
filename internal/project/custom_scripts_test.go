package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasCustomScript(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. When custom-scripts dir doesn't exist
	if HasCustomScript(tmpDir, "db-before") {
		t.Errorf("expected false when custom-scripts does not exist")
	}

	// 2. When script is created
	scriptsDir := filepath.Join(tmpDir, "custom-scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("failed to create custom-scripts dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(scriptsDir, "db-before"), []byte("#!/bin/bash\necho hello"), 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	if !HasCustomScript(tmpDir, "db-before") {
		t.Errorf("expected true for existing db-before")
	}
	if HasCustomScript(tmpDir, "db-after") {
		t.Errorf("expected false for nonexistent db-after")
	}

	// 3. When path is a directory instead of a file
	if err := os.MkdirAll(filepath.Join(scriptsDir, "not-a-file"), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if HasCustomScript(tmpDir, "not-a-file") {
		t.Errorf("expected false when path is a directory")
	}
}
