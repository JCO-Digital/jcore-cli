package project

import (
	"fmt"
	"strings"
	"time"

	"github.com/JCO-Digital/jcore/internal/docker"
)

// Overridable by tests.
var (
	activateThemePollInterval = 5 * time.Second
	activateThemeTimeout      = 10 * time.Minute
	// Upper bound for a single `docker compose exec` while polling. Without
	// it a hung exec (e.g. wp-cli waiting on a database that's still
	// initialising) blocks the whole loop past the overall deadline.
	activateThemeProbeTimeout = 15 * time.Second
	activateThemeWPTimeout    = 90 * time.Second
)

// ActivateTheme waits for the wordpress container to finish its entrypoint
// and then activates themeSlug if it isn't already the active theme. It's
// meant to be run concurrently with `docker compose up` right as containers
// are starting, so it tolerates WordPress/the DB not being ready yet -
// retrying silently rather than surfacing the still-starting container's
// errors. A themeSlug that's already active (e.g. one a user deliberately
// switched away from via wp-admin) is left alone.
//
// Readiness is checked with a cheap shell probe rather than wp-cli itself:
// booting PHP + WordPress every few seconds while the entrypoint is still
// downloading core, updating wp-cli and installing the site starves
// low-powered machines and races the entrypoint's own wp-cli invocations.
func ActivateTheme(projectDir, themeSlug string) {
	if themeSlug == "" {
		return
	}

	deadline := time.Now().Add(activateThemeTimeout)
	warn := func() {
		fmt.Printf(
			"Warning: could not activate theme %q (WordPress may not have come up yet) - activate manually with `jcore run \"wp theme activate %s\"`.\n",
			themeSlug, themeSlug,
		)
	}

	for !wordpressReady(projectDir) {
		if time.Now().After(deadline) {
			warn()
			return
		}
		time.Sleep(activateThemePollInterval)
	}

	for {
		switch activateThemeOnce(projectDir, themeSlug) {
		case themeDone:
			return
		case themeMissing:
			fmt.Printf(
				"Warning: theme %q is not installed in wp-content/themes - skipping activation. Set `theme` in jcore.toml to an installed theme, or clear it.\n",
				themeSlug,
			)
			return
		}
		if time.Now().After(deadline) {
			warn()
			return
		}
		time.Sleep(activateThemePollInterval)
	}
}

// wordpressReady reports whether the wordpress container's entrypoint has
// finished setting up the site. php-fpm is the entrypoint's final step, so
// its presence in the container's process table means WordPress is
// downloaded, configured and installed. Only /proc and sh are relied on,
// so this works regardless of which utilities the image ships.
func wordpressReady(projectDir string) bool {
	const script = `for p in /proc/[0-9]*; do read -r c < "$p/comm" 2>/dev/null || continue; case "$c" in php-fpm*) exit 0;; esac; done; exit 1`
	_, err := docker.ComposeExecCapturedTimeout(projectDir, "wordpress", []string{"sh", "-c", script}, activateThemeProbeTimeout)
	return err == nil
}

type themeResult int

const (
	// themeRetry: WordPress isn't reachable yet or activation failed - worth
	// another attempt.
	themeRetry themeResult = iota
	// themeDone: themeSlug is confirmed active (whether it already was, or
	// this call just activated it).
	themeDone
	// themeMissing: WordPress answered, but no theme by that name is
	// installed. Retrying can't help.
	themeMissing
)

// activateThemeOnce makes one attempt at getting themeSlug active.
func activateThemeOnce(projectDir, themeSlug string) themeResult {
	out, err := docker.ComposeExecCapturedTimeout(projectDir, "wordpress", []string{"wp", "theme", "list", "--fields=name,status", "--format=csv"}, activateThemeWPTimeout)
	if err != nil {
		return themeRetry
	}

	// The output can be interleaved with PHP startup notices (deprecation
	// warnings, Xdebug connection failures), so match whole CSV lines rather
	// than the output as a whole.
	installed := false
	for _, line := range strings.Split(out, "\n") {
		name, status, ok := strings.Cut(strings.TrimSpace(line), ",")
		if !ok || name != themeSlug {
			continue
		}
		installed = true
		if status == "active" {
			return themeDone
		}
	}
	if !installed {
		return themeMissing
	}

	if _, err := docker.ComposeExecCapturedTimeout(projectDir, "wordpress", []string{"wp", "theme", "activate", themeSlug}, activateThemeWPTimeout); err != nil {
		return themeRetry
	}
	fmt.Printf("Activated theme %q.\n", themeSlug)
	return themeDone
}
