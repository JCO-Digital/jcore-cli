package config

import "strings"

// ValueType is the on-disk/TOML type of a setting's value.
type ValueType int

const (
	TypeString ValueType = iota
	TypeBool
	TypeInt
	TypeStringSlice
)

// ScopeClass says which scopes a setting may be written to. Ported from the
// legacy TypeScript CLI's settingsSchema/projectSettings classification.
type ScopeClass int

const (
	// ScopeClassProjectEligible settings may be set at Global, Project, or
	// Local. This is the zero value/default so that SettingDef literals
	// below only need to opt *in* to ScopeClassGlobalOnly explicitly,
	// rather than every project-eligible entry (the majority) needing to
	// spell out its class.
	ScopeClassProjectEligible ScopeClass = iota
	// ScopeClassGlobalOnly settings may only be set at ScopeGlobal.
	ScopeClassGlobalOnly
)

// SettingDef describes one known jcore setting.
type SettingDef struct {
	// Key is the exact viper/mapstructure/TOML key.
	Key string
	// Type is the value's on-disk type, used to coerce raw string input
	// (e.g. from `jcore config set`) into the correct TOML type.
	Type ValueType
	// Default is the effective value when nothing overrides it in any
	// scope. nil means "no default; empty/zero value if unset".
	Default any
	// Description is a one-line human explanation, shown in the TUI.
	Description string
	// Category groups the setting for display (e.g. in the TUI menu).
	Category string
	// ScopeClass says whether this can be set at project/local scope, or
	// only globally.
	ScopeClass ScopeClass
	// Sensitive settings (credentials) should be masked in UIs.
	Sensitive bool
	// Options, if non-empty, is the fixed set of known values this setting
	// accepts. The TUI offers these as a select list instead of free-text
	// entry. It's advisory, not enforced: a value already set outside this
	// list (e.g. hand-edited, or written by an older version of jcore with
	// a different set of known values) stays editable and is offered
	// alongside Options rather than rejected.
	Options []string
}

