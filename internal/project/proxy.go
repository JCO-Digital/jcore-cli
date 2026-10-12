package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/JCO-Digital/jcore/internal/proxy"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// ProxyOverrideRelPath is the compose override jcore generates to put a
// project behind the shared proxy. It's referenced from .env through
// COMPOSE_FILE, so every `docker compose` invocation in the project
// directory (jcore's own and manual ones) picks it up.
const ProxyOverrideRelPath = ".jcore/compose.proxy.yml"

// Labels jcore puts on a project's web container, used to find running
// projects and the domains they serve.
const (
	LabelProject = "fi.jco.jcore.project"
	LabelDomains = "fi.jco.jcore.domains"
	LabelPath    = "fi.jco.jcore.path"
)

// loopbackImage runs the in-container loopback forwarder (see
// renderProxyOverride).
const loopbackImage = "alpine/socat:1.8.1.3"

var composeNameInvalidRe = regexp.MustCompile(`[^-_a-z0-9]+`)

// ComposeProjectName returns the compose project name docker compose
// derives for projectDir: its base name, lowercased, with characters
// compose doesn't allow removed (mirroring compose-go's
// NormalizeProjectName). jcore deliberately never overrides it - volumes
// are named after it, so changing it would orphan every existing database.
func ComposeProjectName(projectDir string) string {
	name := strings.ToLower(filepath.Base(projectDir))
	name = composeNameInvalidRe.ReplaceAllString(name, "")
	return strings.TrimLeft(name, "_-")
}

// LocalDomain returns the configured localDomain, falling back to
// "<slugified projectName>.localhost" for a project whose jcore.toml
// predates `init` seeding it.
func LocalDomain() string {
	if localDomain := viper.GetString("localDomain"); localDomain != "" {
		return localDomain
	}
	if projectName := viper.GetString("projectName"); projectName != "" {
		return Slugify(projectName) + ".localhost"
	}
	return "localhost"
}

// ProxyDomains returns every domain the proxy should route to this
// project: localDomain followed by the `domains` setting, deduplicated.
// Each one also covers its subdomains (multisite).
func ProxyDomains() []string {
	var domains []string
	for _, d := range append([]string{LocalDomain()}, viper.GetStringSlice("domains")...) {
		d = strings.ToLower(strings.TrimSpace(d))
		if d != "" && !slices.Contains(domains, d) {
			domains = append(domains, d)
		}
	}
	return domains
}

// ToolURLs returns the URLs the proxy serves this project's Adminer and
// mail catcher on.
func ToolURLs() (adminer, mail string) {
	localDomain := LocalDomain()
	return "http://adminer." + localDomain, "http://mail." + localDomain
}

// composeServices is the minimal shape of docker-compose.yml needed to
// generate the override: which services exist, and which networks they
// already declare.
type composeServices struct {
	Services map[string]struct {
		Networks yaml.Node `yaml:"networks"`
	} `yaml:"services"`
}

// readComposeServices parses projectDir's docker-compose.yml, returning
// each service's declared network names (empty if it declares none, i.e.
// it's implicitly on "default"). ok is false if there's no compose file.
func readComposeServices(projectDir string) (services map[string][]string, ok bool, err error) {
	raw, err := os.ReadFile(filepath.Join(projectDir, "docker-compose.yml"))
	if os.IsNotExist(err) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}

	var compose composeServices
	if err := yaml.Unmarshal(raw, &compose); err != nil {
		return nil, false, fmt.Errorf("parsing docker-compose.yml: %w", err)
	}

	services = make(map[string][]string, len(compose.Services))
	for name, service := range compose.Services {
		var networks []string
		switch service.Networks.Kind {
		case yaml.SequenceNode:
			for _, n := range service.Networks.Content {
				networks = append(networks, n.Value)
			}
		case yaml.MappingNode:
			for i := 0; i < len(service.Networks.Content); i += 2 {
				networks = append(networks, service.Networks.Content[i].Value)
			}
		}
		services[name] = networks
	}
	return services, true, nil
}

