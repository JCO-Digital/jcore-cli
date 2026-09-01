package project

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestBindMountSource(t *testing.T) {
	cases := []struct {
		entry     string
		wantSrc   string
		wantMatch bool
	}{
		{"./.jcore/ssl:/home/jcore/ssl", "./.jcore/ssl", true},
		{"./wp-content:/var/www/html/wp-content", "./wp-content", true},
		{"  ./vendor:/var/www/html/vendor  ", "./vendor", true},
		{".:/project", "", false},                                                // project root itself
		{"db:/var/lib/mysql", "", false},                                         // named volume
		{"~/.config/jcore/ssl:/home/jcore/ca", "", false},                        // global, not project-relative
		{"/absolute/path:/target", "", false},                                    // absolute
		{"../escaping:/target", "", false},                                       // escapes project root
		{"${SSH_AUTH_SOCK:-/dev/null}:/run/user/1000/sock.socket:ro", "", false}, // shell-expansion source
		{"./.jcore/php.ini:/usr/local/etc/php/php.ini", "./.jcore/php.ini", true},
	}
	for _, c := range cases {
		src, ok := bindMountSource(c.entry)
		if ok != c.wantMatch || src != c.wantSrc {
			t.Errorf("bindMountSource(%q) = (%q, %v), want (%q, %v)", c.entry, src, ok, c.wantSrc, c.wantMatch)
		}
	}
}

func TestComposeMountedFolders_RealTemplate(t *testing.T) {
	dir := t.TempDir()
	raw, err := os.ReadFile("../../container/base/docker-compose.yml")
	if err != nil {
		t.Fatalf("reading real docker-compose.yml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), raw, 0644); err != nil {
		t.Fatal(err)
	}

	folders, err := ComposeMountedFolders(dir)
	if err != nil {
		t.Fatalf("ComposeMountedFolders error = %v", err)
	}

	want := []string{"./.jcore/ssl", "./.jcore/wordpress", "./.config/nginx", "./vendor", "./wp-content"}
	// ./.jcore/php.ini is a file mount, but bindMountSource can't tell
	// files from folders by name alone — EnsureComposeMountedFolders is
	// what actually skips it (it already exists as a file by the time
	// that runs), not this parsing step, so it's expected in this list
	// too.
	want = append(want, "./.jcore/php.ini")
	sort.Strings(want)

	if !reflect.DeepEqual(folders, want) {
		t.Fatalf("ComposeMountedFolders() = %v, want %v", folders, want)
	}
}

func TestComposeMountedFolders_MissingFile(t *testing.T) {
	folders, err := ComposeMountedFolders(t.TempDir())
	if err != nil {
		t.Fatalf("ComposeMountedFolders error = %v", err)
	}
	if len(folders) != 0 {
		t.Fatalf("ComposeMountedFolders() for a project with no docker-compose.yml = %v, want none", folders)
	}
}

func TestEnsureComposeMountedFolders(t *testing.T) {
	dir := t.TempDir()
	compose := `services:
  app:
    volumes:
      - ./wp-content:/var/www/html/wp-content
      - ./.jcore/existing-file.txt:/etc/something
      - db:/var/lib/mysql
`
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(compose), 0644); err != nil {
		t.Fatal(err)
	}

	// Pre-create one entry as a real FILE, to confirm it's left alone
	// (not clobbered into a directory).
	if err := os.MkdirAll(filepath.Join(dir, ".jcore"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".jcore", "existing-file.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureComposeMountedFolders(dir); err != nil {
		t.Fatalf("EnsureComposeMountedFolders error = %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "wp-content"))
	if err != nil {
		t.Fatalf("expected wp-content to be created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected wp-content to be a directory")
	}

	content, err := os.ReadFile(filepath.Join(dir, ".jcore", "existing-file.txt"))
	if err != nil {
		t.Fatalf("expected existing-file.txt to survive: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("existing-file.txt content = %q, want %q (must not be clobbered)", content, "hello")
	}

	if _, err := os.Stat(filepath.Join(dir, "db")); err == nil {
		t.Fatal("named volume 'db' must not have been created as a folder")
	}
}
