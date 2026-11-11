package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteShellRC(t *testing.T) {
	projectDir := t.TempDir()

	got, err := WriteShellRC(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "/project/.jcore/shell.rc" {
		t.Errorf("container path = %q", got)
	}

	path := filepath.Join(projectDir, ".jcore", "shell.rc")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, want := range []string{
		"[ -f ~/.bashrc ] && . ~/.bashrc\n",
		`opts="$(wp cli completions --line="$COMP_LINE" --point="$COMP_POINT" 2>/dev/null)"`,
		"complete -o nospace -F _wp_complete wp\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("shell.rc lacks %q:\n%s", want, text)
		}
	}

	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteShellRC(projectDir); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || !info.ModTime().Equal(old) {
		t.Errorf("unchanged shell.rc was rewritten")
	}

	if err := os.WriteFile(path, []byte("stale"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteShellRC(projectDir); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != text {
		t.Errorf("stale shell.rc was not rewritten: %q", got)
	}
}