// WriteProxyOverride generates .jcore/compose.proxy.yml for projectDir. It
// reports false (and writes nothing) for a directory without a
// docker-compose.yml, in which case COMPOSE_FILE must not point at it.
func WriteProxyOverride(projectDir string) (bool, error) {
	services, ok, err := readComposeServices(projectDir)
	if err != nil || !ok {
		return false, err
	}

	content := renderProxyOverride(ComposeProjectName(projectDir), projectDir, LocalDomain(), ProxyDomains(), services)

	path := filepath.Join(projectDir, ProxyOverrideRelPath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, content, 0644)
}

// sniRule builds the Traefik TCP rule matching every domain and its
// subdomains.
func sniRule(domains []string) string {
	var parts []string
	for _, d := range domains {
		parts = append(parts, fmt.Sprintf("HostSNI(`%s`) || HostSNIRegexp(`^.+\\.%s$`)", d, regexp.QuoteMeta(d)))
	}
	return strings.Join(parts, " || ")
}

// yamlString quotes s as a YAML double-quoted scalar, escaping "$" so
// compose doesn't try to interpolate it.
func yamlString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(strings.ReplaceAll(s, "$", "$$"))
	return strings.TrimSpace(buf.String())
}

// renderProxyOverride renders the compose override that puts a project
// behind the shared proxy:
//
//   - web, adminer and mailhog drop their host ports and join the shared
//     network, with Traefik labels routing <domain> / *.<domain> (TLS
//     passthrough to web) and adminer./mail.<localDomain> (plain HTTP).
//     Only these HTTP-facing services ever join the shared network: compose
//     registers service names as DNS aliases on every network a service
//     joins, so a shared `wordpress` or `db` could resolve to another
//     project's container.
//   - a "loopback" sidecar sharing the wordpress container's network
//     namespace forwards its 127.0.0.1:80/443 to web. curl (and so the WP
//     HTTP API) resolves every *.localhost name to loopback without asking
//     DNS, so this is what makes WordPress's requests to its own site and
//     multisite subsites (wp-cron, REST, site health) reach nginx.
//
// Services the project's compose file doesn't define are skipped, since an
// override can't add a partial service.
func renderProxyOverride(composeName, projectDir, localDomain string, domains []string, services map[string][]string) []byte {
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }

	w("# Generated by jcore on every start - do not edit, changes are overwritten.")
	w("# Puts this project behind the shared JCore proxy (see `jcore proxy`).")
	w("services:")

	routed := func(service string, labels [][2]string) {
		networks, ok := services[service]
		if !ok {
			return
		}
		w("  %s:", service)
		w("    ports: !reset []")
		w("    networks:")
		if len(networks) == 0 {
			w("      default: {}")
		}
		w("      %s: {}", proxy.NetworkName)
		w("    labels:")
		w("      traefik.enable: \"true\"")
		for _, l := range labels {
			w("      %s: %s", l[0], yamlString(l[1]))
		}
	}

	router := composeName
	routed("web", [][2]string{
		{"traefik.tcp.routers." + router + ".entrypoints", "websecure"},
		{"traefik.tcp.routers." + router + ".rule", sniRule(domains)},
		{"traefik.tcp.routers." + router + ".tls.passthrough", "true"},
		{"traefik.tcp.routers." + router + ".service", router},
		{"traefik.tcp.services." + router + ".loadbalancer.server.port", "443"},
		{LabelProject, composeName},
		{LabelDomains, strings.Join(domains, ",")},
		{LabelPath, projectDir},
	})
	routed("adminer", [][2]string{
		{"traefik.http.routers." + router + "-adminer.entrypoints", "web"},
		{"traefik.http.routers." + router + "-adminer.rule", "Host(`adminer." + localDomain + "`)"},
		{"traefik.http.routers." + router + "-adminer.service", router + "-adminer"},
		{"traefik.http.services." + router + "-adminer.loadbalancer.server.port", "8080"},
	})
	routed("mailhog", [][2]string{
		{"traefik.http.routers." + router + "-mail.entrypoints", "web"},
		{"traefik.http.routers." + router + "-mail.rule", "Host(`mail." + localDomain + "`)"},
		{"traefik.http.routers." + router + "-mail.service", router + "-mail"},
		{"traefik.http.services." + router + "-mail.loadbalancer.server.port", "8025"},
	})

	_, hasWordpress := services["wordpress"]
	_, hasWeb := services["web"]
	if hasWordpress && hasWeb {
		// Prefer a dual-stack listener (curl tries ::1 first), falling back
		// to IPv4 only where the container has IPv6 disabled.
		script := "for p in 80 443; do " +
			"(socat TCP6-LISTEN:$p,fork,reuseaddr,ipv6only=0 TCP:web:$p 2>/dev/null " +
			"|| socat TCP4-LISTEN:$p,fork,reuseaddr TCP:web:$p) & " +
			"done; wait"
		w("  loopback:")
		w("    image: %s", loopbackImage)
		w("    network_mode: \"service:wordpress\"")
		w("    entrypoint: [\"sh\", \"-c\"]")
		w("    command: [%s]", yamlString(script))
		w("    restart: unless-stopped")
	}

	w("networks:")
	w("  %s:", proxy.NetworkName)
	w("    external: true")

	return []byte(b.String())
}

