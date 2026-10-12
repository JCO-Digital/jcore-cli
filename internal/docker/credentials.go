package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// dockerHubServer is the registry key Docker looks Docker Hub credentials up
// by, and so what it asks the credential helper for before pulling a public
// image.
const dockerHubServer = "https://index.docker.io/v1/"

// CredentialHelperError means the credential helper configured as
// "credsStore" in the Docker config is missing or broken. Docker runs it
// before every image pull, even of public images, so every pull fails with
// "error getting credentials". Typical on WSL, where a config left over
// from Docker Desktop points at "desktop.exe".
type CredentialHelperError struct {
	// Store is the configured credsStore, e.g. "desktop.exe".
	Store string
	// ConfigPath is the Docker config file setting it.
	ConfigPath string
	// Problem says what's wrong with the helper.
	Problem string
}

func (e *CredentialHelperError) Error() string {
	return fmt.Sprintf("Docker's credential helper docker-credential-%s (\"credsStore\" in %s) %s.\n"+
		"Docker runs it before pulling any image, so pulls fail. Remove the \"credsStore\" line from %s\n"+
		"(or fix the helper, e.g. start Docker Desktop if it's set to \"desktop.exe\"), then try again.",
		e.Store, e.ConfigPath, e.Problem, e.ConfigPath)
}

// configPath returns the Docker CLI config file, honoring DOCKER_CONFIG.
func configPath() (string, error) {
	if dir := os.Getenv("DOCKER_CONFIG"); dir != "" {
		return filepath.Join(dir, "config.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".docker", "config.json"), nil
}

// CheckCredentialHelper verifies that the credential helper Docker uses, if
// any, works, by asking it for Docker Hub credentials the way Docker does
// before a pull. It returns a *CredentialHelperError if it doesn't, and nil
// when it works, when none is configured, or when the config can't be read
// (Docker reports that itself).
func CheckCredentialHelper() error {
	path, err := configPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var config struct {
		CredsStore string `json:"credsStore"`
	}
	if json.Unmarshal(data, &config) != nil || config.CredsStore == "" {
		return nil
	}

	fail := func(problem string) error {
		return &CredentialHelperError{Store: config.CredsStore, ConfigPath: path, Problem: problem}
	}

	helper := "docker-credential-" + config.CredsStore
	if _, err := exec.LookPath(helper); err != nil {
		return fail("isn't installed or on the PATH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, helper, "get")
	cmd.Stdin = strings.NewReader(dockerHubServer)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fail("didn't respond within 10 seconds")
	}
	output := strings.TrimSpace(string(out))
	// Not being logged in to Docker Hub is fine: Docker pulls anonymously.
	if strings.Contains(output, "credentials not found") {
		return nil
	}
	if output == "" {
		return fail(fmt.Sprintf("fails (%v, no output)", err))
	}
	return fail(fmt.Sprintf("fails (%v: %s)", err, output))
}

// WithCredentialHint adds an explanation to err, from a docker command that
// may have pulled images, if a broken credential helper is what made it
// fail.
func WithCredentialHint(err error) error {
	if err == nil {
		return nil
	}
	if cerr := CheckCredentialHelper(); cerr != nil {
		return fmt.Errorf("%w\n%v", err, cerr)
	}
	return err
}
