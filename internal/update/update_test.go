package update

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		name    string
		latest  string
		current string
		want    bool
		wantErr bool
	}{
		{"newer available", "v3.17.0", "3.16.3", true, false},
		{"same version", "v3.16.3", "3.16.3", false, false},
		{"current is newer", "v3.16.0", "3.16.3", false, false},
		{"dev build never newer", "v3.17.0", "dev", false, false},
		{"invalid latest errors", "not-a-version", "3.16.3", false, true},
		{"git describe suffix, same tag not newer", "v3.16.3", "v3.16.3-11-gda4c707", false, false},
		{"git describe suffix, tag moved on", "v3.17.0", "v3.16.3-11-gda4c707", true, false},
		{"git describe suffix, dirty build", "v3.16.3", "v3.16.3-11-gda4c707-dirty", false, false},
		{"git describe suffix, current ahead of older tag", "v3.16.0", "v3.16.3-11-gda4c707", false, false},
		{"malformed doubled v prefix on latest", "vv3.17.0", "3.16.3", true, false},
		{"malformed doubled v prefix, not newer", "vv3.16.3", "3.16.3", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := IsNewer(tc.latest, tc.current)
			if tc.wantErr != (err != nil) {
				t.Fatalf("IsNewer(%q, %q) error = %v, wantErr %v", tc.latest, tc.current, err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("IsNewer(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
			}
		})
	}
}

func TestValidateDownloadURL(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"github release asset", "https://github.com/JCO-Digital/jcore-cli/releases/download/v1.0.0/jcore_linux_amd64", false},
		{"github objects redirect", "https://objects.githubusercontent.com/foo", false},
		{"non-https rejected", "http://github.com/JCO-Digital/jcore-cli/releases/download/v1.0.0/jcore_linux_amd64", true},
		{"other host rejected", "https://evil.example.com/jcore_linux_amd64", true},
		{"lookalike host rejected", "https://github.com.evil.example.com/jcore_linux_amd64", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDownloadURL(tc.url)
			if tc.wantErr != (err != nil) {
				t.Fatalf("validateDownloadURL(%q) error = %v, wantErr %v", tc.url, err, tc.wantErr)
			}
		})
	}
}

func TestAssetName(t *testing.T) {
	name := AssetName()
	if name == "" {
		t.Fatal("AssetName() returned empty string")
	}
}

func TestIsPrerelease(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"v4.0.0-beta.1", true},
		{"v4.0.0-beta.2", true},
		{"v4.0.0-alpha.1", true},
		{"v4.0.0-rc.1", true},
		{"v4.0.0-beta.1-12-g1234567", true},
		{"v4.0.0-beta.1-12-g1234567-dirty", true},
		{"v3.16.3", false},
		{"v3.16.3-11-gda4c707", false},
		{"3.16.3", false},
		{"dev", false},
		{"dev-beta", true},
	}

	for _, tc := range cases {
		t.Run(tc.version, func(t *testing.T) {
			got := IsPrerelease(tc.version)
			if got != tc.want {
				t.Fatalf("IsPrerelease(%q) = %v, want %v", tc.version, got, tc.want)
			}
		})
	}
}

func TestSelectBestRelease(t *testing.T) {
	assetName := AssetName()
	sigName := assetName + ".minisig"

	mockAssets := []Asset{
		{Name: assetName, BrowserDownloadURL: "https://github.com/test/download"},
		{Name: sigName, BrowserDownloadURL: "https://github.com/test/sig"},
	}

	releases := []Release{
		{
			TagName:    "v3.22.0",
			Prerelease: false,
			Assets:     mockAssets,
		},
		{
			TagName:    "v4.0.0-beta.1",
			Prerelease: true,
			Assets:     mockAssets,
		},
		{
			TagName:    "v4.0.0-beta.2",
			Prerelease: true,
			Assets:     mockAssets,
		},
		{
			TagName:    "v4.0.0-beta.3",
			Draft:      true,
			Prerelease: true,
			Assets:     mockAssets,
		},
		{
			TagName:    "v4.0.0-beta.4",
			Prerelease: true,
			Assets:     []Asset{}, // missing assets
		},
	}

	// Without beta: should select v3.22.0
	r := SelectBestRelease(releases, false)
	if r == nil || r.TagName != "v3.22.0" {
		t.Fatalf("SelectBestRelease(false) = %v, want v3.22.0", r)
	}

	// With beta: should select highest valid beta (v4.0.0-beta.2, skipping draft beta.3 and asset-missing beta.4)
	r = SelectBestRelease(releases, true)
	if r == nil || r.TagName != "v4.0.0-beta.2" {
		t.Fatalf("SelectBestRelease(true) = %v, want v4.0.0-beta.2", r)
	}

	// When a stable release is newer than the beta
	releasesWithStable := append(releases, Release{
		TagName:    "v4.0.0",
		Prerelease: false,
		Assets:     mockAssets,
	})
	r = SelectBestRelease(releasesWithStable, true)
	if r == nil || r.TagName != "v4.0.0" {
		t.Fatalf("SelectBestRelease(true) with newer stable = %v, want v4.0.0", r)
	}
}
