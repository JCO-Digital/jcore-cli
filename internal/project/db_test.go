package project

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestParseSSHHostPort(t *testing.T) {
	tests := []struct {
		input        string
		expectedHost string
		expectedPort string
	}{
		{"user@example.com", "user@example.com", ""},
		{"user@example.com:2222", "user@example.com", "2222"},
		{"example.com:2222", "example.com", "2222"},
		{"example.com", "example.com", ""},
		{"user@[::1]:2222", "user@::1", "2222"},
		{"", "", ""},
		{"   ", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			host, port := ParseSSHHostPort(tt.input)
			if host != tt.expectedHost {
				t.Errorf("host = %q, want %q", host, tt.expectedHost)
			}
			if port != tt.expectedPort {
				t.Errorf("port = %q, want %q", port, tt.expectedPort)
			}
		})
	}
}

func TestBuildExcludeTablesArg(t *testing.T) {
	tests := []struct {
		name     string
		exclude  []string
		prefix   string
		expected string
	}{
		{
			name:     "empty exclude",
			exclude:  []string{},
			prefix:   "wp_",
			expected: "",
		},
		{
			name:     "single table without prefix",
			exclude:  []string{"users"},
			prefix:   "wp_",
			expected: "--exclude_tables=wp_users",
		},
		{
			name:     "table with prefix and table without prefix",
			exclude:  []string{"wp_users", "posts", "comments"},
			prefix:   "wp_",
			expected: "--exclude_tables=wp_users,wp_posts,wp_comments",
		},
		{
			name:     "custom prefix",
			exclude:  []string{"custom_cache", "sessions"},
			prefix:   "custom_",
			expected: "--exclude_tables=custom_cache,custom_sessions",
		},
		{
			name:     "default prefix when empty",
			exclude:  []string{"users"},
			prefix:   "",
			expected: "--exclude_tables=wp_users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildExcludeTablesArg(tt.exclude, tt.prefix)
			if result != tt.expected {
				t.Errorf("BuildExcludeTablesArg() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestDecompressGzip(t *testing.T) {
	tmpDir := t.TempDir()
	gzPath := filepath.Join(tmpDir, "test.sql.gz")
	destPath := filepath.Join(tmpDir, "test.sql")

	content := "CREATE TABLE test (id INT);"

	// Create gzip file
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write gzip buffer: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}
	if err := os.WriteFile(gzPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("failed to write gz file: %v", err)
	}

	// Decompress
	if err := DecompressGzip(gzPath, destPath); err != nil {
		t.Fatalf("DecompressGzip failed: %v", err)
	}

	out, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read destPath: %v", err)
	}
	if string(out) != content {
		t.Errorf("decompressed content = %q, want %q", string(out), content)
	}
}

func TestRotateSQLFiles(t *testing.T) {
	tmpDir := t.TempDir()

	dbSql := filepath.Join(tmpDir, "db.sql")
	updateSql := filepath.Join(tmpDir, "update.sql")
	dbOldSql := filepath.Join(tmpDir, "db.old.sql")

	if err := os.WriteFile(dbSql, []byte("old db content"), 0644); err != nil {
		t.Fatalf("failed to write db.sql: %v", err)
	}
	if err := os.WriteFile(updateSql, []byte("new update content"), 0644); err != nil {
		t.Fatalf("failed to write update.sql: %v", err)
	}

	RotateSQLFiles(tmpDir)

	// db.old.sql should have old db content
	oldOut, err := os.ReadFile(dbOldSql)
	if err != nil {
		t.Fatalf("failed to read db.old.sql: %v", err)
	}
	if string(oldOut) != "old db content" {
		t.Errorf("db.old.sql content = %q, want 'old db content'", string(oldOut))
	}

	// db.sql should have new update content
	newOut, err := os.ReadFile(dbSql)
	if err != nil {
		t.Fatalf("failed to read db.sql: %v", err)
	}
	if string(newOut) != "new update content" {
		t.Errorf("db.sql content = %q, want 'new update content'", string(newOut))
	}

	// update.sql should no longer exist
	if _, err := os.Stat(updateSql); !os.IsNotExist(err) {
		t.Errorf("update.sql still exists after rotation")
	}
}

func TestGetDBCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	viper.Reset()
	defer viper.Reset()

	// 1. Defaults when nothing is configured
	u, p, n := getDBCredentials(tmpDir)
	if u != "wordpress" || p != "wordpress" || n != "wordpress" {
		t.Errorf("defaults: got (%s, %s, %s), want (wordpress, wordpress, wordpress)", u, p, n)
	}

	// 2. Fallback to .env file
	envContent := `
WP_DB_USER="custom_user"
WP_DB_PASSWORD="custom_password"
WP_DB_NAME="custom_db"
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}
	u, p, n = getDBCredentials(tmpDir)
	if u != "custom_user" || p != "custom_password" || n != "custom_db" {
		t.Errorf("from .env: got (%s, %s, %s), want (custom_user, custom_password, custom_db)", u, p, n)
	}

	// 3. Viper takes precedence
	viper.Set("wpDbUser", "viper_user")
	viper.Set("wpDbPassword", "viper_password")
	viper.Set("wpDbName", "viper_db")
	u, p, n = getDBCredentials(tmpDir)
	if u != "viper_user" || p != "viper_password" || n != "viper_db" {
		t.Errorf("from viper: got (%s, %s, %s), want (viper_user, viper_password, viper_db)", u, p, n)
	}
}
