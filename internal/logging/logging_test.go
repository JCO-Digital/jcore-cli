package logging

import (
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// whatever it wrote.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = old

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestSetLevelAndLevel(t *testing.T) {
	defer SetLevel(DefaultLevel)

	SetLevel(LevelDebug)
	if got := Level(); got != LevelDebug {
		t.Fatalf("Level() = %d, want %d", got, LevelDebug)
	}
}

func TestErrorAlwaysPrints(t *testing.T) {
	defer SetLevel(DefaultLevel)
	SetLevel(LevelError)

	out := captureStderr(t, func() { Error("boom %d", 1) })
	if !strings.Contains(out, "boom 1") {
		t.Errorf("Error() output = %q, want it to contain %q", out, "boom 1")
	}
}

func TestWarnGatedByLevel(t *testing.T) {
	defer SetLevel(DefaultLevel)

	SetLevel(LevelError) // below LevelWarn
	if out := captureStderr(t, func() { Warn("careful") }); out != "" {
		t.Errorf("Warn() at LevelError printed %q, want nothing", out)
	}

	SetLevel(LevelWarn)
	if out := captureStderr(t, func() { Warn("careful") }); !strings.Contains(out, "careful") {
		t.Errorf("Warn() at LevelWarn = %q, want it to contain %q", out, "careful")
	}
}

func TestInfoGatedByLevel(t *testing.T) {
	defer SetLevel(DefaultLevel)

	SetLevel(LevelWarn) // below LevelInfo (quiet)
	if out := captureStdout(t, func() { Info("hello") }); out != "" {
		t.Errorf("Info() at LevelWarn printed %q, want nothing", out)
	}

	SetLevel(LevelInfo)
	if out := captureStdout(t, func() { Info("hello") }); !strings.Contains(out, "hello") {
		t.Errorf("Info() at LevelInfo = %q, want it to contain %q", out, "hello")
	}
}

func TestVerboseAndDebugGatedByLevel(t *testing.T) {
	defer SetLevel(DefaultLevel)

	SetLevel(LevelInfo)
	if out := captureStdout(t, func() { Verbose("v") }); out != "" {
		t.Errorf("Verbose() at LevelInfo printed %q, want nothing", out)
	}
	if out := captureStdout(t, func() { Debug("d") }); out != "" {
		t.Errorf("Debug() at LevelInfo printed %q, want nothing", out)
	}

	SetLevel(LevelVerbose)
	if out := captureStdout(t, func() { Verbose("v") }); !strings.Contains(out, "v") {
		t.Errorf("Verbose() at LevelVerbose = %q, want it to contain %q", out, "v")
	}
	if out := captureStdout(t, func() { Debug("d") }); out != "" {
		t.Errorf("Debug() at LevelVerbose printed %q, want nothing (needs LevelDebug)", out)
	}

	SetLevel(LevelDebug)
	if out := captureStdout(t, func() { Debug("d") }); !strings.Contains(out, "d") {
		t.Errorf("Debug() at LevelDebug = %q, want it to contain %q", out, "d")
	}
}
