package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/JCO-Digital/jcore/internal/config"
)

// LegacyConfigFilename is the pre-jcore.toml, bash-based project config file.
const LegacyConfigFilename = "config.sh"

var (
	legacyCommentRe   = regexp.MustCompile(`(?m) *#.*$`)
	legacyAssignRe    = regexp.MustCompile(`(?m)^([A-Z_]+)= *([^(].*)$`)
	legacyArrayRe     = regexp.MustCompile(`(?m)^([A-Z_]+)= ?\(\s*([^)]+)\s*\)`)
	legacyVarRefRe    = regexp.MustCompile(`\$([A-Z_]+)`)
	legacyQuoteTrimRe = regexp.MustCompile(`(?m)^["' ]+|["' ]+$`)
)

// legacyTemplateSubmodules maps a parent theme committed as a git submodule
// in a legacy project to the template it was built on, most specific first.
var legacyTemplateSubmodules = []struct{ path, template string }{
	{"wp-content/themes/jcore2", "jcore2"},
	{"wp-content/themes/jcore", "jcore1"},
}

// DetectLegacyTemplate guesses which template a legacy project was built on
// from the parent theme committed to it as a submodule (a gitlink in the
// index), falling back to "blank". .gitmodules alone isn't trusted: old
// projects sometimes list a submodule there that was never committed.
func DetectLegacyTemplate(projectDir string) string {
	args := []string{"ls-files", "--stage", "--"}
	for _, s := range legacyTemplateSubmodules {
		args = append(args, s.path)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = projectDir
	out, err := cmd.Output()
	if err != nil {
		return "blank"
	}

	gitlinks := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		// "<mode> <object> <stage>\t<path>"; submodules have mode 160000.
		meta, path, ok := strings.Cut(line, "\t")
		if ok && strings.HasPrefix(meta, "160000 ") {
			gitlinks[path] = true
		}
	}
	for _, s := range legacyTemplateSubmodules {
		if gitlinks[s.path] {
			return s.template
		}
	}
	return "blank"
}

// ConvertLegacyConfig parses a legacy config.sh file in projectDir and writes
// an equivalent jcore.toml alongside it, using the given template.
func ConvertLegacyConfig(projectDir string, template string) error {
	data, err := os.ReadFile(filepath.Join(projectDir, LegacyConfigFilename))
	if err != nil {
		return err
	}

	values := map[string]any{"name": filepath.Base(projectDir)}
	parseLegacySettings(values, string(data))
	applyVagrantLegacySettings(projectDir, values)

	var newDomain, newLocal string
	var domains, replace []string
	if rawDomains, ok := values["domains"].([]string); ok {
		for _, domain := range rawDomains {
			parts := strings.SplitN(domain, ";", 2)
			if len(parts) < 2 {
				continue
			}
			local := parts[1] + ".localhost"
			replace = append(replace, "//"+parts[0]+"|//"+local)
			if !slices.Contains(domains, local) {
				domains = append(domains, local)
			}
			if newDomain == "" {
				newDomain = parts[0]
				newLocal = local
			}
		}
	}

	// Deliberately uses config.Store rather than a bare *viper.Viper: viper
	// lower-cases every key on WriteConfig, which previously corrupted the
	// camelCase keys (projectName, remoteHost, ...) this function writes.
	store, err := config.OpenStore(config.ScopeProject, projectDir, "")
	if err != nil {
		return err
	}
	_ = store.Set("projectName", values["name"])
	_ = store.Set("template", template)
	setIfPresent(store, "theme", values["theme"])
	setIfPresent(store, "branch", values["branch"])
	setIfPresent(store, "remoteHost", values["remotehost"])
	setIfPresent(store, "remotePath", values["remotepath"])
	_ = store.Set("replace", replace)
	_ = store.Set("domains", domains)
	_ = store.Set("remoteDomain", newDomain)
	_ = store.Set("localDomain", newLocal)
	setIfPresent(store, "dbExclude", values["db_exclude"])
	setIfPresent(store, "pluginExclude", values["plugin_exclude"])
	setIfPresent(store, "pluginGit", values["plugin_git"])
	setIfPresent(store, "pluginInstall", values["plugin_install"])
	_ = store.Set("install", values["install"] == "true")

	return store.Save()
}

