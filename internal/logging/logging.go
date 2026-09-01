// Package logging provides jcore's leveled console output, mirroring the
// legacy TypeScript CLI's logger.ts/logLevels: a single numeric level
// (Error=0 through Silly=6) that CLI flags (--quiet/--verbose/--debug/
// --loglevel) or the persisted `logLevel` setting can raise or lower,
// gating which messages are actually printed.
package logging

import (
	"fmt"
	"os"
)

const (
	LevelError   = 0
	LevelWarn    = 1
	LevelInfo    = 2
	LevelHTTP    = 3
	LevelVerbose = 4
	LevelDebug   = 5
	LevelSilly   = 6
)

// DefaultLevel is used whenever nothing else overrides it, matching the
// `logLevel` setting's own schema default.
const DefaultLevel = LevelInfo

var level = DefaultLevel

// SetLevel sets the effective log level for the rest of this process.
func SetLevel(l int) {
	level = l
}

// Level returns the currently effective log level.
func Level() int {
	return level
}

// Error prints to stderr unconditionally — errors are never suppressed by
// level, matching logger.ts's error().
func Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Warn prints to stderr if the effective level is at least LevelWarn (i.e.
// unless --quiet was passed).
func Warn(format string, args ...any) {
	if level >= LevelWarn {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

// Info prints to stdout if the effective level is at least LevelInfo — the
// default, so visible unless --quiet was passed.
func Info(format string, args ...any) {
	if level >= LevelInfo {
		fmt.Printf(format+"\n", args...)
	}
}

// Verbose prints to stdout if the effective level is at least LevelVerbose
// (--verbose, --debug, or an explicit --loglevel/logLevel setting that high
// or higher).
func Verbose(format string, args ...any) {
	if level >= LevelVerbose {
		fmt.Printf(format+"\n", args...)
	}
}

// Debug prints to stdout if the effective level is at least LevelDebug
// (--debug, or an explicit --loglevel/logLevel setting that high or
// higher).
func Debug(format string, args ...any) {
	if level >= LevelDebug {
		fmt.Printf(format+"\n", args...)
	}
}
