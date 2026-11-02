package project

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/JCO-Digital/jcore/container"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func TestResolveComposeFiles(t *testing.T) {
	cases := []struct {
		name       string
		legacy     string // docker-compose.yml content, "" for none
		checksum   bool   // store docker-compose.yml's current checksum
		override   bool
		mounts     []string
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
			name:       "mounts win over a modified docker-compose.yml",
			legacy:     "services: {}\n",
			mounts:     []string{"./app:/var/www/html/app"},
			wantFiles:  []string{GeneratedComposeFile},
			wantNotice: "no longer used",
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
			viper.Reset()
			t.Cleanup(viper.Reset)
			if c.mounts != nil {
				viper.Set("mounts", c.mounts)
			}

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

func TestRenderComposeFile_Mounts(t *testing.T) {
	mounts := []string{"./app:/var/www/html/app", "./keys:/var/www/.keys:ro"}
	raw, err := renderComposeFile(mounts)
	if err != nil {
		t.Fatalf("renderComposeFile error = %v", err)
	}

	var got, want map[string]any
	if err := yaml.Unmarshal(raw, &got); err != nil {
		t.Fatalf("rendered file isn't valid YAML: %v\n%s", err, raw)
	}
	if err := yaml.Unmarshal(container.ComposeFile, &want); err != nil {
		t.Fatal(err)
	}

	// The mounts are on wordpress and web, and nothing else changed.
	for _, name := range mountServices {
		service := got["services"].(map[string]any)[name].(map[string]any)
		volumes := service["volumes"].([]any)
		n := len(volumes) - len(mounts)
		for i, mount := range mounts {
			if volumes[n+i] != mount {
				t.Errorf("%s volumes end with %v, want %v", name, volumes[n:], mounts)
			}
		}
		service["volumes"] = volumes[:n]
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rendering changed more than the mounts:\n%s", raw)
	}

	// The template's comments survive, the header included.
	if !strings.HasPrefix(string(raw), "# Generated by jcore") {
		t.Errorf("rendered file lost its header:\n%s", raw)
	}
}

func TestRenderComposeFile_NoMountsIsTemplate(t *testing.T) {
	raw, err := renderComposeFile(nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(container.ComposeFile) {
		t.Error("without mounts the generated file should be the template as is")
	}
}

func TestValidateMount(t *testing.T) {
	valid := []string{"./app:/var/www/html/app", "../shared:/shared", "./keys:/var/www/.keys:ro", "~/x:/x", "/abs:/abs"}
	invalid := []string{"app:/var/www/html/app", "./app", "./app:", ":/x", "./app:relative"}
	for _, m := range valid {
		if err := validateMount(m); err != nil {
			t.Errorf("validateMount(%q) = %v, want ok", m, err)
		}
	}
	for _, m := range invalid {
		if err := validateMount(m); err == nil {
			t.Errorf("validateMount(%q) = ok, want an error", m)
		}
	}
}

// The mounts' source folders get pre-created like the template's own.
func TestComposeMountedFolders_IncludesMounts(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("mounts", []string{"./vue/dist/static:/var/www/html/static"})

	dir := t.TempDir()
	if err := WriteComposeFile(dir); err != nil {
		t.Fatal(err)
	}
	folders, err := ComposeMountedFolders(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(folders, "./vue/dist/static") {
		t.Errorf("ComposeMountedFolders() = %v, want it to include ./vue/dist/static", folders)
	}
}
