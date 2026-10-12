package project

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/JCO-Digital/jcore/internal/docker"
	"github.com/JCO-Digital/jcore/internal/logging"
	"github.com/spf13/viper"
)

// ImportDatabase imports the database either natively from outside the container
// (using host SSH and a direct pipe into MariaDB) or falls back to the legacy
// in-container importdb script if legacy is true or dbPullLegacy is configured.
func ImportDatabase(projectDir string, legacy bool) error {
	if legacy || viper.GetBool("dbPullLegacy") {
		return ImportDatabaseLegacy(projectDir)
	}
	return ImportDatabaseNative(projectDir)
}

// ImportDatabaseLegacy runs the container's importdb script inside the wordpress container.
func ImportDatabaseLegacy(projectDir string) error {
	KnockIfNeeded()
	return docker.ComposeExec(projectDir, "wordpress", []string{"/project/.config/scripts/importdb"})
}

// ImportDatabaseNative orchestrates the host-driven database migration:
// 1. Decompresses update.sql.gz if present.
// 2. If update.sql is absent, knocks (if configured) and fetches via host SSH.
// 3. Updates table prefix in wp-config.php.
// 4. Pipes the SQL dump directly into the MariaDB container.
// 5. Rotates update.sql to db.sql.
// 6. Runs wp search-replace in the wordpress container with --skip-plugins --skip-themes.
func ImportDatabaseNative(projectDir string) error {
	// Execute custom before-script inside container if present
	if err := RunCustomScript(projectDir, "db-before"); err != nil {
		return err
	}

	sqlDir := filepath.Join(projectDir, ".jcore", "sql")
	if err := os.MkdirAll(sqlDir, 0755); err != nil {
		return fmt.Errorf("failed to create sql directory: %w", err)
	}

	updateSql := filepath.Join(sqlDir, "update.sql")
	updateGz := filepath.Join(sqlDir, "update.sql.gz")

	// 1. Decompress if update.sql.gz exists
	if _, err := os.Stat(updateGz); err == nil {
		fmt.Println("Decompressing update.sql.gz...")
		if err := DecompressGzip(updateGz, updateSql); err != nil {
			return fmt.Errorf("failed to decompress %s: %w", updateGz, err)
		}
		_ = os.Remove(updateGz)
	}

	// 2. Fetch if update.sql does not exist
	if _, err := os.Stat(updateSql); os.IsNotExist(err) {
		KnockIfNeeded()

		remoteHost := viper.GetString("remoteHost")
		if remoteHost == "" {
			return errors.New("remoteHost is not configured in jcore.toml")
		}
		remotePath := viper.GetString("remotePath")
		if remotePath == "" {
			return errors.New("remotePath is not configured in jcore.toml")
		}

		dbPrefix := viper.GetString("dbPrefix")
		if dbPrefix == "" {
			dbPrefix = "wp_"
		}
		dbExclude := viper.GetStringSlice("dbExclude")
		excludeArg := BuildExcludeTablesArg(dbExclude, dbPrefix)

		fmt.Printf("Fetching database from %s...\n", remoteHost)
		if err := FetchRemoteDatabase(remoteHost, remotePath, excludeArg, updateSql); err != nil {
			return err
		}
	}

	// 3. Import if update.sql exists
	if _, err := os.Stat(updateSql); err == nil {
		fmt.Println("Found update")

		dbPrefix := viper.GetString("dbPrefix")
		if dbPrefix == "" {
			dbPrefix = "wp_"
		}
		fmt.Println("Update table prefix")
		if err := UpdateTablePrefix(projectDir, dbPrefix); err != nil {
			logging.Warn("Failed to update table prefix: %v", err)
		}

		fmt.Println("Import DB")
		if err := ImportSQLDump(projectDir, updateSql); err != nil {
			return fmt.Errorf("database import failed: %w", err)
		}

		RotateSQLFiles(sqlDir)

		if err := ApplyDomainReplacements(projectDir); err != nil {
			logging.Warn("Domain replacement warning: %v", err)
		}
	}

	// Execute custom after-script inside container if present
	if err := RunCustomScript(projectDir, "db-after"); err != nil {
		return err
	}

	return nil
}

// ParseSSHHostPort extracts host and optional port from a remoteHost string
// which can be in formats:
// - "user@hostname"
// - "hostname"
// - "user@hostname:2222"
// - "hostname:2222"
// - "[ipv6]:port"
// - "user@[ipv6]:port"
func ParseSSHHostPort(remoteHost string) (string, string) {
	remoteHost = strings.TrimSpace(remoteHost)
	if remoteHost == "" {
		return "", ""
	}

	var user string
	hostPart := remoteHost
	if idx := strings.Index(remoteHost, "@"); idx != -1 {
		user = remoteHost[:idx+1]
		hostPart = remoteHost[idx+1:]
	}

	if host, port, err := net.SplitHostPort(hostPart); err == nil {
		return user + host, port
	}

	return remoteHost, ""
}