// RunningProject is a project currently served through the proxy, as
// found from the labels on its web container.
type RunningProject struct {
	Name    string
	Path    string
	Domains []string
}

// ProxiedProjects returns every project whose web container is running
// with jcore's proxy labels, sorted by name.
func ProxiedProjects() ([]RunningProject, error) {
	out, err := exec.Command("docker", "ps",
		"--filter", "label="+LabelProject,
		"--filter", "status=running",
		"--format", `{{.Label "`+LabelProject+`"}}|{{.Label "`+LabelPath+`"}}|{{.Label "`+LabelDomains+`"}}`).Output()
	if err != nil {
		return nil, err
	}

	var projects []RunningProject
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 || parts[0] == "" {
			continue
		}
		var domains []string
		if parts[2] != "" {
			domains = strings.Split(parts[2], ",")
		}
		projects = append(projects, RunningProject{Name: parts[0], Path: parts[1], Domains: domains})
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	return projects, nil
}

// CheckStartConflicts reports why projectDir can't be started alongside
// the projects already known to Docker, or nil if it can:
//
//   - another checkout with the same directory name (and so the same
//     compose project name) exists elsewhere - the two would share
//     containers and the database volume;
//   - another running project already serves one of this project's domains.
func CheckStartConflicts(projectDir string) error {
	name := ComposeProjectName(projectDir)

	all, err := listComposeProjects()
	if err != nil {
		return err
	}
	for _, p := range all {
		if p.Name == name && filepath.Clean(p.Path) != filepath.Clean(projectDir) {
			return fmt.Errorf("another checkout named %q already exists at %s.\n"+
				"Both would share the same containers and database volume, so they can't be used side by side.\n"+
				"Remove the other one (`jcore clean` there) or rename one of the directories", name, p.Path)
		}
	}

	running, err := ProxiedProjects()
	if err != nil {
		return err
	}
	domains := ProxyDomains()
	for _, p := range running {
		if filepath.Clean(p.Path) == filepath.Clean(projectDir) {
			continue
		}
		for _, d := range p.Domains {
			if slices.Contains(domains, d) {
				return fmt.Errorf("domain %s is already served by running project %q (%s).\n"+
					"Stop it first (`jcore stop` in %s), or give this project a different localDomain", d, p.Name, p.Path, p.Path)
			}
		}
	}
	return nil
}