// Settings is the canonical, ordered list of every known jcore setting.
// Order is intentional (grouped by Category) and drives display order.
var Settings = []SettingDef{
	// Project
	{Key: "projectName", Type: TypeString, Category: "Project", Description: "Name of the project."},
	{Key: "template", Type: TypeString, Default: "jcore3", Category: "Project", ScopeClass: ScopeClassGlobalOnly, Description: "Template used to scaffold new projects."},
	{Key: "theme", Type: TypeString, Default: "jcore-ilme", Category: "Project", Description: "WordPress theme to install."},
	{Key: "branch", Type: TypeString, Category: "Project", Description: "Git branch of the theme/plugins to track."},

	// Domains
	{Key: "remoteDomain", Type: TypeString, Category: "Domains", Description: "Domain of the remote/production site."},
	{Key: "localDomain", Type: TypeString, Category: "Domains", Description: "Domain used for the local development site."},
	{Key: "domains", Type: TypeStringSlice, Category: "Domains", Description: "Additional local domain aliases."},
	{Key: "replace", Type: TypeStringSlice, Category: "Domains", Description: "Extra search/replace pairs applied when pulling the database."},

	// WordPress
	{Key: "wpImage", Type: TypeString, Default: "jcodigi/wordpress:latest", Category: "WordPress", Description: "Docker image used for the WordPress container."},
	{Key: "wpVersion", Type: TypeString, Default: "latest", Category: "WordPress", Description: "WordPress version to install."},
	{Key: "wpDebug", Type: TypeBool, Default: true, Category: "WordPress", Description: "Enable WP_DEBUG."},
	{Key: "wpDebugLog", Type: TypeBool, Default: false, Category: "WordPress", Description: "Enable WP_DEBUG_LOG."},
	{Key: "wpDebugDisplay", Type: TypeBool, Default: true, Category: "WordPress", Description: "Enable WP_DEBUG_DISPLAY."},

	// Database
	{Key: "dbPrefix", Type: TypeString, Default: "wp_", Category: "Database", Description: "Database table prefix."},
	{Key: "dbExclude", Type: TypeStringSlice, Category: "Database", Description: "Database tables excluded when pulling/pushing."},
	{Key: "wpDbName", Type: TypeString, Category: "Database", Description: "Remote database name override."},
	{Key: "wpDbUser", Type: TypeString, Category: "Database", Description: "Remote database user override."},
	{Key: "wpDbPassword", Type: TypeString, Category: "Database", Sensitive: true, Description: "Remote database password override."},

	// Plugins
	// "composer" is a deprecated third value jcore refuses to run most
	// commands with (see cmd.refusePluginInstallComposer) — deliberately
	// left out of Options so the TUI's select list only ever offers the
	// two supported values, while still showing "composer" as an editable
	// (if unlisted) current value for a project that still has it set.
	{Key: "pluginInstall", Type: TypeString, Category: "Plugins", Description: "How plugins are installed: remote (default) or local. \"composer\" is deprecated.", Options: []string{"remote", "local"}},
	{Key: "pluginExclude", Type: TypeStringSlice, Category: "Plugins", Description: "Plugins excluded from install/sync."},
	{Key: "pluginGit", Type: TypeStringSlice, Category: "Plugins", Description: "Plugins installed from their own git repository."},
	{Key: "pluginLocal", Type: TypeStringSlice, Category: "Plugins", ScopeClass: ScopeClassGlobalOnly, Description: "Plugins symlinked from a local checkout."},

	// Deployment
	{Key: "remoteHost", Type: TypeString, Category: "Deployment", Description: "SSH host of the remote/production server."},
	{Key: "remotePath", Type: TypeString, Category: "Deployment", Description: "Remote path to the WordPress installation."},

	// CLI Behavior
	{Key: "debug", Type: TypeBool, Default: false, Category: "CLI Behavior", ScopeClass: ScopeClassGlobalOnly, Description: "Print debug output."},
	{Key: "logLevel", Type: TypeInt, Default: 2, Category: "CLI Behavior", ScopeClass: ScopeClassGlobalOnly, Description: "Numeric log verbosity level."},
	{Key: "mode", Type: TypeString, Default: "foreground", Category: "CLI Behavior", ScopeClass: ScopeClassGlobalOnly, Description: "Run containers in the foreground or background.", Options: []string{"foreground", "background"}},
	{Key: "install", Type: TypeBool, Default: true, Category: "CLI Behavior", ScopeClass: ScopeClassGlobalOnly, Description: "Install node/composer dependencies automatically."},
	{Key: "verbose", Type: TypeBool, Default: false, Category: "CLI Behavior", ScopeClass: ScopeClassGlobalOnly, Description: "Print more output."},
	{Key: "projectDefault", Type: TypeString, Category: "CLI Behavior", ScopeClass: ScopeClassGlobalOnly, Description: "Git URL template used by `jcore clone`."},
}

var (
	settingsByKey      map[string]SettingDef
	settingsByLowerKey map[string]string
)

func init() {
	settingsByKey = make(map[string]SettingDef, len(Settings))
	settingsByLowerKey = make(map[string]string, len(Settings))
	for _, s := range Settings {
		settingsByKey[s.Key] = s
		settingsByLowerKey[strings.ToLower(s.Key)] = s.Key
	}
}

// Lookup returns the SettingDef for key, if known. key must already be in
// its canonical case (see CanonicalKey) — Lookup itself does not fold case.
func Lookup(key string) (SettingDef, bool) {
	def, ok := settingsByKey[key]
	return def, ok
}

// CanonicalKey returns the correctly-cased key for a known setting, matched
// case-insensitively (e.g. "wpimage" -> "wpImage"), or key unchanged if it
// doesn't match any known setting. TOML files written by versions of this
// CLI predating Store (which used viper's WriteConfig, silently lower-casing
// every key) can contain keys in the wrong case; this lets reads still find
// them, and a subsequent Save() rewrites them back to the correct case.
func CanonicalKey(key string) string {
	if canonical, ok := settingsByLowerKey[strings.ToLower(key)]; ok {
		return canonical
	}
	return key
}

// Categories returns the ordered, de-duplicated list of setting categories,
// in the order they first appear in Settings.
func Categories() []string {
	seen := make(map[string]bool)
	var categories []string
	for _, s := range Settings {
		if !seen[s.Category] {
			seen[s.Category] = true
			categories = append(categories, s.Category)
		}
	}
	return categories
}

// InCategory returns the settings belonging to category, in Settings order.
func InCategory(category string) []SettingDef {
	var defs []SettingDef
	for _, s := range Settings {
		if s.Category == category {
			defs = append(defs, s)
		}
	}
	return defs
}