// BuildExcludeTablesArg constructs the --exclude_tables=... flag for wp db export.
func BuildExcludeTablesArg(dbExclude []string, dbPrefix string) string {
	if len(dbExclude) == 0 {
		return ""
	}
	if dbPrefix == "" {
		dbPrefix = "wp_"
	}

	var tables []string
	for _, t := range dbExclude {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, dbPrefix) {
			tables = append(tables, t)
		} else {
			tables = append(tables, dbPrefix+t)
		}
	}

	if len(tables) == 0 {
		return ""
	}
	return fmt.Sprintf("--exclude_tables=%s", strings.Join(tables, ","))
}

// DecompressGzip extracts a .gz file to destPath.
func DecompressGzip(gzPath, destPath string) error {
	gzFile, err := os.Open(gzPath)
	if err != nil {
		return err
	}
	defer gzFile.Close()

	gzReader, err := gzip.NewReader(gzFile)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, gzReader); err != nil {
		_ = os.Remove(destPath)
		return err
	}

	return nil
}

// FetchRemoteDatabase runs wp db export on the remote host via the host's ssh command,
// streaming stdout directly to destPath.
func FetchRemoteDatabase(remoteHost, remotePath string, excludeArg string, destPath string) error {
	host, port := ParseSSHHostPort(remoteHost)
	if host == "" {
		return errors.New("remoteHost is empty")
	}
	if remotePath == "" {
		return errors.New("remotePath is empty")
	}

	remoteCmdParts := []string{
		"wp", "db", "export",
	}
	if excludeArg != "" {
		remoteCmdParts = append(remoteCmdParts, excludeArg)
	}
	remoteCmdParts = append(remoteCmdParts,
		"--single-transaction",
		"--quick",
		"--lock-tables=false",
		"--skip-plugins",
		"--skip-themes",
		fmt.Sprintf("--path=%s", remotePath),
		"-",
	)
	remoteCmd := strings.Join(remoteCmdParts, " ")

	sshArgs := []string{
		"-T",
		"-o", "ConnectTimeout=10",
	}
	if port != "" {
		sshArgs = append(sshArgs, "-p", port)
	}
	sshArgs = append(sshArgs, host, remoteCmd)

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
	}
	defer destFile.Close()

	cmd := exec.Command("ssh", sshArgs...)
	cmd.Stdout = destFile
	cmd.Stderr = os.Stderr

	var progress *progressWriter
	if isTerminalOutput(os.Stderr) {
		progress = newProgressWriter(destFile, os.Stderr)
		cmd.Stdout = progress
	}

	err = cmd.Run()
	if progress != nil {
		progress.finish()
	}
	if err != nil {
		_ = os.Remove(destPath)
		return fmt.Errorf("ssh export failed: %w", err)
	}

	if info, err := os.Stat(destPath); err == nil && info.Size() == 0 {
		_ = os.Remove(destPath)
		return errors.New("remote database export produced an empty file")
	}

	return nil
}

// ImportSQLDump streams the SQL file from the host directly into MariaDB in the db container.
func ImportSQLDump(projectDir string, sqlFilePath string) error {
	dbUser, dbPassword, dbName := getDBCredentials(projectDir)

	file, err := os.Open(sqlFilePath)
	if err != nil {
		return fmt.Errorf("failed to open sql dump %s: %w", sqlFilePath, err)
	}
	defer file.Close()

	var stdin io.Reader = file
	if info, err := file.Stat(); err == nil && info.Size() > 0 {
		fmt.Printf("Importing %s...\n", formatBytes(info.Size()))
		if isTerminalOutput(os.Stderr) {
			progress := newProgressReader(file, info.Size(), os.Stderr)
			defer progress.finish()
			stdin = progress
		}
	}

	mariadbArgs := []string{
		"sh", "-c",
		`if command -v mariadb >/dev/null 2>&1; then exec mariadb "$@"; else exec mysql "$@"; fi`,
		"--",
		"--default-character-set=utf8mb4",
		"-u", dbUser,
		"-p" + dbPassword,
		dbName,
	}

	return docker.ComposeExecWithStdin(projectDir, "db", mariadbArgs, stdin)
}

// progress redraws a single status line on out as bytes are transferred,
// so multi-GB fetches and imports don't look like a hang. A total of 0 means
// the size is unknown (e.g. a streamed remote export), so no percentage is
// shown.
type progress struct {
	out      io.Writer
	total    int64
	done     int64
	start    time.Time
	lastDraw time.Time
}

func newProgress(out io.Writer, total int64) *progress {
	return &progress{out: out, total: total, start: time.Now()}
}

func (p *progress) add(n int) {
	p.done += int64(n)
	if time.Since(p.lastDraw) >= 500*time.Millisecond {
		p.draw()
	}
}

