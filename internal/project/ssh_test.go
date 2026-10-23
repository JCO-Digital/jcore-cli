package project

import (
	"slices"
	"testing"
)

func TestBuildSSHArgs(t *testing.T) {
	tests := []struct {
		name       string
		remoteHost string
		remotePath string
		command    []string
		want       []string
	}{
		{
			name:       "shell without path",
			remoteHost: "user@server.example.com",
			want:       []string{"-t", "user@server.example.com"},
		},
		{
			name:       "shell with port and path",
			remoteHost: "user@server.example.com:2222",
			remotePath: "/var/www/site/",
			want:       []string{"-t", "-p", "2222", "user@server.example.com", `cd '/var/www/site' 2>/dev/null; exec "$SHELL" -l`},
		},
		{
			name:       "path with single quote",
			remoteHost: "user@server.example.com",
			remotePath: "/var/www/it's",
			want:       []string{"-t", "user@server.example.com", `cd '/var/www/it'\''s' 2>/dev/null; exec "$SHELL" -l`},
		},
		{
			name:       "command without path",
			remoteHost: "user@server.example.com",
			command:    []string{"wp", "plugin", "list"},
			want:       []string{"user@server.example.com", "wp", "plugin", "list"},
		},
		{
			name:       "command with path",
			remoteHost: "user@server.example.com",
			remotePath: "/var/www/site",
			command:    []string{"wp", "plugin", "list"},
			want:       []string{"user@server.example.com", "cd '/var/www/site' && wp plugin list"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildSSHArgs(tt.remoteHost, tt.remotePath, tt.command)
			if err != nil {
				t.Fatalf("BuildSSHArgs failed: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildSSHArgs_EmptyHost(t *testing.T) {
	if _, err := BuildSSHArgs("  ", "/var/www", nil); err == nil {
		t.Fatal("expected an error for an empty remoteHost, got nil")
	}
}
