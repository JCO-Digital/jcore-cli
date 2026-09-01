package project

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// requireNetwork skips the test if a quick HEAD request can't reach
// GitHub, so this suite doesn't hard-fail in a sandboxed/offline CI runner.
func requireNetwork(t *testing.T, url string) {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Head(url)
	if err != nil {
		t.Skipf("network unavailable, skipping: %v", err)
	}
	resp.Body.Close()
}

func TestCreateTheme(t *testing.T) {
	const url = "https://github.com/JCO-Digital/jcore-ilme/archive/refs/heads/{{branch}}.zip"
	requireNetwork(t, "https://github.com")

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte("theme := wp-content/themes/jcore-ilme\nother := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pnpm-workspace.yaml"), []byte("packages:\n    - 'wp-content/themes/jcore-ilme'\n"), 0644); err != nil {
		t.Fatal(err)
	}

	slug, err := CreateTheme(dir, "My Test Project", url, "hurricane")
	if err != nil {
		t.Fatalf("CreateTheme error = %v", err)
	}
	if slug != "my-test-project" {
		t.Fatalf("slug = %q, want %q", slug, "my-test-project")
	}

	themeDir := filepath.Join(dir, "wp-content", "themes", slug)
	styleCSS, err := os.ReadFile(filepath.Join(themeDir, "style.css"))
	if err != nil {
		t.Fatalf("expected style.css to exist: %v", err)
	}
	if !strings.Contains(string(styleCSS), "Theme Name: My Test Project") {
		t.Errorf("style.css does not contain the rewritten Theme Name; got:\n%s", styleCSS)
	}

	makefile, err := os.ReadFile(filepath.Join(dir, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(makefile), "theme := wp-content/themes/"+slug) {
		t.Errorf("Makefile theme path not updated; got:\n%s", makefile)
	}
	if !strings.Contains(string(makefile), "other := 1") {
		t.Errorf("Makefile lost unrelated content; got:\n%s", makefile)
	}

	workspace, err := os.ReadFile(filepath.Join(dir, "pnpm-workspace.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(workspace), "wp-content/themes/"+slug) {
		t.Errorf("pnpm-workspace.yaml theme path not updated; got:\n%s", workspace)
	}
}
