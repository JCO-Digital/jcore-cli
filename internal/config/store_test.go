package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetConfigPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	globalPath, err := GetConfigPath(ScopeGlobal, "")
	if err != nil {
		t.Fatalf("GetConfigPath(Global, \"\") error = %v", err)
	}
	if !filepath.IsAbs(globalPath) {
		t.Fatalf("global path %q is not absolute", globalPath)
	}

	// Global must not depend on the current working directory.
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWD)
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	globalPath2, err := GetConfigPath(ScopeGlobal, "")
	if err != nil {
		t.Fatalf("GetConfigPath(Global, \"\") error = %v", err)
	}
	if globalPath != globalPath2 {
		t.Fatalf("global path changed with cwd: %q vs %q", globalPath, globalPath2)
	}

	root := "/some/project/root"
	projectPath, err := GetConfigPath(ScopeProject, root)
	if err != nil {
		t.Fatalf("GetConfigPath(Project, root) error = %v", err)
	}
	if want := filepath.Join(root, ProjectConfigName); projectPath != want {
		t.Fatalf("project path = %q, want %q", projectPath, want)
	}

	localPath, err := GetConfigPath(ScopeLocal, root)
	if err != nil {
		t.Fatalf("GetConfigPath(Local, root) error = %v", err)
	}
	if want := filepath.Join(root, LocalConfigName); localPath != want {
		t.Fatalf("local path = %q, want %q", localPath, want)
	}

	if _, err := GetConfigPath(ScopeProject, ""); err == nil {
		t.Fatal("expected error resolving Project scope with no project root")
	}
	if _, err := GetConfigPath(ScopeLocal, ""); err == nil {
		t.Fatal("expected error resolving Local scope with no project root")
	}
}

