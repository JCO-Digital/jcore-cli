package project

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"My Cool Project": "my-cool-project",
		"already-slug":    "already-slug",
		"  spaced  out  ": "spaced-out",
		"Special!@#Chars": "special-chars",
		"":                "",
	}
	for input, want := range cases {
		if got := Slugify(input); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", input, got, want)
		}
	}
}
