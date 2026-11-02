package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func mkSkill(t *testing.T, checkout, name string) {
	t.Helper()
	dir := filepath.Join(checkout, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func statuses(results []Result) map[string]Status {
	m := map[string]Status{}
	for _, r := range results {
		m[r.Name] = r.Status
	}
	return m
}

func TestDiscoverOnlyDirsWithSkillMD(t *testing.T) {
	checkout := t.TempDir()
	mkSkill(t, checkout, "b-skill")
	mkSkill(t, checkout, "a-skill")
	_ = os.MkdirAll(filepath.Join(checkout, "not-a-skill"), 0o755)
	_ = os.MkdirAll(filepath.Join(checkout, ".git"), 0o755)
	_ = os.WriteFile(filepath.Join(checkout, "README.md"), nil, 0o644)

	names, err := Discover(checkout)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "a-skill" || names[1] != "b-skill" {
		t.Fatalf("got %v", names)
	}
}

func TestInstallLinksAndIsIdempotent(t *testing.T) {
	checkout, target := t.TempDir(), filepath.Join(t.TempDir(), "skills")
	mkSkill(t, checkout, "jcore-plugin")

	res, err := Install(checkout, target, false)
	if err != nil {
		t.Fatal(err)
	}
	if s := statuses(res)["jcore-plugin"]; s != Linked {
		t.Fatalf("first install: %s", s)
	}
	if link, _ := os.Readlink(filepath.Join(target, "jcore-plugin")); link != filepath.Join(checkout, "jcore-plugin") {
		t.Fatalf("link points at %q", link)
	}

	res, _ = Install(checkout, target, false)
	if s := statuses(res)["jcore-plugin"]; s != UpToDate {
		t.Fatalf("second install: %s", s)
	}
}

func TestInstallRespectsForeignSymlinkUnlessForced(t *testing.T) {
	checkout, target, devClone := t.TempDir(), t.TempDir(), t.TempDir()
	mkSkill(t, checkout, "jcore-plugin")
	if err := os.Symlink(devClone, filepath.Join(target, "jcore-plugin")); err != nil {
		t.Fatal(err)
	}

	res, _ := Install(checkout, target, false)
	if s := statuses(res)["jcore-plugin"]; s != Skipped {
		t.Fatalf("without force: %s", s)
	}
	if link, _ := os.Readlink(filepath.Join(target, "jcore-plugin")); link != devClone {
		t.Fatal("foreign symlink was changed without --force")
	}

	res, _ = Install(checkout, target, true)
	if s := statuses(res)["jcore-plugin"]; s != Replaced {
		t.Fatalf("with force: %s", s)
	}
}

func TestInstallNeverTouchesRealDirectory(t *testing.T) {
	checkout, target := t.TempDir(), t.TempDir()
	mkSkill(t, checkout, "jcore-plugin")
	mkSkill(t, target, "jcore-plugin")

	res, _ := Install(checkout, target, true)
	if s := statuses(res)["jcore-plugin"]; s != Skipped {
		t.Fatalf("got %s", s)
	}
	if _, err := os.Stat(filepath.Join(target, "jcore-plugin", "SKILL.md")); err != nil {
		t.Fatal("real directory was modified")
	}
}

func TestInstallPrunesOnlyOwnStaleLinks(t *testing.T) {
	checkout, target, other := t.TempDir(), t.TempDir(), t.TempDir()
	mkSkill(t, checkout, "kept")
	_ = os.Symlink(filepath.Join(checkout, "gone"), filepath.Join(target, "gone"))
	_ = os.Symlink(other, filepath.Join(target, "unrelated"))

	res, err := Install(checkout, target, false)
	if err != nil {
		t.Fatal(err)
	}
	if s := statuses(res)["gone"]; s != Removed {
		t.Fatalf("stale link: %s", s)
	}
	if _, err := os.Lstat(filepath.Join(target, "unrelated")); err != nil {
		t.Fatal("unrelated symlink was removed")
	}
}
