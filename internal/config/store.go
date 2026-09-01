package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/JCO-Digital/jcore/container"
	"github.com/pelletier/go-toml/v2"
)

// Store reads and writes exactly one scope's TOML file. Unlike the
// package-level viper singleton (which merges every scope together and
// lower-cases every key internally, corrupting the camelCase keys used
// throughout jcore.toml/config.toml on write), Store operates on a plain
// map decoded/encoded with go-toml/v2 directly, so key casing round-trips
// exactly and each scope can be inspected independently.
//
// A file may also contain one or more "branch-<name>" tables — per-branch
// overrides, e.g. a different remoteHost while on a "staging" branch. Store
// keeps these separate from the top-level settings (data) so Get/All can
// report the branch-adjusted effective value while Set/Unset only ever
// touch the top-level settings, and Save round-trips every branch table
// whether or not it matches the currently active branch.
type Store struct {
	scope    Scope
	path     string
	branch   string
	data     map[string]any
	branches map[string]map[string]any
}

// OpenStore loads the TOML file for scope. A missing file is not an error —
// it just yields an empty Store, ready to be written to via Set/Save.
// branch is the currently checked-out git branch (see project.CurrentBranch);
// pass "" to disable branch overrides entirely (e.g. outside a project).
func OpenStore(scope Scope, projectRoot string, branch string) (*Store, error) {
	path, err := GetConfigPath(scope, projectRoot)
	if err != nil {
		return nil, err
	}

	decoded, err := parseTOMLFile(path)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	data, branches := splitBranchTables(decoded)

	return &Store{scope: scope, path: path, branch: branch, data: data, branches: branches}, nil
}

// Path returns the on-disk file this Store reads/writes.
func (s *Store) Path() string {
	return s.path
}

// Get returns the effective value of key in this scope's file only (not
// merged with any other scope): the active branch's override if one exists
// for key, otherwise the top-level value. Normalized to the Go type
// matching the setting's schema type where known.
func (s *Store) Get(key string) (any, bool) {
	v, _, ok := s.getWithSource(key)
	return v, ok
}

// getWithSource is like Get but also reports whether the value came from
// the active branch's override table rather than the top-level settings.
func (s *Store) getWithSource(key string) (value any, fromBranchOverride bool, ok bool) {
	key = CanonicalKey(key)
	if override, has := s.branches[s.branch]; s.branch != "" && has {
		if v, ok := override[key]; ok {
			return normalizeValue(v), true, true
		}
	}
	v, ok := s.data[key]
	if !ok {
		return nil, false, false
	}
	return normalizeValue(v), false, true
}

// SourceIsBranchOverride reports whether key's effective value in this
// scope's file comes from the active branch's override table rather than
// its top-level settings.
func (s *Store) SourceIsBranchOverride(key string) bool {
	_, fromBranch, _ := s.getWithSource(key)
	return fromBranch
}

// All returns every key/value explicitly set in this scope's file
// (branch-adjusted), with values normalized to plain Go types.
func (s *Store) All() map[string]any {
	effective := applyBranch(s.data, s.branches, s.branch)
	out := make(map[string]any, len(effective))
	for k, v := range effective {
		out[k] = normalizeValue(v)
	}
	return out
}

// Set stores value for key in this scope's top-level settings, coercing it
// to the type declared in the setting's schema (if key is a known
// setting). rawValue may already be the correct Go type (e.g. a bool from
// a TUI toggle) or a string (e.g. from `jcore config set`, or a
// comma-separated list for TypeStringSlice). Use SetForBranch instead to
// write into a specific branch override table.
func (s *Store) Set(key string, rawValue any) error {
	key, value, err := canonicalizeAndCoerce(key, rawValue)
	if err != nil {
		return err
	}
	s.data[key] = value
	return nil
}

// SetForBranch stores value for key inside the "branch-<branch>" override
// table, creating that table if it doesn't exist yet. Use this instead of
// Set when editing a value that's currently effective *because* of that
// branch's override (Resolution.FromBranchOverride) — writing to the
// top-level settings instead wouldn't change anything observable, since
// the branch override still takes precedence over them.
func (s *Store) SetForBranch(branch, key string, rawValue any) error {
	key, value, err := canonicalizeAndCoerce(key, rawValue)
	if err != nil {
		return err
	}
	if s.branches == nil {
		s.branches = map[string]map[string]any{}
	}
	table := s.branches[branch]
	if table == nil {
		table = map[string]any{}
		s.branches[branch] = table
	}
	table[key] = value
	return nil
}

func canonicalizeAndCoerce(key string, rawValue any) (canonicalKey string, value any, err error) {
	canonicalKey = CanonicalKey(key)
	value = rawValue
	if def, ok := Lookup(canonicalKey); ok {
		coerced, err := coerceValue(def, rawValue)
		if err != nil {
			return canonicalKey, nil, err
		}
		value = coerced
	}
	return canonicalKey, value, nil
}

