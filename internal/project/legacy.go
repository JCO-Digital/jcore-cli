package project

import (
	"os"
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

// ConvertLegacyConfig parses a legacy config.sh file in projectDir and writes
// an equivalent jcore.toml alongside it.
func ConvertLegacyConfig(projectDir string) error {
	data, err := os.ReadFile(filepath.Join(projectDir, LegacyConfigFilename))
	if err != nil {
		return err
	}

	values := map[string]any{"name": filepath.Base(projectDir)}
	parseLegacySettings(values, string(data))

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
	_ = store.Set("template", "jcore2")
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
