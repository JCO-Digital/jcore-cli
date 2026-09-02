package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLohkoBlockTemplateNames(t *testing.T) {
	names, err := LohkoBlockTemplateNames()
	if err != nil {
		t.Fatalf("LohkoBlockTemplateNames error = %v", err)
	}

	want := map[string]bool{"dynamic": true, "static": true}
	if len(names) != len(want) {
		t.Fatalf("LohkoBlockTemplateNames() = %v, want %v", names, want)
	}
	for _, n := range names {
		if !want[n] {
			t.Errorf("unexpected template name %q", n)
		}
	}
}

func TestCreateBlock(t *testing.T) {
	dir := t.TempDir()
	destDir := filepath.Join(dir, "my-block")

	if err := CreateBlock(destDir, "dynamic", "My Block", "my-block", "A test block."); err != nil {
		t.Fatalf("CreateBlock error = %v", err)
	}

	// Every template file should have been copied, with ".tmpl" rendered
	// and its suffix dropped.
	for _, f := range []string{"block.json", "index.js", "edit.js", "editor.css", "render.php", "style.css", "view.twig"} {
		if _, err := os.Stat(filepath.Join(destDir, f)); err != nil {
			t.Errorf("expected %s to exist: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(destDir, "edit.js.tmpl")); err == nil {
		t.Error("expected the .tmpl suffix to be dropped from the destination filename")
	}

	// Rendered ".tmpl" files should have the slug substituted, not a
	// literal "{{.Slug}}".
	renderedCSS, err := os.ReadFile(filepath.Join(destDir, "editor.css"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(renderedCSS), ".wp-block-lohko-my-block") {
		t.Errorf("editor.css = %q, want slug substituted in", renderedCSS)
	}

	// view.twig has no template extension and its own unrelated
	// "{{ wrapper_attributes }}" tag (rendered later by Timber, not us) —
	// it must be copied byte-for-byte untouched.
	twig, err := os.ReadFile(filepath.Join(destDir, "view.twig"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(twig), "{{ wrapper_attributes }}") {
		t.Errorf("view.twig = %q, want its own template tag left untouched", twig)
	}

	// block.json must have title/name/description rewritten to match.
	raw, err := os.ReadFile(filepath.Join(destDir, "block.json"))
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data["title"] != "My Block" {
		t.Errorf("block.json title = %v, want %q", data["title"], "My Block")
	}
	if data["name"] != "lohko/my-block" {
		t.Errorf("block.json name = %v, want %q", data["name"], "lohko/my-block")
	}
	if data["description"] != "A test block." {
		t.Errorf("block.json description = %v, want %q", data["description"], "A test block.")
	}

	attrs, ok := data["attributes"].(map[string]any)
	if !ok || attrs["content"] == nil {
		t.Errorf("block.json attributes = %v, want a \"content\" attribute", data["attributes"])
	}
	if _, ok := attrs["test"]; ok {
		t.Error("block.json still has the placeholder \"test\" attribute")
	}
}

// TestCreateBlock_Static mirrors TestCreateBlock for the "static" template,
// which additionally has a save.js instead of dynamic's render.php/view.twig.
func TestCreateBlock_Static(t *testing.T) {
	dir := t.TempDir()
	destDir := filepath.Join(dir, "my-block")

	if err := CreateBlock(destDir, "static", "My Block", "my-block", "A test block."); err != nil {
		t.Fatalf("CreateBlock error = %v", err)
	}

	for _, f := range []string{"block.json", "index.js", "edit.js", "editor.css", "save.js", "style.css"} {
		if _, err := os.Stat(filepath.Join(destDir, f)); err != nil {
			t.Errorf("expected %s to exist: %v", f, err)
		}
	}
}

// TestCreateBlock_NoLeftoverPlaceholders guards against reintroducing the
// upstream template's placeholder "test" TextControl or console.debug
// messages into either block template.
func TestCreateBlock_NoLeftoverPlaceholders(t *testing.T) {
	for _, template := range []string{"dynamic", "static"} {
		dir := t.TempDir()
		destDir := filepath.Join(dir, "my-block")

		if err := CreateBlock(destDir, template, "My Block", "my-block", ""); err != nil {
			t.Fatalf("CreateBlock(%q) error = %v", template, err)
		}

		_ = filepath.WalkDir(destDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(content), "console.debug") {
				t.Errorf("%s: leftover console.debug message", path)
			}
			if strings.Contains(string(content), "Example Static") || strings.Contains(string(content), "Example Dynamic") {
				t.Errorf("%s: leftover placeholder example text", path)
			}
			return nil
		})
	}
}
