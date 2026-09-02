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
