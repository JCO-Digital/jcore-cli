package project

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// fallbackPHPPatches is the newest patch release of each PHP minor known when
// this CLI was built. It is only used when php.net can't be reached and the
// project's own composer.json has no full version for the minor either.
var fallbackPHPPatches = map[string]string{
	"8.2": "8.2.36",
	"8.3": "8.3.37",
	"8.4": "8.4.28",
	"8.5": "8.5.13",
}

// defaultPHPMinor is used when neither the phpVersion setting, the project's
// composer.json nor the template catalog names a PHP version.
const defaultPHPMinor = "8.4"

var (
	phpMinorRe = regexp.MustCompile(`^\d+\.\d+$`)
	phpFullRe  = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

	// composerPlatformPHPRe matches the "php" key inside composer.json's
	// config.platform object, capturing the version string so it can be
	// replaced without reformatting the rest of the file.
	composerPlatformPHPRe = regexp.MustCompile(`("platform"\s*:\s*\{[^{}]*?"php"\s*:\s*")([^"]*)(")`)
)

// latestPHPPatch returns the newest patch release of a PHP minor version
// ("8.4" -> "8.4.28"). It is a variable so tests can avoid the network.
var latestPHPPatch = fetchLatestPHPPatch

var (
	phpPatchCacheMu sync.Mutex
	phpPatchCache   = map[string]string{}
)

// fetchLatestPHPPatch asks php.net for the newest release of minor. Results
// are cached for the life of the process, since one command can render
// several templates.
func fetchLatestPHPPatch(minor string) (string, error) {
	phpPatchCacheMu.Lock()
	defer phpPatchCacheMu.Unlock()
	if v, ok := phpPatchCache[minor]; ok {
		return v, nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://www.php.net/releases/index.php?json&version=" + url.QueryEscape(minor))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("php.net returned %s", resp.Status)
	}

	var release struct {
		Version string `json:"version"`
		Error   string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("unexpected php.net response: %w", err)
	}
	if release.Error != "" {
		return "", fmt.Errorf("php.net: %s", release.Error)
	}
	if !phpFullRe.MatchString(release.Version) || !strings.HasPrefix(release.Version, minor+".") {
		return "", fmt.Errorf("unexpected PHP version %q from php.net for %s", release.Version, minor)
	}

	phpPatchCache[minor] = release.Version
	return release.Version, nil
}

// phpMinor returns the "major.minor" part of a PHP version string, or "" if
// it isn't one.
func phpMinor(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return ""
	}
	minor := parts[0] + "." + parts[1]
	if !phpMinorRe.MatchString(minor) {
		return ""
	}
	return minor
}

// ComposerPlatformPHP returns config.platform.php from the project's
// composer.json, or "" if the file or key is missing.
func ComposerPlatformPHP(projectDir string) string {
	raw, err := os.ReadFile(filepath.Join(projectDir, "composer.json"))
	if err != nil {
		return ""
	}
	var composer struct {
		Config struct {
			Platform struct {
				PHP string `json:"php"`
			} `json:"platform"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &composer); err != nil {
		return ""
	}
	return composer.Config.Platform.PHP
}

// ResolvePHPVersion returns the full PHP version (e.g. "8.4.28") to write as
// composer.json's config.platform.php. roave/security-advisories refuses to
// install on vulnerable PHP versions, and composer reads a bare "8.4" as
// 8.4.0, so this must always be a current patch release.
//
// The minor version comes from, in order: the phpVersion setting, the
// project's existing composer.json (so updating never moves a project to
// another minor), and the template catalog. A phpVersion setting with a full
// version pins it as-is. Otherwise the newest patch is fetched from php.net,
// falling back to the project's current full version for that minor, then to
// the newest patch this CLI knows of.
func ResolvePHPVersion(projectDir, templateName string) string {
	setting := strings.TrimSpace(viper.GetString("phpVersion"))
	if phpFullRe.MatchString(setting) {
		return setting
	}

	existing := ComposerPlatformPHP(projectDir)

	minor := phpMinor(setting)
	if minor == "" {
		minor = phpMinor(existing)
	}
	if minor == "" {
		if catalog, err := LoadTemplateCatalog(); err == nil {
			minor = phpMinor(catalog[templateName].PHP)
		}
	}
	if minor == "" {
		minor = defaultPHPMinor
	}

	latest, err := latestPHPPatch(minor)
	if err == nil {
		return latest
	}

	fallback := ""
	if phpFullRe.MatchString(existing) && phpMinor(existing) == minor {
		fallback = existing
	} else if v, ok := fallbackPHPPatches[minor]; ok {
		fallback = v
	}
	if fallback == "" {
		fmt.Printf("Warning: could not look up the latest PHP %s release (%v); using %s.0, which composer may refuse as vulnerable. Set phpVersion to a full version to pin one.\n", minor, err, minor)
		return minor + ".0"
	}
	fmt.Printf("Warning: could not look up the latest PHP %s release (%v); using %s.\n", minor, err, fallback)
	return fallback
}

// SyncComposerPlatformPHP sets config.platform.php in the project's
// composer.json to version, editing only that value so the rest of the file
// (which projects routinely change) is kept as-is. If the file was otherwise
// unmodified, its stored checksum is refreshed so later updates still treat
// it as pristine. It returns whether the file changed.
func SyncComposerPlatformPHP(projectDir, version string, checksums map[string]string) (bool, error) {
	path := filepath.Join(projectDir, "composer.json")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	match := composerPlatformPHPRe.FindSubmatch(raw)
	if match == nil || string(match[2]) == version {
		return false, nil
	}

	wasPristine := false
	if stored, ok := checksums["composer.json"]; ok {
		if current, err := CalculateChecksum(path); err == nil && current == stored {
			wasPristine = true
		}
	}

	updated := composerPlatformPHPRe.ReplaceAll(raw, []byte("${1}"+version+"${3}"))
	if err := os.WriteFile(path, updated, 0644); err != nil {
		return false, err
	}
	if wasPristine {
		checksums["composer.json"], _ = CalculateChecksum(path)
	}
	return true, nil
}
