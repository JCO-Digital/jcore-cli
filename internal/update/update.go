// Package update implements the "jcore update self" flow: checking GitHub
// releases for a newer version of the CLI, verifying the downloaded binary
// against a detached Ed25519 signature, and replacing the running
// executable in place.
package update

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	hcversion "github.com/hashicorp/go-version"
)

// Repo is the GitHub repository releases are published to.
const Repo = "JCO-Digital/jcore-cli"

// LatestReleaseURL is the GitHub API endpoint for the latest release.
const LatestReleaseURL = "https://api.github.com/repos/" + Repo + "/releases/latest"

// ReleasesListURL is the GitHub API endpoint for recent releases.
const ReleasesListURL = "https://api.github.com/repos/" + Repo + "/releases"

// maxDownloadSize caps how much data will be read from a release asset, to
// bound memory/disk use if an upstream host is compromised or misbehaves.
const maxDownloadSize = 200 * 1024 * 1024 // 200 MiB

// maxSignatureSize caps how much data will be read for a detached signature.
const maxSignatureSize = 4 * 1024 // 4 KiB

// allowedDownloadHosts restricts release asset downloads to GitHub's own
// hosts, so a compromised release API response can't redirect elsewhere.
var allowedDownloadHosts = []string{"github.com", "objects.githubusercontent.com"}

// Release is the subset of the GitHub release API response used here.
type Release struct {
	TagName    string  `json:"tag_name"`
	HTMLURL    string  `json:"html_url"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

// Asset is a single GitHub release asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// AssetName returns the release asset name for the binary that matches the
// platform this process is running on, e.g. "jcore_linux_amd64" or
// "jcore_windows_amd64.exe".
func AssetName() string {
	name := fmt.Sprintf("jcore_%s_%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// GetLatestRelease fetches the latest release from the GitHub API.
func GetLatestRelease(apiURL string) (*Release, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch latest release: received status code %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode release data: %w", err)
	}

	return &release, nil
}

// GetReleases fetches recent releases from the GitHub API.
func GetReleases(apiURL string) ([]Release, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch releases: received status code %d", resp.StatusCode)
	}

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode release data: %w", err)
	}

	return releases, nil
}

// SelectBestRelease returns the release with the highest semantic version
// among those that contain binary and signature assets for the current platform.
// If includeBeta is false, pre-releases are ignored.
func SelectBestRelease(releases []Release, includeBeta bool) *Release {
	assetName := AssetName()
	sigName := assetName + ".minisig"

	var bestRelease *Release
	var bestVersion *hcversion.Version

	for i := range releases {
		r := &releases[i]
		if r.Draft {
			continue
		}
		if !includeBeta && r.Prerelease {
			continue
		}

		hasAsset := false
		hasSig := false
		for _, asset := range r.Assets {
			if asset.Name == assetName {
				hasAsset = true
			} else if asset.Name == sigName {
				hasSig = true
			}
		}
		if !hasAsset || !hasSig {
			continue
		}

		tag := redundantVPrefix.ReplaceAllString(r.TagName, "v")
		v, err := hcversion.NewVersion(tag)
		if err != nil {
			continue
		}

		if bestVersion == nil || v.GreaterThan(bestVersion) {
			bestVersion = v
			bestRelease = r
		}
	}

	return bestRelease
}

// devSuffix matches the "-<n>-g<hash>[-dirty]" suffix `git describe --tags
// --always --dirty` appends to the nearest tag when built from a commit
// that isn't itself tagged (see Makefile's LDFLAGS). Semver treats a
// trailing "-..." as a pre-release, which sorts *below* the bare tag — the
// opposite of what it means here, since those commits come *after* the
// tag. Stripped down to the tag itself, "am I newer" becomes "did the tag
// move past me", which is what's actually being asked.
var devSuffix = regexp.MustCompile(`-\d+-g[0-9a-fA-F]+(-dirty)?$`)

// redundantVPrefix strips every leading "v" but one, tolerating a release
// tag like "vv3.17.0" - seen in the wild from a CI step that prepended its
// own "v" onto a version foonver had already prefixed with one. hcversion
// already accepts a single leading "v" itself; this only handles doubling
// up on top of that.
var redundantVPrefix = regexp.MustCompile(`^v+`)

// IsPrerelease reports whether the given version string represents a
// pre-release or beta version (e.g. "v4.0.0-beta.1", "4.0.0-beta", etc.).
func IsPrerelease(version string) bool {
	cleaned := devSuffix.ReplaceAllString(version, "")
	cleaned = redundantVPrefix.ReplaceAllString(cleaned, "v")

	v, err := hcversion.NewVersion(cleaned)
	if err == nil {
		return v.Prerelease() != ""
	}

	lower := strings.ToLower(version)
	return strings.Contains(lower, "beta") || strings.Contains(lower, "alpha") || strings.Contains(lower, "rc")
}

// IsNewer reports whether latestVersion is a greater semantic version than
// currentVersion. If currentVersion isn't valid semver (e.g. a "dev"
// build), it's treated as "not newer" rather than an error, since there's
// no reliable way to compare it.
func IsNewer(latestVersion, currentVersion string) (bool, error) {
	currentVersion = devSuffix.ReplaceAllString(currentVersion, "")
	latestVersion = redundantVPrefix.ReplaceAllString(latestVersion, "v")

	vCurrent, err := hcversion.NewVersion(currentVersion)
	if err != nil {
		return false, nil
	}

	vLatest, err := hcversion.NewVersion(latestVersion)
	if err != nil {
		return false, fmt.Errorf("failed to parse version %s: %w", latestVersion, err)
	}

	return vLatest.GreaterThan(vCurrent), nil
}

// CheckForUpdate checks whether a newer release of the CLI is available. It
// returns the latest version tag, the download URL and signature URL for
// the current platform's binary, and whether an update is available.
// If includeBeta is specified, it controls whether pre-release/beta versions
// are considered; if omitted, it defaults to true if currentVersion is already
// a pre-release version.
func CheckForUpdate(currentVersion string, opts ...bool) (latest, downloadURL, sigURL string, available bool, err error) {
	includeBeta := IsPrerelease(currentVersion)
	if len(opts) > 0 {
		includeBeta = opts[0]
	}

	var release *Release
	if includeBeta {
		releases, err := GetReleases(ReleasesListURL + "?per_page=20")
		if err != nil {
			return "", "", "", false, fmt.Errorf("failed to check for updates: %w", err)
		}
		release = SelectBestRelease(releases, true)
		if release == nil {
			return "", "", "", false, fmt.Errorf("no release found matching this platform (%s)", AssetName())
		}
	} else {
		var err error
		release, err = GetLatestRelease(LatestReleaseURL)
		if err != nil {
			return "", "", "", false, fmt.Errorf("failed to check for updates: %w", err)
		}
	}
	release.TagName = redundantVPrefix.ReplaceAllString(release.TagName, "v")

	assetName := AssetName()
	sigName := assetName + ".minisig"

	for _, asset := range release.Assets {
		switch asset.Name {
		case assetName:
			downloadURL = asset.BrowserDownloadURL
		case sigName:
			sigURL = asset.BrowserDownloadURL
		}
	}

	newer, err := IsNewer(release.TagName, currentVersion)
	if err != nil {
		return "", "", "", false, err
	}

	return release.TagName, downloadURL, sigURL, newer && downloadURL != "", nil
}

func validateDownloadURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("URL must use https, got %q", u.Scheme)
	}
	for _, host := range allowedDownloadHosts {
		if u.Host == host || strings.HasSuffix(u.Host, "."+host) {
			return nil
		}
	}
	return fmt.Errorf("URL host %q is not an allowed download host", u.Host)
}

