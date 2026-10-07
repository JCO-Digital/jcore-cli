package project

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/JCO-Digital/jcore/internal/docker"
)

// HasCustomScript checks whether a custom script exists on the host.
func HasCustomScript(projectDir, scriptName string) bool {
	scriptPath := filepath.Join(projectDir, "custom-scripts", scriptName)
	info, err := os.Stat(scriptPath)
	return err == nil && !info.IsDir()
}

// RunCustomScript executes a custom script inside the wordpress container if it exists.
// Environment variables from /project/.env are loaded before running the script.
func RunCustomScript(projectDir, scriptName string) error {
	if !HasCustomScript(projectDir, scriptName) {
		return nil
	}

	fmt.Printf("Running custom script inside container: custom-scripts/%s\n", scriptName)
	cmd := fmt.Sprintf("export PROJECT_PATH=/project; if [ -f /project/.env ]; then set -a; . /project/.env; set +a; fi; bash /project/custom-scripts/%s", scriptName)
	if err := docker.ComposeExec(projectDir, "wordpress", []string{"bash", "-c", cmd}); err != nil {
		return fmt.Errorf("custom script custom-scripts/%s failed: %w", scriptName, err)
	}
	return nil
}
