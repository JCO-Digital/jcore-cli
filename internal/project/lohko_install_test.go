package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLohkoInstalled(t *testing.T) {
	dir := t.TempDir()
	if LohkoInstalled(dir) {
		t.Fatal("LohkoInstalled() = true for a project with no Lohko plugin")
	}

	if err := os.MkdirAll(filepath.Join(dir, LohkoPluginRelPath), 0755); err != nil {
		t.Fatal(err)
	}
	if !LohkoInstalled(dir) {
		t.Fatal("LohkoInstalled() = false after creating the plugin directory")
	}
}

func TestInstallLohkoAndBundledBlocks(t *testing.T) {
	requireNetwork(t, "https://github.com")

	dir := t.TempDir()
	if err := InstallLohko(dir); err != nil {
		t.Fatalf("InstallLohko error = %v", err)
	}
	if !LohkoInstalled(dir) {
		t.Fatal("expected LohkoInstalled() to be true after InstallLohko")
	}

	if _, err := os.Stat(filepath.Join(dir, LohkoPluginRelPath, "src")); err != nil {
		t.Fatalf("expected a src/ directory in the installed plugin: %v", err)
	}

	blocks, err := LohkoBundledBlocks(dir)
	if err != nil {
		t.Fatalf("LohkoBundledBlocks error = %v", err)
	}
	if len(blocks) == 0 {
		t.Fatal("expected at least one bundled example block")
	}
	for _, b := range blocks {
		if b.Folder == "" || b.Title == "" {
			t.Errorf("bundled block missing Folder/Title: %#v", b)
		}
	}

	// Remove one and confirm it's actually gone and no longer listed.
	toRemove := blocks[0]
	if err := RemoveLohkoBlock(dir, toRemove.Folder); err != nil {
		t.Fatalf("RemoveLohkoBlock error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, LohkoPluginRelPath, "src", toRemove.Folder)); err == nil {
		t.Fatalf("expected %s to be removed", toRemove.Folder)
	}

	remaining, err := LohkoBundledBlocks(dir)
	if err != nil {
		t.Fatalf("LohkoBundledBlocks error = %v", err)
	}
	if len(remaining) != len(blocks)-1 {
		t.Fatalf("LohkoBundledBlocks() after removal = %d blocks, want %d", len(remaining), len(blocks)-1)
	}
	for _, b := range remaining {
		if b.Folder == toRemove.Folder {
			t.Fatalf("removed block %q still listed", toRemove.Folder)
		}
	}
}
