package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestUpdateProjectTargetedFileOverridesChecksumMismatch(t *testing.T) {
	dir := t.TempDir()

	viper.Reset()
	viper.Set("projectName", "test-project")
	viper.Set("theme", "jcore-ilme")
	viper.Set("template", "jcore3")
	t.Cleanup(viper.Reset)

	if err := ScaffoldProject(dir, "jcore3"); err != nil {
		t.Fatalf("ScaffoldProject failed: %v", err)
	}

	composePath := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte("# locally modified\n"), 0644); err != nil {
		t.Fatalf("failed to simulate a local edit: %v", err)
	}

	// Without targeting the file, the checksum mismatch should cause it to be skipped.
	if err := UpdateProject(dir, nil); err != nil {
		t.Fatalf("UpdateProject (untargeted) failed: %v", err)
	}
	content, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("failed to read docker-compose.yml: %v", err)
	}
	if string(content) != "# locally modified\n" {
		t.Errorf("expected untargeted update to skip the modified file, but it was overwritten")
	}

	// Explicitly targeting the file must force the overwrite regardless of checksum.
	if err := UpdateProject(dir, []string{"docker-compose.yml"}); err != nil {
		t.Fatalf("UpdateProject (targeted) failed: %v", err)
	}
	content, err = os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("failed to read docker-compose.yml: %v", err)
	}
	if string(content) == "# locally modified\n" {
		t.Errorf("expected targeting docker-compose.yml to force an overwrite, but it was left unchanged")
	}
}