func (p *progress) draw() {
	p.lastDraw = time.Now()
	elapsed := time.Since(p.start)
	rate := ""
	if secs := elapsed.Seconds(); secs >= 1 {
		rate = fmt.Sprintf("  %s/s", formatBytes(int64(float64(p.done)/secs)))
	}
	if p.total > 0 {
		fmt.Fprintf(p.out, "\r\033[K  %5.1f%%  %s / %s%s  (%s)",
			float64(p.done)*100/float64(p.total), formatBytes(p.done), formatBytes(p.total), rate, elapsed.Round(time.Second))
	} else {
		fmt.Fprintf(p.out, "\r\033[K  %s%s  (%s)", formatBytes(p.done), rate, elapsed.Round(time.Second))
	}
}

// finish draws the final state and ends the progress line.
func (p *progress) finish() {
	p.draw()
	fmt.Fprintln(p.out)
}

// progressReader reports bytes read from r.
type progressReader struct {
	*progress
	r io.Reader
}

func newProgressReader(r io.Reader, total int64, out io.Writer) *progressReader {
	return &progressReader{progress: newProgress(out, total), r: r}
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	p.add(n)
	return n, err
}

// progressWriter reports bytes written to w.
type progressWriter struct {
	*progress
	w io.Writer
}

func newProgressWriter(w io.Writer, out io.Writer) *progressWriter {
	return &progressWriter{progress: newProgress(out, 0), w: w}
}

func (p *progressWriter) Write(buf []byte) (int, error) {
	n, err := p.w.Write(buf)
	p.add(n)
	return n, err
}

// formatBytes renders a byte count with a binary unit suffix (e.g. "2.2 GB").
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// isTerminalOutput reports whether f is attached to a terminal, so carriage
// return redraws aren't written into redirected logs.
func isTerminalOutput(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// RotateSQLFiles rotates update.sql to db.sql, and existing db.sql to db.old.sql.
func RotateSQLFiles(sqlDir string) {
	dbSql := filepath.Join(sqlDir, "db.sql")
	dbOldSql := filepath.Join(sqlDir, "db.old.sql")
	updateSql := filepath.Join(sqlDir, "update.sql")

	if _, err := os.Stat(dbSql); err == nil {
		_ = os.Rename(dbSql, dbOldSql)
	}
	if _, err := os.Stat(updateSql); err == nil {
		_ = os.Rename(updateSql, dbSql)
	}
}

// UpdateTablePrefix updates the table_prefix in wp-config.php via wp-cli in the wordpress container.
func UpdateTablePrefix(projectDir string, dbPrefix string) error {
	if dbPrefix == "" {
		dbPrefix = "wp_"
	}
	return docker.ComposeExec(projectDir, "wordpress", []string{
		"wp", "config", "set", "--path=/var/www/html", "--skip-plugins", "--skip-themes", "--type=variable", "table_prefix", dbPrefix,
	})
}

// ApplyDomainReplacements performs search-replace for domains and configured replace rules.
func ApplyDomainReplacements(projectDir string) error {
	remoteDomain := viper.GetString("remoteDomain")
	localDomain := LocalDomain()

	replaceRules := viper.GetStringSlice("replace")
	if remoteDomain != "" && localDomain != "" {
		defaultRule := fmt.Sprintf("//%s|//%s", remoteDomain, localDomain)
		if !slices.Contains(replaceRules, defaultRule) {
			replaceRules = append([]string{defaultRule}, replaceRules...)
		}
	}

	for _, rule := range replaceRules {
		parts := strings.Split(rule, "|")
		if len(parts) >= 2 {
			from := strings.TrimSpace(parts[0])
			to := strings.TrimSpace(parts[1])
			if from == "" || to == "" {
				continue
			}
			fmt.Printf("Replacing %s with %s...\n", from, to)
			err := docker.ComposeExec(projectDir, "wordpress", []string{
				"wp", "search-replace", "--network", "--path=/var/www/html", "--skip-plugins", "--skip-themes",
				from, to, "--recurse-objects", "--report-changed-only", "--skip-columns=guid",
			})
			if err != nil {
				logging.Warn("Search-replace warning for %s -> %s: %v", from, to, err)
			}
		}
	}
	return nil
}

// getDBCredentials resolves the database credentials from viper or .env.
func getDBCredentials(projectDir string) (user, password, name string) {
	user = viper.GetString("wpDbUser")
	password = viper.GetString("wpDbPassword")
	name = viper.GetString("wpDbName")

	if user == "" || password == "" || name == "" {
		envPath := filepath.Join(projectDir, ".env")
		if content, err := os.ReadFile(envPath); err == nil {
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					k := strings.TrimSpace(parts[0])
					v := strings.Trim(strings.TrimSpace(parts[1]), "\"")
					switch k {
					case "WP_DB_USER":
						if user == "" {
							user = v
						}
					case "WP_DB_PASSWORD":
						if password == "" {
							password = v
						}
					case "WP_DB_NAME":
						if name == "" {
							name = v
						}
					}
				}
			}
		}
	}

	if user == "" {
		user = "wordpress"
	}
	if password == "" {
		password = "wordpress"
	}
	if name == "" {
		name = "wordpress"
	}
	return user, password, name
}