// DownloadAndReplace downloads the binary at downloadURL, verifies it
// against the detached signature at sigURL, and atomically replaces
// targetPath with it on success. Any failure leaves targetPath untouched.
func DownloadAndReplace(downloadURL, sigURL, targetPath string) error {
	if err := validateDownloadURL(downloadURL); err != nil {
		return fmt.Errorf("refusing to download update: %w", err)
	}
	if sigURL == "" {
		return fmt.Errorf("refusing to install update: no signature asset found")
	}
	if err := validateDownloadURL(sigURL); err != nil {
		return fmt.Errorf("refusing to download update signature: %w", err)
	}

	dir := filepath.Dir(targetPath)

	// Default mode to 0755 if the target doesn't exist yet.
	mode := os.FileMode(0755)
	if info, err := os.Stat(targetPath); err == nil {
		mode = info.Mode().Perm()
	}

	// Download to a temp file in the same directory as the target, so the
	// final rename is an atomic same-filesystem operation.
	tmpFile, err := os.CreateTemp(dir, "jcore-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(downloadURL)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("failed to download update: received status code %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxDownloadSize+1)
	written, err := io.Copy(tmpFile, limited)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write update to temporary file: %w", err)
	}
	if written > maxDownloadSize {
		tmpFile.Close()
		return fmt.Errorf("update download exceeded maximum allowed size of %d bytes", maxDownloadSize)
	}

	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}

	// Verify signature. A missing or invalid signature is a hard failure —
	// every release binary is signed in CI, so its absence indicates a
	// compromised or malformed release and must not be installed.
	if _, err := tmpFile.Seek(0, 0); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to seek temporary file: %w", err)
	}
	content, err := io.ReadAll(io.LimitReader(tmpFile, maxDownloadSize+1))
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to read temporary file for verification: %w", err)
	}
	if err := verifySignature(content, sigURL); err != nil {
		tmpFile.Close()
		return fmt.Errorf("signature verification failed: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	if err := os.Chmod(tmpPath, mode); err != nil {
		return fmt.Errorf("failed to set permissions on new binary: %w", err)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("failed to replace binary (do you have write permission?): %w", err)
	}

	return nil
}

func verifySignature(content []byte, sigURL string) error {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(sigURL)
	if err != nil {
		return fmt.Errorf("failed to download signature: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download signature: received status code %d", resp.StatusCode)
	}

	sigBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxSignatureSize+1))
	if err != nil {
		return fmt.Errorf("failed to read signature: %w", err)
	}
	if len(sigBytes) > maxSignatureSize {
		return fmt.Errorf("signature response exceeded maximum allowed size of %d bytes", maxSignatureSize)
	}

	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigBytes)))
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	pubKeyBytes, err := base64.StdEncoding.DecodeString(PublicKey)
	if err != nil {
		return fmt.Errorf("failed to decode public key: %w", err)
	}

	if !ed25519.Verify(ed25519.PublicKey(pubKeyBytes), content, signature) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}
