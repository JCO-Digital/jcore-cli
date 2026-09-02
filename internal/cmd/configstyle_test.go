package cmd

import (
	"strings"
	"testing"
)

// stripANSI removes any ANSI escape sequences, so these assertions hold
// whether or not lipgloss decided to colorize in this test environment.
func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case inEscape:
			if r == 'm' {
				inEscape = false
			}
		case r == '\x1b':
			inEscape = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestColorizeValue(t *testing.T) {
	// Covers both normalized Go-native types (from config.Store/
	// config.ProjectDefaultsFile) and go-toml's raw decoded types (from
	// viper.AllSettings(), which the real merge pipeline feeds with no
	// normalization pass) — see colorizeValue's doc comment.
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"bool", true, "true"},
		{"int", 3, "3"},
		{"int64", int64(3), "3"},
		{"string-slice", []string{"lohko"}, "[lohko]"},
		{"any-slice", []any{"lohko"}, "[lohko]"},
		{"string", "hello", "hello"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := stripANSI(colorizeValue(c.in)); got != c.want {
				t.Errorf("colorizeValue(%#v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestColorizeScopeLabel(t *testing.T) {
	// Only checking text survives round-trip un-mangled; the actual color
	// choice isn't observable without a real TTY.
	for _, label := range []string{"default", "global", "project", "local", "project@staging"} {
		if got := stripANSI(colorizeScopeLabel(label)); got != label {
			t.Errorf("colorizeScopeLabel(%q) = %q, want %q unchanged", label, got, label)
		}
	}
}
