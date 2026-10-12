// Package wpout tames the stderr noise of wp-cli runs (locally in the
// container or remotely over SSH). Both wp-cli's own warnings and PHP's
// notices/deprecations arrive on stderr, while stdout carries the real
// output, so only stderr is filtered:
//
//   - Errors (wp-cli "Error:", PHP fatal/parse errors) are always shown in
//     full, along with their continuation lines (stack traces).
//   - Warnings, notices and deprecations are shown once, on first
//     occurrence; repeats are only counted and listed in a summary when the
//     jcore command finishes. By default the first occurrence is cut to a
//     single line; --verbose shows it in full; --debug disables filtering.
//   - Anything else (rsync, ssh, bash errors, ...) passes through untouched.
//
// Everything, filtered or not, is also written to a per-command log file in
// .jcore/logs, of which the newest maxLogs are kept.
package wpout

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/JCO-Digital/jcore/internal/logging"
	"golang.org/x/term"
)

const (
	maxLogs        = 10
	fallbackWidth  = 80
	maxSummaryRows = 10
)

var (
	// Optional "[12-Oct-2026 10:00:00 UTC] " timestamp from PHP's error log,
	// then an optional "PHP " prefix (CLI error_log to stderr adds it).
	linePrefix = `^(?:\[[^\]]*\]\s*)?(?:PHP\s+)?`
	prefixRe   = regexp.MustCompile(linePrefix)
	errorRe    = regexp.MustCompile(linePrefix + `(?:(?:Fatal|Parse|Recoverable fatal|Catchable fatal) error|Error)\s*:`)
	noiseRe    = regexp.MustCompile(linePrefix + `(?:Warning|Notice|Deprecated|Strict Standards)\s*:`)
	// Lines that continue a previous message: indented text, stack trace
	// headers/frames ("#0 ...", "PHP   1. ...") and "thrown in ...".
	continuationRe = regexp.MustCompile(`^(?:\s|#\d|(?:PHP\s+)?Stack trace:|PHP\s+\d+\.|thrown in )`)
	locationRe     = regexp.MustCompile(`\s+in\s+.+\s+on\s+line\s+\d+\.?$`)
	ansiRe         = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
	spaceRe        = regexp.MustCompile(`\s+`)
)

type noise struct {
	text  string // cleaned first occurrence, for the summary
	count int
}

// state is shared by every filtered command of one jcore invocation, so
// warnings repeated across calls (e.g. one search-replace per rule) are
// still shown only once.
type state struct {
	mu      sync.Mutex
	dir     string // .jcore/logs, or "" outside a project
	name    string // jcore command, used in the log file name
	logPath string
	log     *os.File
	logErr  bool
	width   int
	out     io.Writer
	seen    map[string]*noise
	order   []string
}

var st = newState()

func newState() *state {
	return &state{width: terminalWidth(), out: os.Stderr, seen: map[string]*noise{}}
}

// Begin prepares filtering for one jcore command. projectDir may be empty
// (outside a project), in which case nothing is logged to disk.
func Begin(projectDir, command string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if projectDir != "" {
		st.dir = filepath.Join(projectDir, ".jcore", "logs")
	}
	st.name = sanitizeName(command)
}

// LogCommand records the command about to run in the log file.
func LogCommand(parts []string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.writeLog([]byte("$ " + strings.Join(parts, " ") + "\n"))
}

// Log returns a writer that only appends to the log file, used to tee a
// command's stdout so the log holds the complete output.
func Log() io.Writer { return logWriter{} }

type logWriter struct{}

func (logWriter) Write(p []byte) (int, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.writeLog(p)
	return len(p), nil
}

// Stderr returns a writer to use as one command's stderr. It splits the
// stream into lines and filters them; Close flushes a trailing partial line.
func Stderr() io.WriteCloser { return &Writer{} }

// Writer filters one command's stderr. Continuation tracking is per writer,
// since it describes the stream it was seen on.
type Writer struct {
	buf  []byte
	cont contMode
}

type contMode int

const (
	contNone contMode = iota // previous line wasn't a recognized message
	contShow                 // continuation lines of the previous message are shown
	contHide                 // ... are hidden
)

func (w *Writer) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		w.line(string(w.buf[:i]))
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

// Close flushes any unterminated last line.
func (w *Writer) Close() error {
	if len(w.buf) > 0 {
		w.line(string(w.buf))
		w.buf = nil
	}
	return nil
}

