package project

import (
	"regexp"
	"strings"
)

var slugifyRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts s into a lowercase, hyphen-separated slug suitable for a
// directory name, theme name, or block identifier.
func Slugify(s string) string {
	s = strings.ToLower(s)
	s = slugifyRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
