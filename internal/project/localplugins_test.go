package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// fakeDocker puts a docker on PATH that logs each call's arguments to the
// returned file and runs script (sh, with the arguments as "$@").
func fakeDocker(t *testing.T, script string) string {
	t.Helper()
	binDir := t.TempDir()
	logFile := filepath.Join(t.TempDir(), "calls.log")
	body := "#!/bin/sh\necho \"$@\" >> " + logFile + "\n" + script + "\n"
	if err := os.WriteFile(filepath.Join(binDir, "docker"), []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logFile
}

func TestInstallLocalPlugins(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("pluginLocal", []string{"query-monitor", "debug-bar", "broken"})

	// query-monitor is installed already, the others aren't; installing
	// broken fails.
	logFile := fakeDocker(t, `
case "$*" in
  *"is-installed --path=/var/www/html query-monitor") exit 0 ;;
  *is-installed*) exit 1 ;;
  *"install --path=/var/www/html --activate broken") exit 1 ;;
esac`)

	err := InstallLocalPlugins(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "broken") || strings.Contains(err.Error(), "debug-bar") {
		t.Errorf("InstallLocalPlugins error = %v, want one naming only broken", err)
	}

	raw, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	calls := string(raw)
	for _, want := range []string{
		"wordpress wp plugin activate --path=/var/www/html query-monitor",
		"wordpress wp plugin install --path=/var/www/html --activate debug-bar",
		"wordpress wp plugin install --path=/var/www/html --activate broken",
	} {
		if !strings.Contains(calls, want) {
			t.Errorf("no call ending %q in:\n%s", want, calls)
		}
	}
	if strings.Contains(calls, "install --path=/var/www/html --activate query-monitor") {
		t.Errorf("installed query-monitor again:\n%s", calls)
	}
}

func TestInstallLocalPlugins_NoneConfigured(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	logFile := fakeDocker(t, "")

	if err := InstallLocalPlugins(t.TempDir()); err != nil {
		t.Fatalf("InstallLocalPlugins error = %v", err)
	}
	if _, err := os.Stat(logFile); !os.IsNotExist(err) {
		t.Error("ran docker without any pluginLocal plugins")
	}
}