func (w *Writer) line(raw string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.writeLog([]byte(raw + "\n"))

	level := logging.Level()
	if level >= logging.LevelDebug {
		st.print(raw)
		return
	}

	clean := ansiRe.ReplaceAllString(strings.TrimRight(raw, "\r"), "")
	switch {
	case errorRe.MatchString(clean):
		st.print(raw)
		w.cont = contShow
	case noiseRe.MatchString(clean):
		w.cont = contHide
		key := normalize(clean)
		n, seen := st.seen[key]
		if !seen {
			n = &noise{text: clean}
			st.seen[key] = n
			st.order = append(st.order, key)
		}
		n.count++
		if seen || level < logging.LevelWarn {
			return
		}
		if level >= logging.LevelVerbose {
			st.print(clean)
			w.cont = contShow
		} else {
			st.print(truncate(clean, st.width))
		}
	case w.cont != contNone && continuationRe.MatchString(clean):
		if w.cont == contShow {
			st.print(raw)
		}
	default:
		w.cont = contNone
		st.print(raw)
	}
}

// Finish prints the summary of repeated warnings and closes the log. Call
// it once when the jcore command is done.
func Finish() {
	st.mu.Lock()
	defer st.mu.Unlock()
	defer st.closeLog()

	level := logging.Level()
	if len(st.order) == 0 || level < logging.LevelWarn || level >= logging.LevelDebug {
		return
	}

	total, repeated := 0, []*noise{}
	for _, key := range st.order {
		n := st.seen[key]
		total += n.count
		if n.count > 1 {
			repeated = append(repeated, n)
		}
	}
	if len(repeated) == 0 && st.logPath == "" {
		return
	}

	summary := fmt.Sprintf("wp-cli reported %d warning(s), %d unique.", total, len(st.order))
	if st.logPath != "" {
		summary += " Full output: " + relLogPath(st.logPath)
	}
	fmt.Fprintln(st.out, summary)

	sort.SliceStable(repeated, func(i, j int) bool { return repeated[i].count > repeated[j].count })
	for i, n := range repeated {
		if i == maxSummaryRows {
			fmt.Fprintf(st.out, "  ... and %d more repeated\n", len(repeated)-maxSummaryRows)
			break
		}
		prefix := fmt.Sprintf("  %4d× ", n.count)
		text := n.text
		if level < logging.LevelVerbose {
			text = truncate(text, st.width-len([]rune(prefix)))
		}
		fmt.Fprintln(st.out, prefix+text)
	}
}

func (s *state) print(line string) {
	fmt.Fprintln(s.out, line)
}

// writeLog appends to the log file, creating it (and pruning old ones) on
// first use so commands that never run wp-cli leave no empty logs behind.
// Logging is best effort: a failure is reported once and then ignored.
func (s *state) writeLog(p []byte) {
	if s.dir == "" || s.logErr {
		return
	}
	if s.log == nil {
		if err := s.openLog(); err != nil {
			s.logErr = true
			logging.Warn("Could not write wp-cli log: %v", err)
			return
		}
	}
	// Colour codes (wp-cli colours when SHELL_PIPE=0) would clutter the file.
	_, _ = s.log.Write(ansiRe.ReplaceAll(p, nil))
}

func (s *state) openLog() error {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return err
	}
	name := time.Now().Format("2006-01-02T15-04-05") + "-" + s.name + ".log"
	path := filepath.Join(s.dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	s.log, s.logPath = f, path
	pruneLogs(s.dir, maxLogs)
	return nil
}

func (s *state) closeLog() {
	if s.log != nil {
		_ = s.log.Close()
		s.log = nil
	}
}

// pruneLogs keeps the newest keep logs in dir. Names start with a sortable
// timestamp, so lexical order is chronological.
func pruneLogs(dir string, keep int) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.log"))
	if err != nil || len(matches) <= keep {
		return
	}
	sort.Strings(matches)
	for _, old := range matches[:len(matches)-keep] {
		_ = os.Remove(old)
	}
}

// normalize builds the de-duplication key: the message without its "PHP "
// prefix, timestamp, file/line location and whitespace differences.
func normalize(line string) string {
	line = prefixRe.ReplaceAllString(line, "")
	line = locationRe.ReplaceAllString(line, "")
	return strings.TrimSpace(spaceRe.ReplaceAllString(line, " "))
}

// truncate cuts s to at most width runes, marking a cut with "…".
func truncate(s string, width int) string {
	s = strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
	if width < 2 {
		width = 2
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	return string(r[:width-1]) + "…"
}

func terminalWidth() int {
	fd := int(os.Stderr.Fd())
	if term.IsTerminal(fd) {
		if w, _, err := term.GetSize(fd); err == nil && w > 0 {
			return w
		}
	}
	return fallbackWidth
}

func sanitizeName(s string) string {
	s = strings.Join(strings.Fields(s), "-")
	s = strings.Map(func(r rune) rune {
		if r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
	if s == "" {
		return "jcore"
	}
	return s
}

// relLogPath shows the log path relative to the working directory when
// that's shorter, e.g. ".jcore/logs/...".
func relLogPath(path string) string {
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, path); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	return path
}