// Unset removes key from this scope's file's top-level settings. It's a
// no-op if key wasn't set there. It never touches branch override tables —
// use UnsetForBranch for that — so if the active branch overrides key, Get
// still returns that override's value after Unset.
func (s *Store) Unset(key string) error {
	delete(s.data, CanonicalKey(key))
	return nil
}

// UnsetForBranch removes key from the "branch-<branch>" override table, if
// that table exists and sets it. It's a no-op otherwise.
func (s *Store) UnsetForBranch(branch, key string) error {
	if table, ok := s.branches[branch]; ok {
		delete(table, CanonicalKey(key))
	}
	return nil
}

// Save writes the store's current contents back to its file: the
// top-level settings plus every branch override table, verbatim.
func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}

	out := make(map[string]any, len(s.data)+len(s.branches))
	for k, v := range s.data {
		out[k] = v
	}
	for name, table := range s.branches {
		out["branch-"+name] = table
	}

	encoded, err := toml.Marshal(out)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, encoded, 0644)
}

// Resolution is the outcome of resolving a setting's effective value across
// every applicable scope.
type Resolution struct {
	Value       any
	SourceScope Scope
	// IsDefault is true when no scope overrides the setting; Value is then
	// the schema default (or nil, if the setting has none).
	IsDefault bool
	// FromBranchOverride is true when Value came from a "branch-<name>"
	// table matching the currently checked-out branch, rather than
	// SourceScope's top-level settings.
	FromBranchOverride bool
}

// Resolve returns the effective value of key, checking every layer in the
// exact same precedence order the running CLI itself merges them in (see
// cmd/root.go's initConfig): Local overrides Project (jcore.toml) overrides
// the project's own defaults.toml overrides Global overrides the embedded
// base defaults.toml overrides the setting's schema default. Local and
// Project layers are only checked when projectRoot is non-empty. Within
// each of those files, a "branch-<branch>" table (if present and branch is
// non-empty) overrides that same file's own top-level values.
//
// Only Local, Project, and Global are scopes the TUI/`config set` can write
// to — a value sourced from the project defaults.toml or the embedded
// defaults is still reported as IsDefault (nothing you can override with
// `config set` is actually setting it), just with the real, non-empty value
// instead of the (often empty) schema.SettingDef.Default placeholder.
func Resolve(key string, projectRoot string, branch string) (Resolution, error) {
	key = CanonicalKey(key)
	if projectRoot != "" {
		for _, scope := range []Scope{ScopeLocal, ScopeProject} {
			store, err := OpenStore(scope, projectRoot, branch)
			if err != nil {
				continue
			}
			if v, fromBranch, ok := store.getWithSource(key); ok {
				return Resolution{Value: v, SourceScope: scope, FromBranchOverride: fromBranch}, nil
			}
		}

		if v, ok := projectDefaultsFile(projectRoot, branch)[key]; ok {
			return Resolution{Value: normalizeValue(v), IsDefault: true}, nil
		}
	}

	if store, err := OpenStore(ScopeGlobal, projectRoot, branch); err == nil {
		if v, fromBranch, ok := store.getWithSource(key); ok {
			return Resolution{Value: v, SourceScope: ScopeGlobal, FromBranchOverride: fromBranch}, nil
		}
	}

	if v, ok := embeddedBaseDefaults()[key]; ok {
		return Resolution{Value: normalizeValue(v), IsDefault: true}, nil
	}

	if def, ok := Lookup(key); ok && def.Default != nil {
		return Resolution{Value: def.Default, IsDefault: true}, nil
	}
	return Resolution{IsDefault: true}, nil
}

var embeddedBaseDefaultsCache map[string]any

// embeddedBaseDefaults reads and caches container/base/defaults.toml, the
// lowest-priority layer merged by every jcore invocation. It's compiled
// into the binary, so branch overrides don't apply to it.
func embeddedBaseDefaults() map[string]any {
	if embeddedBaseDefaultsCache != nil {
		return embeddedBaseDefaultsCache
	}

	data := map[string]any{}
	if raw, err := container.BaseAssets.ReadFile("base/defaults.toml"); err == nil {
		_ = toml.Unmarshal(raw, &data)
	}
	embeddedBaseDefaultsCache = data
	return data
}

// projectDefaultsFile reads <projectRoot>/defaults.toml (the per-template
// defaults file scaffolded into every project, distinct from jcore.toml),
// branch-adjusted. A missing or unparsable file just yields no defaults,
// not an error.
func projectDefaultsFile(projectRoot string, branch string) map[string]any {
	decoded, err := parseTOMLFile(filepath.Join(projectRoot, "defaults.toml"))
	if err != nil {
		return map[string]any{}
	}
	top, branches := splitBranchTables(decoded)
	return applyBranch(top, branches, branch)
}

