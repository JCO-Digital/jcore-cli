package docker

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupCredentials points Docker's config at a temp dir holding config, and
// PATH at a dir holding docker-credential-test running script (if any).
func setupCredentials(t *testing.T, config, script string) {
	t.Helper()
	configDir := t.TempDir()
	if config != "" {
		if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(config), 0644); err != nil {
			t.Fatal(err)
		}
	}
	binDir := t.TempDir()
	if script != "" {
		helper := filepath.Join(binDir, "docker-credential-test")
		if err := os.WriteFile(helper, []byte("#!/bin/sh\n"+script+"\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("DOCKER_CONFIG", configDir)
	t.Setenv("PATH", binDir)
}

func TestCheckCredentialHelper(t *testing.T) {
	const withStore = `{"credsStore": "test"}`
	tests := []struct {
		name    string
		config  string
		script  string
		problem string // "" when no error is expected
	}{
		{name: "no config"},
		{name: "no credsStore", config: `{"auths": {}}`},
		{name: "invalid config", config: `{`},
		{name: "working helper", config: withStore, script: `echo '{"Username":"u","Secret":"s"}'`},
		{name: "not logged in", config: withStore, script: "echo 'credentials not found in native keychain'; exit 1"},
		{name: "missing helper", config: withStore, problem: "isn't installed"},
		{name: "silently failing helper", config: withStore, script: "exit 1", problem: "fails (exit status 1, no output)"},
		{name: "failing helper with output", config: withStore, script: "echo boom; exit 1", problem: "fails (exit status 1: boom)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupCredentials(t, tt.config, tt.script)
			err := CheckCredentialHelper()
			if tt.problem == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			var cerr *CredentialHelperError
			if !errors.As(err, &cerr) {
				t.Fatalf("expected a *CredentialHelperError, got %v", err)
			}
			if cerr.Store != "test" || !strings.Contains(cerr.Problem, tt.problem) {
				t.Errorf("got store %q, problem %q; want store \"test\", problem containing %q", cerr.Store, cerr.Problem, tt.problem)
			}
		})
	}
}

func TestWithCredentialHint(t *testing.T) {
	setupCredentials(t, `{"credsStore": "test"}`, "exit 1")
	base := errors.New("exit status 1")

	if WithCredentialHint(nil) != nil {
		t.Error("expected nil for a nil error")
	}
	err := WithCredentialHint(base)
	if !errors.Is(err, base) || !strings.Contains(err.Error(), "credsStore") {
		t.Errorf("expected the hint wrapped around the original error, got %v", err)
	}

	setupCredentials(t, "", "")
	if err := WithCredentialHint(base); err != base {
		t.Errorf("expected the error unchanged with no helper configured, got %v", err)
	}
}
