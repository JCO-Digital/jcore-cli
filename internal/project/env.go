package project

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/viper"
)

// GenerateEnvFile creates a .env file from current configuration
func GenerateEnvFile(projectDir string) error {
	// 1. Start with values from env-values.toml (if it exists)
	envMap := make(map[string]string)

	envValuesPath := filepath.Join(projectDir, "env-values.toml")
	if _, err := os.Stat(envValuesPath); err == nil {
		content, err := os.ReadFile(envValuesPath)
		if err == nil {
			// Basic parsing of the TOML to preserve key casing
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.Trim(strings.TrimSpace(parts[1]), "\"")
					envMap[camelToSnake(key)] = value
				}
			}
		}
	}

	// 2. Overlay values from Viper (Project settings)
	// We manually map known keys to ensure correct snake_case with underscores
	settingsMap := map[string]string{
		"projectName":    "PROJECT_NAME",
		"localDomain":    "LOCAL_DOMAIN",
		"domains":        "DOMAINS",
		"remoteDomain":   "REMOTE_DOMAIN",
		"wpImage":        "WP_IMAGE",
		"wpVersion":      "WP_VERSION",
		"dbPrefix":       "DB_PREFIX",
		"wpDbName":       "WP_DB_NAME",
		"wpDbUser":       "WP_DB_USER",
		"wpDbPassword":   "WP_DB_PASSWORD",
		"wpDebug":        "WP_DEBUG",
		"wpDebugLog":     "WP_DEBUG_LOG",
		"wpDebugDisplay": "WP_DEBUG_DISPLAY",
		"remoteHost":     "REMOTE_HOST",
		"remotePath":     "REMOTE_PATH",
		"dbExclude":      "DB_EXCLUDE",
		"replace":        "REPLACE",
		"pluginExclude":  "PLUGIN_EXCLUDE",
		"pluginGit":      "PLUGIN_GIT",
		"pluginInstall":  "PLUGIN_INSTALL",
		"pluginLocal":    "PLUGIN_LOCAL",
	}

	for configKey, envName := range settingsMap {
		if viper.IsSet(configKey) {
			envMap[envName] = formatEnvValue(viper.Get(configKey))
		}
	}

	// 3. Ensure defaults for critical Compose variables if still missing
	if envMap["WP_IMAGE"] == "" {
		envMap["WP_IMAGE"] = "jcodigi/wordpress:latest"
	}
	if envMap["LOCAL_DOMAIN"] == "" {
		// Fallback for a project whose jcore.toml predates `init` seeding
		// this itself — ".localhost" matches the convention used
		// everywhere else (legacy CLI project creation, legacy-config
		// migration).
		projectName := viper.GetString("projectName")
		if projectName != "" {
			envMap["LOCAL_DOMAIN"] = Slugify(projectName) + ".localhost"
		} else {
			envMap["LOCAL_DOMAIN"] = "localhost"
		}
	}
	if envMap["DOMAINS"] == "" {
		envMap["DOMAINS"] = envMap["LOCAL_DOMAIN"]
	}

	// 4. Always ensure the base domain search/replace row is present in
	// REPLACE, on top of the user's own `replace` setting (if any) —
	// mirroring the legacy TypeScript CLI's createEnv(), which
	// unconditionally added this row so `jcore pull db` rewrites the
	// remote domain to the local one even with no explicit `replace`
	// setting configured.
	defaultRow := fmt.Sprintf("//%s|//%s", envMap["REMOTE_DOMAIN"], envMap["LOCAL_DOMAIN"])
	replaceRows := viper.GetStringSlice("replace")
	if !slices.Contains(replaceRows, defaultRow) {
		replaceRows = append([]string{defaultRow}, replaceRows...)
	}
	envMap["REPLACE"] = formatEnvValue(replaceRows)

	// 5. Write to .env file
	var env strings.Builder
	for key, value := range envMap {
		env.WriteString(fmt.Sprintf("%s=\"%s\"\n", key, value))
	}

	envPath := filepath.Join(projectDir, ".env")
	return os.WriteFile(envPath, []byte(env.String()), 0644)
}

func camelToSnake(name string) string {
	matchFirstCap := regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap := regexp.MustCompile("([a-z0-9])([A-Z])")

	snake := matchFirstCap.ReplaceAllString(name, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToUpper(snake)
}

func formatEnvValue(value interface{}) string {
	switch v := value.(type) {
	case bool:
		if v {
			return "true"
		}
		return "false"
	case []interface{}:
		var parts []string
		for _, part := range v {
			parts = append(parts, fmt.Sprint(part))
		}
		return strings.Join(parts, " ")
	case []string:
		return strings.Join(v, " ")
	default:
		return fmt.Sprint(v)
	}
}
