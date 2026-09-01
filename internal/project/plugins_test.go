package project

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInstallGithubPlugin(t *testing.T) {
	requireNetwork(t, "https://github.com")

	dir := t.TempDir()
	name, err := InstallGithubPlugin(dir, "https://github.com/JCO-Digital/jcore-dynamic-archive/releases/latest/download/jcore-dynamic-archive.zip")
	if err != nil {
		t.Fatalf("InstallGithubPlugin error = %v", err)
	}
	if name != "jcore-dynamic-archive" {
		t.Fatalf("name = %q, want %q", name, "jcore-dynamic-archive")
	}

	pluginDir := filepath.Join(dir, "wp-content", "plugins", name)
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		t.Fatalf("expected plugin files at %s: %v", pluginDir, err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one file/folder in the installed plugin directory")
	}
}

func TestInstallGithubPlugin_RejectsURLWithNoAssetName(t *testing.T) {
	dir := t.TempDir()
	if _, err := InstallGithubPlugin(dir, ""); err == nil {
		t.Fatal("expected an error for a URL with no asset filename")
	}
}

func TestMergeUnique(t *testing.T) {
	cases := []struct {
		base, add, want []string
	}{
		{[]string{"lohko"}, []string{"jcore-turva"}, []string{"lohko", "jcore-turva"}},
		{[]string{"lohko"}, []string{"lohko"}, []string{"lohko"}},
		{nil, []string{"a", "a", "b"}, []string{"a", "b"}},
		{[]string{"a"}, nil, []string{"a"}},
	}
	for _, c := range cases {
		if got := MergeUnique(c.base, c.add); !reflect.DeepEqual(got, c.want) {
			t.Errorf("MergeUnique(%v, %v) = %v, want %v", c.base, c.add, got, c.want)
		}
	}
}
