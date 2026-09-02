package project

import (
	"github.com/JCO-Digital/jcore/internal/docker"
)

// ImportDatabase runs the container's importdb script, which fetches the
// database from the configured remote host (unless a specific dump was
// already staged at .jcore/sql/update.sql) and imports it via wp-cli,
// applying the configured domain search-replace.
func ImportDatabase(projectDir string) error {
	return docker.ComposeExec(projectDir, "wordpress", []string{"/project/.config/scripts/importdb"})
}
