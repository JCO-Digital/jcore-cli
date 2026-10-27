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
}
