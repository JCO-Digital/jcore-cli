package project

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveComposeFiles(t *testing.T) {
	cases := []struct {
		name       string
		legacy     string // docker-compose.yml content, "" for none
		checksum   bool   // store docker-compose.yml's current checksum
		override   bool
		wantFiles  []string
		wantNotice string // substring, "" for no notice
	}{
		{
			name:      "new project",
			wantFiles: []string{GeneratedComposeFile},
		},
		{
			name:       "unmodified docker-compose.yml is ignored",
			legacy:     "services: {}\n",
			checksum:   true,
			wantFiles:  []string{GeneratedComposeFile},
			wantNotice: "can be deleted",
		},
		{
			name:       "modified docker-compose.yml stays in use",
			legacy:     "services: {}\n",
			wantFiles:  []string{LegacyComposeFile},
			wantNotice: "Warning: docker-compose.yml has local changes",
		},
		{
			name:      "override",
			override:  true,
			wantFiles: []string{GeneratedComposeFile, ComposeOverrideFile},
		},
		{
			name:       "override wins over a modified docker-compose.yml",
			legacy:     "services: {}\n",
			override:   true,
			wantFiles:  []string{GeneratedComposeFile, ComposeOverrideFile},
			wantNotice: "no longer used",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			if c.legacy != "" {
				if err := os.WriteFile(filepath.Join(dir, LegacyComposeFile), []byte(c.legacy), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if c.checksum {
				if err := UpdateChecksum(dir, LegacyComposeFile); err != nil {
					t.Fatal(err)
				}
			}
			if c.override {
				if err := os.WriteFile(filepath.Join(dir, ComposeOverrideFile), []byte("services: {}\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}

			setup, err := ResolveComposeFiles(dir)
			if err != nil {
				t.Fatalf("ResolveComposeFiles error = %v", err)
			}
			if !reflect.DeepEqual(setup.Files, c.wantFiles) {
				t.Errorf("Files = %v, want %v", setup.Files, c.wantFiles)
			}
			if c.wantNotice == "" && setup.Notice != "" {
				t.Errorf("Notice = %q, want none", setup.Notice)
			}
			if !strings.Contains(setup.Notice, c.wantNotice) {
				t.Errorf("Notice = %q, want it to mention %q", setup.Notice, c.wantNotice)
			}
		})
	}
}

// An override adding a network to a service must add to the networks the
// generated file gives it, not replace them, as compose merges them.
func TestReadComposeServices_MergesFiles(t *testing.T) {
	dir := t.TempDir()
	base := "services:\n  web:\n    networks:\n      - default\n  db: {}\n"
	override := "services:\n  web:\n    networks:\n      extra: {}\n  solr: {}\n"
	if err := os.WriteFile(filepath.Join(dir, "base.yml"), []byte(base), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "override.yml"), []byte(override), 0644); err != nil {
		t.Fatal(err)
	}

	services, err := readComposeServices(dir, []string{"base.yml", "override.yml"})
	if err != nil {
		t.Fatalf("readComposeServices error = %v", err)
	}
	want := map[string][]string{
		"web":  {"default", "extra"},
		"db":   nil,
		"solr": nil,
	}
	if !reflect.DeepEqual(services, want) {
		t.Errorf("services = %v, want %v", services, want)
	}
}

func TestRunLegacyPullScript_MissingScript(t *testing.T) {
	err := runLegacyPullScript(t.TempDir(), "importdb", "dbPullLegacy")
	if err == nil {
		t.Fatal("expected an error for a project without the legacy scripts")
	}
	for _, want := range []string{"importdb not found", "dbPullLegacy"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q doesn't mention %q", err, want)
		}
	}
}
