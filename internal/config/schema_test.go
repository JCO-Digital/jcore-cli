package config

import "testing"

func TestLookup(t *testing.T) {
	def, ok := Lookup("projectName")
	if !ok {
		t.Fatal("expected projectName to be a known setting")
	}
	if def.Type != TypeString {
		t.Fatalf("projectName type = %v, want TypeString", def.Type)
	}
	if def.Category != "Project" {
		t.Fatalf("projectName category = %q, want Project", def.Category)
	}

	if _, ok := Lookup("notARealSetting"); ok {
		t.Fatal("expected notARealSetting to be unknown")
	}
}

func TestScopeClassification(t *testing.T) {
	globalOnly := []string{"template", "debug", "install", "logLevel", "mode", "pluginLocal", "projectDefault", "verbose"}
	for _, key := range globalOnly {
		def, ok := Lookup(key)
		if !ok {
			t.Fatalf("expected %s to be a known setting", key)
		}
		if def.ScopeClass != ScopeClassGlobalOnly {
			t.Errorf("%s ScopeClass = %v, want ScopeClassGlobalOnly", key, def.ScopeClass)
		}
	}

	projectEligible := []string{"projectName", "theme", "branch", "remoteDomain", "wpImage", "wpDbPassword"}
	for _, key := range projectEligible {
		def, ok := Lookup(key)
		if !ok {
			t.Fatalf("expected %s to be a known setting", key)
		}
		if def.ScopeClass != ScopeClassProjectEligible {
			t.Errorf("%s ScopeClass = %v, want ScopeClassProjectEligible", key, def.ScopeClass)
		}
	}
}

func TestCategoriesAndInCategory(t *testing.T) {
	categories := Categories()
	if len(categories) == 0 {
		t.Fatal("expected at least one category")
	}

	seen := make(map[string]bool)
	total := 0
	for _, c := range categories {
		if seen[c] {
			t.Fatalf("category %q listed more than once", c)
		}
		seen[c] = true

		defs := InCategory(c)
		if len(defs) == 0 {
			t.Fatalf("category %q has no settings", c)
		}
		for _, d := range defs {
			if d.Category != c {
				t.Fatalf("InCategory(%q) returned setting %s with Category %q", c, d.Key, d.Category)
			}
		}
		total += len(defs)
	}

	if total != len(Settings) {
		t.Fatalf("categories account for %d settings, want %d", total, len(Settings))
	}
}
