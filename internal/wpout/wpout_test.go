package wpout

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JCO-Digital/jcore/internal/logging"
)

// setup resets the package state, captures terminal output in a buffer and
// sets the log level for the duration of the test.
func setup(t *testing.T, level int, projectDir string) *bytes.Buffer {
	t.Helper()
	var out bytes.Buffer
	st = newState()
	st.out = &out
	st.width = 40
	logging.SetLevel(level)
	t.Cleanup(func() {
		st.closeLog()
		st = newState()
		logging.SetLevel(logging.DefaultLevel)
	})
	Begin(projectDir, " pull db")
	return &out
}

func feed(lines ...string) {
	w := Stderr()
	_, _ = w.Write([]byte(strings.Join(lines, "\n") + "\n"))
	_ = w.Close()
}

const longWarning = `Warning: Skipping an uninitialized class "u2flib_server\RegisterRequest", replacements might not be complete.`

func TestDefaultDedupesAndTruncates(t *testing.T) {
	out := setup(t, logging.DefaultLevel, "")
	feed(longWarning, longWarning, "rsync: some other problem", longWarning)

	got := out.String()
	if n := strings.Count(got, "Skipping"); n != 1 {
		t.Errorf("warning shown %d times, want 1:\n%s", n, got)
	}
	first := strings.SplitN(got, "\n", 2)[0]
	if len([]rune(first)) != 40 || !strings.HasSuffix(first, "…") {
		t.Errorf("first line not truncated to 40 runes: %q", first)
	}
	if !strings.Contains(got, "rsync: some other problem\n") {
		t.Errorf("unrelated stderr line was not passed through:\n%s", got)
	}

	out.Reset()
	Finish()
	summary := out.String()
	if !strings.Contains(summary, "3 warning(s), 1 unique") || !strings.Contains(summary, "3× Warning: Skipping") {
		t.Errorf("unexpected summary:\n%s", summary)
	}
}

func TestErrorsAlwaysShownInFull(t *testing.T) {
	out := setup(t, logging.DefaultLevel, "")
	fatal := "PHP Fatal error:  Uncaught Exception: something long enough to be truncated in /var/www/html/x.php:12"
	feed(fatal, "Stack trace:", "#0 {main}", "  thrown in /var/www/html/x.php on line 12", "Error: Something failed.", "Error: Something failed.")

	got := out.String()
	for _, want := range []string{fatal, "Stack trace:", "#0 {main}", "thrown in", "Error: Something failed."} {
		if !strings.Contains(got, want) {
			t.Errorf("output is missing %q:\n%s", want, got)
		}
	}
	if n := strings.Count(got, "Error: Something failed."); n != 2 {
		t.Errorf("repeated error shown %d times, want 2", n)
	}
}

func TestNoiseContinuationHidden(t *testing.T) {
	out := setup(t, logging.DefaultLevel, "")
	feed("PHP Deprecated:  old thing in /a.php on line 3", "PHP Stack trace:", "PHP   1. {main}() /a.php:0", "Success: done")

	got := out.String()
	if strings.Contains(got, "Stack trace") || strings.Contains(got, "{main}") {
		t.Errorf("continuation of a warning should be hidden:\n%s", got)
	}
	if !strings.Contains(got, "Success: done") {
		t.Errorf("line after the continuation was swallowed:\n%s", got)
	}
}

func TestVerboseDoesNotTruncate(t *testing.T) {
	out := setup(t, logging.LevelVerbose, "")
	feed(longWarning, "  more detail", longWarning)

	got := out.String()
	if !strings.Contains(got, longWarning+"\n  more detail\n") {
		t.Errorf("verbose should show the full first occurrence:\n%s", got)
	}
	if n := strings.Count(got, "Skipping"); n != 1 {
		t.Errorf("verbose should still de-duplicate, shown %d times", n)
	}
}

func TestDebugShowsEverything(t *testing.T) {
	out := setup(t, logging.LevelDebug, "")
	feed(longWarning, longWarning)
	Finish()

	if got, want := out.String(), longWarning+"\n"+longWarning+"\n"; got != want {
		t.Errorf("debug output = %q, want raw %q", got, want)
	}
}

func TestQuietHidesWarningsButNotErrors(t *testing.T) {
	out := setup(t, logging.LevelError, "")
	feed(longWarning, "Error: broken")
	Finish()

	if got := out.String(); got != "Error: broken\n" {
		t.Errorf("quiet output = %q", got)
	}
}

func TestNormalizeGroupsVariants(t *testing.T) {
	a := normalize("PHP Warning:  Undefined array key \"x\" in /var/www/a.php on line 3")
	b := normalize("[12-Oct-2026 10:00:00 UTC] Warning: Undefined array key \"x\" in /var/www/b.php on line 99")
	if a != b {
		t.Errorf("normalize(%q) != normalize(%q)", a, b)
	}
}

func TestANSIAndPartialLines(t *testing.T) {
	out := setup(t, logging.DefaultLevel, "")
	w := Stderr()
	_, _ = w.Write([]byte("\x1b[33mWarn"))
	_, _ = w.Write([]byte("ing:\x1b[0m short\n\x1b[33mWarning:\x1b[0m short\nno newline"))
	_ = w.Close()

	if got := out.String(); got != "Warning: short\nno newline\n" {
		t.Errorf("output = %q", got)
	}
}

func TestLogFileWrittenAndPruned(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, ".jcore", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		name := filepath.Join(logDir, fmt.Sprintf("2000-01-01T00-00-%02d-old.log", i))
		if err := os.WriteFile(name, nil, 0644); err != nil {
			t.Fatal(err)
		}
	}

	setup(t, logging.DefaultLevel, dir)
	LogCommand([]string{"wp", "search-replace"})
	feed(longWarning, longWarning)
	_, _ = Log().Write([]byte("\x1b[32;1mSuccess:\x1b[0m Made 2 replacements.\n"))
	st.closeLog()

	logs, _ := filepath.Glob(filepath.Join(logDir, "*.log"))
	if len(logs) != maxLogs {
		t.Fatalf("%d logs kept, want %d", len(logs), maxLogs)
	}
	if _, err := os.Stat(filepath.Join(logDir, "2000-01-01T00-00-02-old.log")); err == nil {
		t.Error("oldest logs were not pruned")
	}
	if !strings.HasSuffix(st.logPath, "-pull-db.log") {
		t.Errorf("log name = %q, want it to end in -pull-db.log", st.logPath)
	}
	content, err := os.ReadFile(st.logPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "$ wp search-replace\n" + longWarning + "\n" + longWarning + "\nSuccess: Made 2 replacements.\n"
	if string(content) != want {
		t.Errorf("log content = %q, want %q", content, want)
	}
}

func TestNoLogWithoutWpCliOutput(t *testing.T) {
	dir := t.TempDir()
	setup(t, logging.DefaultLevel, dir)
	Finish()
	if _, err := os.Stat(filepath.Join(dir, ".jcore", "logs")); err == nil {
		t.Error("log directory created although nothing was logged")
	}
}