// TestStoreReadsPreExistingLowercasedKeys reproduces a real jcore.toml that
// was corrupted by a pre-Store version of `config set`, which wrote through
// viper and so silently lower-cased every key on save. Store must still be
// able to find these values (case-insensitively), and Save must write them
// back out in their correct case.
func TestStoreReadsPreExistingLowercasedKeys(t *testing.T) {
	root := t.TempDir()
	corrupted := `dbprefix = '7q3_'
domains = ['kehitys.localhost']
localdomain = 'kehitys.localhost'
mode = 'bg'
pluginexclude = ['ruudukko']
plugingit = ['lohko', 'jcore-portti']
plugininstall = 'remote'
projectname = 'kehitys'
remotedomain = 'kehitys.jcore.fi'
remotehost = 'kehitys@vs.bojaco.com'
remotepath = 'files'
theme = 'ilme'
wpimage = 'jcodigi/wordpress:8.4'

[branch-testing]
remotedomain = 'stagingtest.jcore.fi'
remotehost = 'staging@vs.bojaco.com'
`
	if err := os.WriteFile(filepath.Join(root, ProjectConfigName), []byte(corrupted), 0644); err != nil {
		t.Fatal(err)
	}

	store, err := OpenStore(ScopeProject, root, "")
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}

	cases := map[string]any{
		"dbPrefix":      "7q3_",
		"localDomain":   "kehitys.localhost",
		"pluginInstall": "remote",
		"projectName":   "kehitys",
		"remoteDomain":  "kehitys.jcore.fi",
		"remoteHost":    "kehitys@vs.bojaco.com",
		"remotePath":    "files",
		"wpImage":       "jcodigi/wordpress:8.4",
	}
	for key, want := range cases {
		got, ok := store.Get(key)
		if !ok || got != want {
			t.Errorf("Get(%q) = %#v, %v, want %#v, true", key, got, ok, want)
		}
	}

	// Any edit rewrites the whole file, self-healing every key's casing.
	if err := store.Set("theme", "updated"); err != nil {
		t.Fatalf("Set error = %v", err)
	}
	if err := store.Save(); err != nil {
		t.Fatalf("Save error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(root, ProjectConfigName))
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	if strings.Contains(content, "wpimage") || strings.Contains(content, "projectname") {
		t.Fatalf("expected lower-cased keys to be healed on save, got:\n%s", content)
	}
	if !strings.Contains(content, "wpImage") || !strings.Contains(content, "projectName") {
		t.Fatalf("expected correctly-cased keys after save, got:\n%s", content)
	}
}

func TestStorePreservesKeyCasing(t *testing.T) {
	root := t.TempDir()

	store, err := OpenStore(ScopeProject, root, "")
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	if err := store.Set("projectName", "demo"); err != nil {
		t.Fatalf("Set error = %v", err)
	}
	if err := store.Set("wpDbPassword", "secret"); err != nil {
		t.Fatalf("Set error = %v", err)
	}
	if err := store.Save(); err != nil {
		t.Fatalf("Save error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(root, ProjectConfigName))
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	content := string(raw)
	if !strings.Contains(content, "projectName") {
		t.Fatalf("expected camelCase key %q preserved in file, got:\n%s", "projectName", content)
	}
	if !strings.Contains(content, "wpDbPassword") {
		t.Fatalf("expected camelCase key %q preserved in file, got:\n%s", "wpDbPassword", content)
	}
	if strings.Contains(content, "projectname") || strings.Contains(content, "wpdbpassword") {
		t.Fatalf("keys were lower-cased on save, got:\n%s", content)
	}
}

func TestStoreTypeCoercion(t *testing.T) {
	root := t.TempDir()

	store, err := OpenStore(ScopeProject, root, "")
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	if err := store.Set("debug", "true"); err != nil {
		t.Fatalf("Set error = %v", err)
	}
	if err := store.Set("logLevel", "5"); err != nil {
		t.Fatalf("Set error = %v", err)
	}
	if err := store.Set("domains", "a.test, b.test"); err != nil {
		t.Fatalf("Set error = %v", err)
	}
	if err := store.Save(); err != nil {
		t.Fatalf("Save error = %v", err)
	}

	reloaded, err := OpenStore(ScopeProject, root, "")
	if err != nil {
		t.Fatalf("OpenStore (reload) error = %v", err)
	}

	debug, ok := reloaded.Get("debug")
	if !ok {
		t.Fatal("expected debug to be set")
	}
	if b, ok := debug.(bool); !ok || !b {
		t.Fatalf("debug = %#v (%T), want bool true", debug, debug)
	}

	logLevel, ok := reloaded.Get("logLevel")
	if !ok {
		t.Fatal("expected logLevel to be set")
	}
	if i, ok := logLevel.(int); !ok || i != 5 {
		t.Fatalf("logLevel = %#v (%T), want int 5", logLevel, logLevel)
	}

	domains, ok := reloaded.Get("domains")
	if !ok {
		t.Fatal("expected domains to be set")
	}
	slice, ok := domains.([]string)
	if !ok || len(slice) != 2 || slice[0] != "a.test" || slice[1] != "b.test" {
		t.Fatalf("domains = %#v, want [a.test b.test]", domains)
	}
}

// TestStoreTypeCoercion_BoolAcceptsLegacyTruthyStrings mirrors the legacy
// TypeScript CLI's parseBoolean, which never errors and accepts a wider set
// of case-insensitive truthy strings than strconv.ParseBool. Anything not
// on that list (typos included) is false, not an error.
func TestStoreTypeCoercion_BoolAcceptsLegacyTruthyStrings(t *testing.T) {
	root := t.TempDir()

	cases := map[string]bool{
		"true": true, "True": true, "TRUE": true,
		"yes": true, "on": true, "y": true, "t": true, "1": true,
		"false": false, "no": false, "off": false, "n": false, "0": false,
		"typo": false, "": false,
	}

	for input, want := range cases {
		store, err := OpenStore(ScopeProject, root, "")
		if err != nil {
			t.Fatalf("OpenStore error = %v", err)
		}
		if err := store.Set("debug", input); err != nil {
			t.Fatalf("Set(%q) unexpected error = %v (parseBoolean must never error)", input, err)
		}
		got, _ := store.Get("debug")
		if got != want {
			t.Errorf("Set(%q) -> debug = %#v, want %v", input, got, want)
		}
	}
}

func TestStoreUnset(t *testing.T) {
	root := t.TempDir()

	store, err := OpenStore(ScopeProject, root, "")
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	_ = store.Set("projectName", "demo")
	_ = store.Set("theme", "jcore-ilme")
	_ = store.Set("branch", "main")
	if err := store.Save(); err != nil {
		t.Fatalf("Save error = %v", err)
	}

	reloaded, err := OpenStore(ScopeProject, root, "")
	if err != nil {
		t.Fatalf("OpenStore (reload) error = %v", err)
	}
	if err := reloaded.Unset("theme"); err != nil {
		t.Fatalf("Unset error = %v", err)
	}
	if err := reloaded.Save(); err != nil {
		t.Fatalf("Save error = %v", err)
	}

	final, err := OpenStore(ScopeProject, root, "")
	if err != nil {
		t.Fatalf("OpenStore (final) error = %v", err)
	}
	if _, ok := final.Get("theme"); ok {
		t.Fatal("expected theme to be removed")
	}
	if v, ok := final.Get("projectName"); !ok || v != "demo" {
		t.Fatalf("projectName = %#v, %v, want demo, true", v, ok)
	}
	if v, ok := final.Get("branch"); !ok || v != "main" {
		t.Fatalf("branch = %#v, %v, want main, true", v, ok)
	}
}

func TestResolvePrecedence(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	const key = "theme"

	// Nothing set anywhere: falls back to schema default.
	res, err := Resolve(key, root, "")
	if err != nil {
		t.Fatalf("Resolve error = %v", err)
	}
	if !res.IsDefault {
		t.Fatalf("expected default resolution, got %+v", res)
	}

	global, _ := OpenStore(ScopeGlobal, root, "")
	_ = global.Set(key, "from-global")
	_ = global.Save()

	res, err = Resolve(key, root, "")
	if err != nil {
		t.Fatalf("Resolve error = %v", err)
	}
	if res.SourceScope != ScopeGlobal || res.Value != "from-global" {
		t.Fatalf("Resolve = %+v, want global/from-global", res)
	}

	project, _ := OpenStore(ScopeProject, root, "")
	_ = project.Set(key, "from-project")
	_ = project.Save()

	res, err = Resolve(key, root, "")
	if err != nil {
		t.Fatalf("Resolve error = %v", err)
	}
	if res.SourceScope != ScopeProject || res.Value != "from-project" {
		t.Fatalf("Resolve = %+v, want project/from-project", res)
	}

	local, _ := OpenStore(ScopeLocal, root, "")
	_ = local.Set(key, "from-local")
	_ = local.Save()

	res, err = Resolve(key, root, "")
	if err != nil {
		t.Fatalf("Resolve error = %v", err)
	}
	if res.SourceScope != ScopeLocal || res.Value != "from-local" {
		t.Fatalf("Resolve = %+v, want local/from-local", res)
	}
}

func TestResolveFallsBackToEmbeddedBaseDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// wpImage is set in container/base/defaults.toml but not by anything
	// else here, so it must come from the embedded layer, not the (empty)
	// schema.SettingDef.Default placeholder.
	res, err := Resolve("wpImage", "", "")
	if err != nil {
		t.Fatalf("Resolve error = %v", err)
	}
	if !res.IsDefault {
		t.Fatalf("Resolve(wpImage) = %+v, want IsDefault true", res)
	}
	if res.Value != "jcodigi/wordpress:latest" {
		t.Fatalf("Resolve(wpImage) = %+v, want value from embedded base defaults.toml", res)
	}
}

// TestResolveFallsBackToSchemaDefault checks a setting with no embedded
// base defaults.toml entry (unlike wpImage) still resolves to its real
// SettingDef.Default, not a blank value — the specific gap
// TestDefaultsArePopulated (schema_test.go) guards from the other side.
func TestResolveFallsBackToSchemaDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	res, err := Resolve("pluginInstall", "", "")
	if err != nil {
		t.Fatalf("Resolve error = %v", err)
	}
	if !res.IsDefault || res.Value != "remote" {
		t.Fatalf(`Resolve(pluginInstall) = %+v, want IsDefault true, Value "remote"`, res)
	}

	res, err = Resolve("projectDefault", "", "")
	if err != nil {
		t.Fatalf("Resolve error = %v", err)
	}
	if !res.IsDefault || res.Value != "git@github.com:JCO-Digital/{name}.git" {
		t.Fatalf(`Resolve(projectDefault) = %+v, want IsDefault true, Value "git@github.com:JCO-Digital/{name}.git"`, res)
	}
}

func TestResolveFallsBackToProjectDefaultsFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	if err := os.WriteFile(filepath.Join(root, "defaults.toml"), []byte(`pluginGit = ["lohko"]`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := Resolve("pluginGit", root, "")
	if err != nil {
		t.Fatalf("Resolve error = %v", err)
	}
	if !res.IsDefault {
		t.Fatalf("Resolve(pluginGit) = %+v, want IsDefault true", res)
	}
	slice, ok := res.Value.([]string)
	if !ok || len(slice) != 1 || slice[0] != "lohko" {
		t.Fatalf("Resolve(pluginGit) = %#v, want [lohko]", res.Value)
	}

	// jcore.toml (Project scope) must still win over the project's own
	// defaults.toml, matching cmd/root.go's real merge order.
	store, err := OpenStore(ScopeProject, root, "")
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Set("pluginGit", "override")
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	res, err = Resolve("pluginGit", root, "")
	if err != nil {
		t.Fatalf("Resolve error = %v", err)
	}
	if res.IsDefault || res.SourceScope != ScopeProject {
		t.Fatalf("Resolve(pluginGit) = %+v, want sourced from Project", res)
	}
}

// TestProjectDefaultsFile checks the read path `config list defaults`/
// `config list all` uses directly: branch-adjusted and value-normalized,
// same as Resolve's own internal use of this layer.
func TestProjectDefaultsFile(t *testing.T) {
	root := t.TempDir()

	content := `pluginGit = ["lohko"]
logLevel = 2

[branch-staging]
logLevel = 5
`
	if err := os.WriteFile(filepath.Join(root, "defaults.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	settings := ProjectDefaultsFile(root, "")
	if got, ok := settings["logLevel"].(int); !ok || got != 2 {
		t.Fatalf("ProjectDefaultsFile()[logLevel] = %#v, want int 2", settings["logLevel"])
	}
	slice, ok := settings["pluginGit"].([]string)
	if !ok || len(slice) != 1 || slice[0] != "lohko" {
		t.Fatalf("ProjectDefaultsFile()[pluginGit] = %#v, want [lohko]", settings["pluginGit"])
	}

	staging := ProjectDefaultsFile(root, "staging")
	if got, ok := staging["logLevel"].(int); !ok || got != 5 {
		t.Fatalf(`ProjectDefaultsFile(root, "staging")[logLevel] = %#v, want int 5 (branch override)`, staging["logLevel"])
	}

	if got := ProjectDefaultsFile(t.TempDir(), ""); len(got) != 0 {
		t.Fatalf("ProjectDefaultsFile() for a project with no defaults.toml = %#v, want empty", got)
	}
}

func TestBranchOverride(t *testing.T) {
	root := t.TempDir()
	jcoreToml := `remoteDomain = 'kehitys.jcore.fi'
remoteHost = 'kehitys@vs.bojaco.com'
theme = 'ilme'

[branch-testing]
remoteDomain = 'stagingtest.jcore.fi'
remoteHost = 'staging@vs.bojaco.com'
`
	if err := os.WriteFile(filepath.Join(root, ProjectConfigName), []byte(jcoreToml), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("no branch uses top-level value", func(t *testing.T) {
		store, err := OpenStore(ScopeProject, root, "")
		if err != nil {
			t.Fatalf("OpenStore error = %v", err)
		}
		v, fromBranch, ok := store.getWithSource("remoteDomain")
		if !ok || fromBranch || v != "kehitys.jcore.fi" {
			t.Fatalf("getWithSource(remoteDomain) = %#v, %v, %v, want kehitys.jcore.fi, false, true", v, fromBranch, ok)
		}
	})

	t.Run("matching branch overrides", func(t *testing.T) {
		store, err := OpenStore(ScopeProject, root, "testing")
		if err != nil {
			t.Fatalf("OpenStore error = %v", err)
		}
		v, fromBranch, ok := store.getWithSource("remoteDomain")
		if !ok || !fromBranch || v != "stagingtest.jcore.fi" {
			t.Fatalf("getWithSource(remoteDomain) on branch testing = %#v, %v, %v, want stagingtest.jcore.fi, true, true", v, fromBranch, ok)
		}

		// theme has no override in [branch-testing], so it still falls
		// through to the top-level value.
		v, fromBranch, ok = store.getWithSource("theme")
		if !ok || fromBranch || v != "ilme" {
			t.Fatalf("getWithSource(theme) on branch testing = %#v, %v, %v, want ilme, false, true", v, fromBranch, ok)
		}
	})

	t.Run("different branch does not override", func(t *testing.T) {
		store, err := OpenStore(ScopeProject, root, "main")
		if err != nil {
			t.Fatalf("OpenStore error = %v", err)
		}
		v, fromBranch, ok := store.getWithSource("remoteDomain")
		if !ok || fromBranch || v != "kehitys.jcore.fi" {
			t.Fatalf("getWithSource(remoteDomain) on branch main = %#v, %v, %v, want kehitys.jcore.fi, false, true", v, fromBranch, ok)
		}
	})

	t.Run("Resolve reports FromBranchOverride", func(t *testing.T) {
		res, err := Resolve("remoteDomain", root, "testing")
		if err != nil {
			t.Fatalf("Resolve error = %v", err)
		}
		if res.SourceScope != ScopeProject || !res.FromBranchOverride || res.Value != "stagingtest.jcore.fi" {
			t.Fatalf("Resolve(remoteDomain, testing) = %+v, want project/stagingtest.jcore.fi/FromBranchOverride", res)
		}
	})

	t.Run("Set and Unset never touch the branch table", func(t *testing.T) {
		store, err := OpenStore(ScopeProject, root, "testing")
		if err != nil {
			t.Fatalf("OpenStore error = %v", err)
		}
		if err := store.Set("theme", "changed"); err != nil {
			t.Fatalf("Set error = %v", err)
		}
		if err := store.Unset("remoteHost"); err != nil {
			t.Fatalf("Unset error = %v", err)
		}
		if err := store.Save(); err != nil {
			t.Fatalf("Save error = %v", err)
		}

		// remoteHost's top-level value is gone, but the branch override
		// for it is untouched, so it's still what Get reports on that
		// branch — Unset only ever removes a top-level value.
		reloaded, err := OpenStore(ScopeProject, root, "testing")
		if err != nil {
			t.Fatalf("OpenStore (reload) error = %v", err)
		}
		v, fromBranch, ok := reloaded.getWithSource("remoteHost")
		if !ok || !fromBranch || v != "staging@vs.bojaco.com" {
			t.Fatalf("getWithSource(remoteHost) after Unset = %#v, %v, %v, want the branch override to still apply", v, fromBranch, ok)
		}

		raw, err := os.ReadFile(filepath.Join(root, ProjectConfigName))
		if err != nil {
			t.Fatal(err)
		}
		content := string(raw)
		if !strings.Contains(content, "[branch-testing]") {
			t.Fatalf("expected [branch-testing] table to survive Save, got:\n%s", content)
		}
		if !strings.Contains(content, "stagingtest.jcore.fi") || !strings.Contains(content, "staging@vs.bojaco.com") {
			t.Fatalf("expected branch override values to survive Save, got:\n%s", content)
		}
	})

	t.Run("SetForBranch writes into the branch table, not top-level", func(t *testing.T) {
		store, err := OpenStore(ScopeProject, root, "testing")
		if err != nil {
			t.Fatalf("OpenStore error = %v", err)
		}
		if err := store.SetForBranch("testing", "remoteDomain", "updated.jcore.fi"); err != nil {
			t.Fatalf("SetForBranch error = %v", err)
		}
		if err := store.Save(); err != nil {
			t.Fatalf("Save error = %v", err)
		}

		onBranch, err := OpenStore(ScopeProject, root, "testing")
		if err != nil {
			t.Fatal(err)
		}
		if v, fromBranch, ok := onBranch.getWithSource("remoteDomain"); !ok || !fromBranch || v != "updated.jcore.fi" {
			t.Fatalf("getWithSource(remoteDomain) on testing = %#v, %v, %v, want updated.jcore.fi, true, true", v, fromBranch, ok)
		}

		noBranch, err := OpenStore(ScopeProject, root, "")
		if err != nil {
			t.Fatal(err)
		}
		if v, ok := noBranch.Get("remoteDomain"); !ok || v != "kehitys.jcore.fi" {
			t.Fatalf("top-level remoteDomain = %#v, %v, want the original top-level value untouched", v, ok)
		}
	})

	t.Run("SetForBranch creates a new branch table if none existed", func(t *testing.T) {
		store, err := OpenStore(ScopeProject, root, "feature-x")
		if err != nil {
			t.Fatalf("OpenStore error = %v", err)
		}
		if err := store.SetForBranch("feature-x", "theme", "experimental"); err != nil {
			t.Fatalf("SetForBranch error = %v", err)
		}
		if err := store.Save(); err != nil {
			t.Fatalf("Save error = %v", err)
		}

		onBranch, err := OpenStore(ScopeProject, root, "feature-x")
		if err != nil {
			t.Fatal(err)
		}
		if v, ok := onBranch.Get("theme"); !ok || v != "experimental" {
			t.Fatalf("theme on feature-x = %#v, %v, want experimental, true", v, ok)
		}
		onMain, err := OpenStore(ScopeProject, root, "")
		if err != nil {
			t.Fatal(err)
		}
		// "changed" (not the original "ilme") because an earlier subtest
		// in this sequence already edited the top-level theme value; the
		// point here is just that SetForBranch didn't touch it further.
		if v, ok := onMain.Get("theme"); !ok || v != "changed" {
			t.Fatalf("top-level theme = %#v, %v, want it untouched by SetForBranch", v, ok)
		}
	})

	t.Run("UnsetForBranch removes only that branch's override", func(t *testing.T) {
		store, err := OpenStore(ScopeProject, root, "testing")
		if err != nil {
			t.Fatalf("OpenStore error = %v", err)
		}
		if err := store.UnsetForBranch("testing", "remoteDomain"); err != nil {
			t.Fatalf("UnsetForBranch error = %v", err)
		}
		if err := store.Save(); err != nil {
			t.Fatalf("Save error = %v", err)
		}

		reloaded, err := OpenStore(ScopeProject, root, "testing")
		if err != nil {
			t.Fatal(err)
		}
		// remoteDomain's branch override is gone, so it falls through to
		// the top-level value.
		if v, fromBranch, ok := reloaded.getWithSource("remoteDomain"); !ok || fromBranch || v != "kehitys.jcore.fi" {
			t.Fatalf("getWithSource(remoteDomain) after UnsetForBranch = %#v, %v, %v, want kehitys.jcore.fi, false, true", v, fromBranch, ok)
		}
		// remoteHost's branch override (set earlier in this test) is
		// untouched.
		if v, fromBranch, ok := reloaded.getWithSource("remoteHost"); !ok || !fromBranch || v != "staging@vs.bojaco.com" {
			t.Fatalf("getWithSource(remoteHost) after unrelated UnsetForBranch = %#v, %v, %v, want it untouched", v, fromBranch, ok)
		}
	})
}

func TestLoadTOMLWithBranchOverlay(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "jcore.toml")
	content := `mode = 'fg'

[branch-staging]
mode = 'bg'
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	merged, err := LoadTOMLWithBranchOverlay(path, "staging")
	if err != nil {
		t.Fatalf("LoadTOMLWithBranchOverlay error = %v", err)
	}
	if merged["mode"] != "bg" {
		t.Fatalf("merged[mode] = %#v, want bg", merged["mode"])
	}
	if _, ok := merged["branch-staging"]; ok {
		t.Fatal("expected the raw branch-staging table itself to be excluded from the merged result")
	}

	merged, err = LoadTOMLWithBranchOverlay(path, "main")
	if err != nil {
		t.Fatalf("LoadTOMLWithBranchOverlay error = %v", err)
	}
	if merged["mode"] != "fg" {
		t.Fatalf("merged[mode] on a non-matching branch = %#v, want fg", merged["mode"])
	}

	merged, err = LoadTOMLWithBranchOverlay(filepath.Join(root, "missing.toml"), "staging")
	if err != nil || len(merged) != 0 {
		t.Fatalf("LoadTOMLWithBranchOverlay on a missing file = %#v, %v, want empty map, nil error", merged, err)
	}
}

func TestValidateScope(t *testing.T) {
	cases := []struct {
		name        string
		key         string
		scope       Scope
		projectRoot string
		wantErr     bool
	}{
		{"project-eligible key at project scope, in project", "projectName", ScopeProject, "/tmp/proj", false},
		{"project-eligible key at project scope, no project", "projectName", ScopeProject, "", true},
		{"project-eligible key at local scope, no project", "projectName", ScopeLocal, "", true},
		{"global-only key at global scope", "debug", ScopeGlobal, "", false},
		{"global-only key at project scope errors", "debug", ScopeProject, "/tmp/proj", true},
		{"global-only key at local scope is allowed (legacy quirk)", "debug", ScopeLocal, "/tmp/proj", false},
		{"unknown key is unrestricted", "someFutureKey", ScopeProject, "/tmp/proj", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateScope(tc.key, tc.scope, tc.projectRoot)
			if tc.wantErr != (err != nil) {
				t.Fatalf("ValidateScope(%q, %v, %q) error = %v, wantErr %v", tc.key, tc.scope, tc.projectRoot, err, tc.wantErr)
			}
		})
	}
}
