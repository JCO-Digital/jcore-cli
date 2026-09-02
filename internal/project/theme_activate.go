package project

import (
	"fmt"
	"strings"
	"time"

	"github.com/JCO-Digital/jcore/internal/docker"
)

// Overridable by tests.
var (
	activateThemePollInterval = 3 * time.Second
	activateThemeTimeout      = 120 * time.Second
)

// ActivateTheme polls the wordpress container until it's reachable and
// activates themeSlug if it isn't already the active theme. It's meant to
// be run concurrently with `docker compose up` right as containers are
// starting, so it tolerates WordPress/the DB not being ready yet -
// retrying silently rather than surfacing the still-starting container's
// errors. A themeSlug that's already active (e.g. one a user deliberately
// switched away from via wp-admin) is left alone.
func ActivateTheme(projectDir, themeSlug string) {
	if themeSlug == "" {
		return
	}

	deadline := time.Now().Add(activateThemeTimeout)
	for {
		if activateThemeOnce(projectDir, themeSlug) {
			return
		}

		if time.Now().After(deadline) {
			fmt.Printf(
				"Warning: could not activate theme %q (WordPress may not have come up yet) - activate manually with `jcore run \"wp theme activate %s\"`.\n",
				themeSlug, themeSlug,
			)
			return
		}
		time.Sleep(activateThemePollInterval)
	}
}

// activateThemeOnce makes one attempt, returning true once themeSlug is
// confirmed active (whether it already was, or this call just activated
// it). A false return means WordPress isn't reachable yet, or activation
// failed - both worth another attempt.
func activateThemeOnce(projectDir, themeSlug string) bool {
	active, err := docker.ComposeExecCaptured(projectDir, "wordpress", []string{"wp", "theme", "list", "--status=active", "--field=name"})
	if err != nil {
		return false
	}
	if strings.TrimSpace(active) == themeSlug {
		return true
	}

	if _, err := docker.ComposeExecCaptured(projectDir, "wordpress", []string{"wp", "theme", "activate", themeSlug}); err != nil {
		return false
	}
	fmt.Printf("Activated theme %q.\n", themeSlug)
	return true
}
