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
	"runtime"
	"strings"
	"time"

	hcversion "github.com/hashicorp/go-version"
)

// Repo is the GitHub repository releases are published to.
const Repo = "JCO-Digital/jcore-cli"

// LatestReleaseURL is the GitHub API endpoint for the latest release.
const LatestReleaseURL = "https://api.github.com/repos/" + Repo + "/releases/latest"

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
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
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

// IsNewer reports whether latestVersion is a greater semantic version than
// currentVersion. If currentVersion isn't valid semver (e.g. a "dev"
// build), it's treated as "not newer" rather than an error, since there's
// no reliable way to compare it.
func IsNewer(latestVersion, currentVersion string) (bool, error) {
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
func CheckForUpdate(currentVersion string) (latest, downloadURL, sigURL string, available bool, err error) {
	release, err := GetLatestRelease(LatestReleaseURL)
	if err != nil {
		return "", "", "", false, fmt.Errorf("failed to check for updates: %w", err)
	}

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