// LoadTOMLWithBranchOverlay reads path (a TOML file) and returns its
// top-level settings with the matching "branch-<branch>" table's keys (if
// any) merged in, mirroring the legacy TypeScript CLI's per-file branch
// override feature. Used by cmd/root.go to apply the same overlay when
// populating the real, merged viper config every command runs against —
// not just this package's own Resolve. A missing file yields an empty map,
// not an error.
func LoadTOMLWithBranchOverlay(path string, branch string) (map[string]any, error) {
	decoded, err := parseTOMLFile(path)
	if err != nil {
		return nil, err
	}
	top, branches := splitBranchTables(decoded)
	return applyBranch(top, branches, branch), nil
}

// parseTOMLFile reads and decodes path. A missing file yields an empty map,
// not an error.
func parseTOMLFile(path string) (map[string]any, error) {
	data := map[string]any{}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return nil, err
	}
	if err := toml.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// splitBranchTables separates a decoded TOML file's top-level settings from
// any "branch-<name>" override tables, folding every key (including inside
// each override table) to its canonical case.
func splitBranchTables(decoded map[string]any) (top map[string]any, branches map[string]map[string]any) {
	top = make(map[string]any, len(decoded))
	branches = map[string]map[string]any{}

	for k, v := range decoded {
		if name, ok := strings.CutPrefix(k, "branch-"); ok {
			if table, ok := v.(map[string]any); ok {
				canon := make(map[string]any, len(table))
				for tk, tv := range table {
					canon[CanonicalKey(tk)] = tv
				}
				branches[name] = canon
				continue
			}
		}
		top[CanonicalKey(k)] = v
	}
	return top, branches
}

// applyBranch returns top with branches[branch]'s keys merged over it, if
// that table exists; top itself is never mutated.
func applyBranch(top map[string]any, branches map[string]map[string]any, branch string) map[string]any {
	override, ok := branches[branch]
	if branch == "" || !ok {
		return top
	}
	merged := make(map[string]any, len(top)+len(override))
	for k, v := range top {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	return merged
}

// ValidateScope reports whether key may be written at requestedScope, given
// whether we're currently inside a project (projectRoot != ""). This mirrors
// the legacy TypeScript CLI's validateScope, including one intentional
// asymmetry it had: a global-only setting may still be written at Local
// scope, even though it may not be written at Project scope. That's kept
// as-is for behavioral fidelity, not fixed as a bug.
func ValidateScope(key string, requestedScope Scope, projectRoot string) error {
	if projectRoot == "" && (requestedScope == ScopeLocal || requestedScope == ScopeProject) {
		return fmt.Errorf("not in a jcore project")
	}

	def, known := Lookup(CanonicalKey(key))
	if !known {
		return nil
	}
	if def.ScopeClass == ScopeClassGlobalOnly && requestedScope == ScopeProject {
		return fmt.Errorf("%s is a global-only setting; use global or local scope", key)
	}
	return nil
}

// normalizeValue converts go-toml/v2's decoded types (int64, []any) into
// the plain Go types the rest of the codebase expects (int, []string).
func normalizeValue(v any) any {
	switch val := v.(type) {
	case int64:
		return int(val)
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			out = append(out, fmt.Sprintf("%v", item))
		}
		return out
	default:
		return v
	}
}

func coerceValue(def SettingDef, raw any) (any, error) {
	switch def.Type {
	case TypeBool:
		switch v := raw.(type) {
		case bool:
			return v, nil
		case string:
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, fmt.Errorf("invalid bool value %q for %s", v, def.Key)
			}
			return b, nil
		default:
			return nil, fmt.Errorf("cannot use %T as bool for %s", raw, def.Key)
		}
	case TypeInt:
		switch v := raw.(type) {
		case int:
			return v, nil
		case int64:
			return int(v), nil
		case string:
			i, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("invalid int value %q for %s", v, def.Key)
			}
			return i, nil
		default:
			return nil, fmt.Errorf("cannot use %T as int for %s", raw, def.Key)
		}
	case TypeStringSlice:
		switch v := raw.(type) {
		case []string:
			return v, nil
		case string:
			return splitList(v), nil
		default:
			return nil, fmt.Errorf("cannot use %T as string slice for %s", raw, def.Key)
		}
	default: // TypeString
		if s, ok := raw.(string); ok {
			return s, nil
		}
		return fmt.Sprintf("%v", raw), nil
	}
}

// splitList parses a comma-separated string into a trimmed, non-empty
// string slice — the TUI's simplified editing format for TypeStringSlice
// settings.
func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
