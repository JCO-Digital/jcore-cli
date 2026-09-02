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
	if err := UpdateProject(dir, nil, nil); err != nil {
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
	if err := UpdateProject(dir, []string{"docker-compose.yml"}, []string{"docker-compose.yml"}); err != nil {
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

func TestUpdateProjectForceWithoutOnlyStillUpdatesOtherFiles(t *testing.T) {
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

	readmePath := filepath.Join(dir, "readme.md")
	if err := os.Remove(readmePath); err != nil {
		t.Fatalf("failed to remove readme.md: %v", err)
	}

	modified, err := DetectModifiedFiles(dir)
	if err != nil {
		t.Fatalf("DetectModifiedFiles failed: %v", err)
	}
	if len(modified) != 1 || modified[0] != "docker-compose.yml" {
		t.Fatalf("expected DetectModifiedFiles to report only docker-compose.yml, got %v", modified)
	}

	// Force docker-compose.yml without restricting "only" - readme.md (missing,
	// unmodified) must still get recreated normally by the same update pass.
	if err := UpdateProject(dir, nil, []string{"docker-compose.yml"}); err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}

	content, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("failed to read docker-compose.yml: %v", err)
	}
	if string(content) == "# locally modified\n" {
		t.Errorf("expected forced docker-compose.yml to be overwritten")
	}

	if _, err := os.Stat(readmePath); err != nil {
		t.Errorf("expected readme.md to be recreated by the normal update pass, got: %v", err)
	}
}