// applyVagrantLegacySettings maps the oldest, Vagrant-era config.sh format
// (WPENAME/WEB/PRODUCTION/STAGING instead of REMOTEHOST/REMOTEPATH/DOMAINS)
// onto the newer legacy keys, so the rest of ConvertLegacyConfig handles
// both. Keys already present in the newer format are left alone.
func applyVagrantLegacySettings(projectDir string, values map[string]any) {
	name, _ := values["name"].(string)

	if wpe, ok := values["wpename"].(string); ok && isResolvedLegacyValue(wpe) {
		if _, exists := values["remotehost"]; !exists {
			values["remotehost"] = wpe + "@" + wpe + ".ssh.wpengine.net"
		}
		if _, exists := values["remotepath"]; !exists {
			values["remotepath"] = "/sites/" + wpe
		}
	}

	if _, exists := values["domains"]; !exists {
		var domains []string
		for _, key := range []string{"production", "staging", "web"} {
			host, ok := values[key].(string)
			if !ok || !isResolvedLegacyValue(host) {
				continue
			}
			if entry := host + ";" + name; !slices.Contains(domains, entry) {
				domains = append(domains, entry)
			}
		}
		if len(domains) > 0 {
			values["domains"] = domains
		}
	}

	// Vagrant-era configs set THEME=$NAME, which often isn't the real theme
	// directory. If it doesn't exist and there's exactly one theme, use that.
	if theme, ok := values["theme"].(string); ok {
		themesDir := filepath.Join(projectDir, "wp-content", "themes")
		if _, err := os.Stat(filepath.Join(themesDir, theme)); os.IsNotExist(err) {
			if only := singleThemeDir(themesDir); only != "" {
				values["theme"] = only
			}
		}
	}
}

// isResolvedLegacyValue reports whether val is non-empty and has no
// remaining unresolved $VAR references.
func isResolvedLegacyValue(val string) bool {
	return val != "" && !strings.Contains(val, "$")
}

// singleThemeDir returns the name of the only directory in themesDir, or ""
// if there are none or several.
func singleThemeDir(themesDir string) string {
	entries, err := os.ReadDir(themesDir)
	if err != nil {
		return ""
	}
	found := ""
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if found != "" {
			return ""
		}
		found = e.Name()
	}
	return found
}

// setIfPresent sets key on store only when val is a non-nil parsed value,
// so fields absent from the legacy config are simply omitted from
// jcore.toml.
func setIfPresent(store *config.Store, key string, val any) {
	if val != nil {
		_ = store.Set(key, val)
	}
}

// parseLegacySettings parses BASH variable assignments and arrays from data
// into values, substituting references to previously parsed variables.
func parseLegacySettings(values map[string]any, data string) {
	clean := legacyCommentRe.ReplaceAllString(data, "")

	for _, m := range legacyAssignRe.FindAllStringSubmatch(clean, -1) {
		values[strings.ToLower(m[1])] = cleanLegacyVar(values, m[2])
	}

	for _, m := range legacyArrayRe.FindAllStringSubmatch(clean, -1) {
		var arr []string
		for _, row := range strings.Split(m[2], "\n") {
			if text := cleanLegacyVar(values, row); text != "" {
				arr = append(arr, text)
			}
		}
		values[strings.ToLower(m[1])] = arr
	}
}

// cleanLegacyVar strips surrounding quotes and substitutes $VAR references
// with previously parsed string values.
func cleanLegacyVar(values map[string]any, text string) string {
	value := legacyQuoteTrimRe.ReplaceAllString(text, "")

	for _, m := range legacyVarRefRe.FindAllStringSubmatch(value, -1) {
		if str, ok := values[strings.ToLower(m[1])].(string); ok {
			value = strings.Replace(value, m[0], str, 1)
		}
	}

	return value
}
