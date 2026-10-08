package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func TestComposeProjectName(t *testing.T) {
	tests := map[string]string{
		"/home/me/projects/acme":           "acme",
		"/home/me/projects/Soilfood-Multi": "soilfood-multi",
		"/home/me/projects/my.site":        "mysite",
		"/home/me/projects/_hidden":        "hidden",
		"/home/me/projects/a_b-c":          "a_b-c",
	}
	for dir, want := range tests {
		if got := ComposeProjectName(dir); got != want {
			t.Errorf("ComposeProjectName(%q) = %q, want %q", dir, got, want)
		}
	}
}

func TestProxyDomains_LocalDomainFirstAndDeduped(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("localDomain", "acme.localhost")
	viper.Set("domains", []string{"Acme.localhost", "acme.fi", " www.acme.fi "})

	got := strings.Join(ProxyDomains(), ",")
	if want := "acme.localhost,acme.fi,www.acme.fi"; got != want {
		t.Errorf("ProxyDomains() = %q, want %q", got, want)
	}
}

func TestSNIRule(t *testing.T) {
	got := sniRule([]string{"acme.localhost", "acme.fi"})
	want := "HostSNI(`acme.localhost`) || HostSNIRegexp(`^.+\\.acme\\.localhost$`) || " +
		"HostSNI(`acme.fi`) || HostSNIRegexp(`^.+\\.acme\\.fi$`)"
	if got != want {
		t.Errorf("sniRule() =\n  %s\nwant\n  %s", got, want)
	}
}

// renderedOverride parses renderProxyOverride's output, with "!reset"
// treated as an ordinary tag.
func renderedOverride(t *testing.T, services map[string][]string) map[string]map[string]any {
	t.Helper()
	out := renderProxyOverride("acme", "/home/me/projects/acme", "acme.localhost", []string{"acme.localhost"}, services)

	var parsed struct {
		Services map[string]map[string]any `yaml:"services"`
	}
	if err := yaml.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("override isn't valid YAML: %v\n%s", err, out)
	}
	return parsed.Services
}

func TestRenderProxyOverride_RoutesHTTPFacingServicesOnly(t *testing.T) {
	services := renderedOverride(t, map[string][]string{
		"web":       {"default", "jcore"},
		"adminer":   {"default"},
		"mailhog":   nil,
		"wordpress": {"default", "jcore"},
		"db":        {"default"},
		"memcached": nil,
	})

	for _, name := range []string{"web", "adminer", "mailhog"} {
		networks, _ := services[name]["networks"].(map[string]any)
		if _, ok := networks["jcore-proxy"]; !ok {
			t.Errorf("%s doesn't join jcore-proxy: %v", name, services[name]["networks"])
		}
	}
	for _, name := range []string{"wordpress", "db", "memcached"} {
		if _, ok := services[name]; ok {
			t.Errorf("%s must not be touched by the override (it would join the shared network)", name)
		}
	}

	// A service declaring no networks is implicitly on "default", which
	// adding jcore-proxy alone would take it off.
	mailNets, _ := services["mailhog"]["networks"].(map[string]any)
	if _, ok := mailNets["default"]; !ok {
		t.Errorf("mailhog should keep the default network: %v", mailNets)
	}
	webNets, _ := services["web"]["networks"].(map[string]any)
	if _, ok := webNets["default"]; ok {
		t.Errorf("web declares its networks, the override shouldn't add default: %v", webNets)
	}

	labels, _ := services["web"]["labels"].(map[string]any)
	if got := labels["traefik.tcp.routers.acme.rule"]; got != "HostSNI(`acme.localhost`) || HostSNIRegexp(`^.+\\.acme\\.localhost$$`)" {
		t.Errorf("web rule = %v (\"$\" must be escaped for compose)", got)
	}
	if got := labels["traefik.tcp.routers.acme.tls.passthrough"]; got != "true" {
		t.Errorf("web passthrough = %v", got)
	}
	if got := labels[LabelPath]; got != "/home/me/projects/acme" {
		t.Errorf("%s = %v", LabelPath, got)
	}

	adminerLabels, _ := services["adminer"]["labels"].(map[string]any)
	if got := adminerLabels["traefik.http.routers.acme-adminer.rule"]; got != "Host(`adminer.acme.localhost`)" {
		t.Errorf("adminer rule = %v", got)
	}

	loopback := services["loopback"]
	if loopback == nil || loopback["network_mode"] != "service:wordpress" {
		t.Errorf("loopback sidecar missing or not sharing wordpress's network: %v", loopback)
	}
}

func TestRenderProxyOverride_SkipsMissingServices(t *testing.T) {
	services := renderedOverride(t, map[string][]string{"web": nil})

	if _, ok := services["web"]; !ok {
		t.Error("web should be routed")
	}
	for _, name := range []string{"adminer", "mailhog", "loopback"} {
		if _, ok := services[name]; ok {
			t.Errorf("%s isn't in the project, the override mustn't add it", name)
		}
	}
}

func TestGenerateEnvFile_PointsComposeAtProxyOverride(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("localDomain", "acme.localhost")

	dir := t.TempDir()
	compose := "services:\n  web:\n    image: nginx\n  wordpress:\n    image: wp\n"
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(compose), 0644); err != nil {
		t.Fatal(err)
	}

	if err := GenerateEnvFile(dir); err != nil {
		t.Fatalf("GenerateEnvFile error = %v", err)
	}

	if got := readEnvVar(t, dir, "COMPOSE_FILE"); got != "docker-compose.yml:"+ProxyOverrideRelPath {
		t.Errorf("COMPOSE_FILE = %q", got)
	}
	if got := readEnvVar(t, dir, "PHP_IDE_CONFIG"); got != "serverName=acme.localhost" {
		t.Errorf("PHP_IDE_CONFIG = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, ProxyOverrideRelPath)); err != nil {
		t.Errorf("override not written: %v", err)
	}
}

func TestGenerateEnvFile_NoComposeFileNoOverride(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	dir := t.TempDir()
	if err := GenerateEnvFile(dir); err != nil {
		t.Fatalf("GenerateEnvFile error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "COMPOSE_FILE=") {
		t.Errorf("COMPOSE_FILE set without a docker-compose.yml:\n%s", raw)
	}
}
