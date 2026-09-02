package project

import "testing"

// TestLoadTemplateCatalog checks the embedded catalog matches what
// internal/cmd/init.go's interactive selector relies on: every template it
// can offer must be present, and jcore3 (the default) must carry a real
// themeUrl and a multi-branch selection.
func TestLoadTemplateCatalog(t *testing.T) {
	catalog, err := LoadTemplateCatalog()
	if err != nil {
		t.Fatalf("LoadTemplateCatalog error = %v", err)
	}

	for _, name := range []string{"jcore3", "jcore2", "jcore1", "blank"} {
		if _, ok := catalog[name]; !ok {
			t.Errorf("catalog missing expected template %q", name)
		}
	}

	jcore3 := catalog["jcore3"]
	if jcore3.Branch == "" {
		t.Error(`catalog["jcore3"].Branch is empty, want a default branch`)
	}
	if len(jcore3.Branches) < 2 {
		t.Errorf(`catalog["jcore3"].Branches = %v, want at least 2 (for the branch-select prompt)`, jcore3.Branches)
	}
	if jcore3.ThemeURL == "" {
		t.Error(`catalog["jcore3"].ThemeURL is empty, want the jcore-ilme archive URL`)
	}

	blank := catalog["blank"]
	if blank.Branch == "" {
		t.Error(`catalog["blank"].Branch is empty, want a fallback default branch`)
	}
	if len(blank.Branches) != 0 {
		t.Errorf(`catalog["blank"].Branches = %v, want none (no branch-select prompt)`, blank.Branches)
	}
}
