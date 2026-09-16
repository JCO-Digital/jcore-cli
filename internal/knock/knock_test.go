package knock

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type mockKnocker struct {
	knocked []KnockPort
	hosts   []string
}

func (m *mockKnocker) KnockOne(host string, kp KnockPort, timeout time.Duration) error {
	m.hosts = append(m.hosts, host)
	m.knocked = append(m.knocked, kp)
	return nil
}

func TestParsePorts(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      []KnockPort
		expectErr bool
	}{
		{
			name:  "empty",
			input: "",
			want:  nil,
		},
		{
			name:  "whitespace only",
			input: "   ",
			want:  nil,
		},
		{
			name:  "comma separated ports default to tcp",
			input: "7000,8000,9000",
			want: []KnockPort{
				{Port: 7000, Protocol: "tcp"},
				{Port: 8000, Protocol: "tcp"},
				{Port: 9000, Protocol: "tcp"},
			},
		},
		{
			name:  "space separated ports",
			input: "7000 8000 9000",
			want: []KnockPort{
				{Port: 7000, Protocol: "tcp"},
				{Port: 8000, Protocol: "tcp"},
				{Port: 9000, Protocol: "tcp"},
			},
		},
		{
			name:  "semicolon separated with whitespace",
			input: " 7000 ; 8000 ; 9000 ",
			want: []KnockPort{
				{Port: 7000, Protocol: "tcp"},
				{Port: 8000, Protocol: "tcp"},
				{Port: 9000, Protocol: "tcp"},
			},
		},
		{
			name:  "ports with explicit protocols using colon",
			input: "7000:tcp,8000:udp,9000:TCP",
			want: []KnockPort{
				{Port: 7000, Protocol: "tcp"},
				{Port: 8000, Protocol: "udp"},
				{Port: 9000, Protocol: "tcp"},
			},
		},
		{
			name:  "ports with explicit protocols using slash",
			input: "7000/tcp 8000/udp 9000/udp",
			want: []KnockPort{
				{Port: 7000, Protocol: "tcp"},
				{Port: 8000, Protocol: "udp"},
				{Port: 9000, Protocol: "udp"},
			},
		},
		{
			name:      "invalid port string",
			input:     "7000,invalid,9000",
			expectErr: true,
		},
		{
			name:      "port out of range low",
			input:     "0,8000",
			expectErr: true,
		},
		{
			name:      "port out of range high",
			input:     "70000,8000",
			expectErr: true,
		},
		{
			name:      "unsupported protocol",
			input:     "7000:http",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePorts(tt.input)
			if (err != nil) != tt.expectErr {
				t.Fatalf("ParsePorts(%q) error = %v, expectErr = %v", tt.input, err, tt.expectErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParsePorts(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"   ", ""},
		{"example.com", "example.com"},
		{"user@example.com", "example.com"},
		{"user@example.com:22", "example.com"},
		{"example.com:2222", "example.com"},
		{"user@192.168.1.100", "192.168.1.100"},
		{"192.168.1.100:22", "192.168.1.100"},
		{"user@[2001:db8::1]:22", "2001:db8::1"},
		{"[2001:db8::1]", "2001:db8::1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ExtractHost(tt.input)
			if got != tt.want {
				t.Errorf("ExtractHost(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSendKnockSequence(t *testing.T) {
	mock := &mockKnocker{}
	ports := []KnockPort{
		{Port: 7000, Protocol: "tcp"},
		{Port: 8000, Protocol: "udp"},
		{Port: 9000, Protocol: "tcp"},
	}

	err := SendKnockSequence("example.com", ports, mock, 0, 0)
	if err != nil {
		t.Fatalf("SendKnockSequence failed: %v", err)
	}

	if len(mock.knocked) != 3 {
		t.Fatalf("expected 3 knocks, got %d", len(mock.knocked))
	}
	if !reflect.DeepEqual(mock.knocked, ports) {
		t.Errorf("knocked ports = %#v, want %#v", mock.knocked, ports)
	}
	for _, h := range mock.hosts {
		if h != "example.com" {
			t.Errorf("knocked host = %q, want example.com", h)
		}
	}
}

func TestStatePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	state, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if len(state) != 0 {
		t.Fatalf("expected empty state, got %v", state)
	}

	now := time.Now().Truncate(time.Second)
	state["server1.example.com"] = now
	state["server2.example.com"] = now.Add(-5 * time.Minute)

	if err := SaveState(state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	loaded, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState (reload) failed: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(loaded))
	}
	if !loaded["server1.example.com"].Equal(now) {
		t.Errorf("loaded time = %v, want %v", loaded["server1.example.com"], now)
	}
}

func TestKnockHostIfNeeded(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	mock := &mockKnocker{}
	ActiveKnocker = mock
	defer func() {
		ActiveKnocker = DefaultKnocker{}
	}()

	// 1. Unset/empty knockdPorts should not knock
	knocked, err := KnockHostIfNeeded("user@example.com", "", 60)
	if err != nil {
		t.Fatalf("KnockHostIfNeeded error = %v", err)
	}
	if knocked {
		t.Error("expected knocked = false for empty knockdPorts")
	}
	if len(mock.knocked) != 0 {
		t.Errorf("expected 0 knocks, got %d", len(mock.knocked))
	}

	// 2. Empty remoteHost should not knock
	knocked, err = KnockHostIfNeeded("", "7000,8000,9000", 60)
	if err != nil {
		t.Fatalf("KnockHostIfNeeded error = %v", err)
	}
	if knocked {
		t.Error("expected knocked = false for empty remoteHost")
	}

	// 3. First knock should execute
	knocked, err = KnockHostIfNeeded("user@server.example.com", "7000,8000:udp,9000", 60)
	if err != nil {
		t.Fatalf("KnockHostIfNeeded error = %v", err)
	}
	if !knocked {
		t.Error("expected knocked = true on first attempt")
	}
	if len(mock.knocked) != 3 {
		t.Fatalf("expected 3 knocks, got %d", len(mock.knocked))
	}

	// 4. Second request within timeout window should NOT knock
	mock.knocked = nil
	knocked, err = KnockHostIfNeeded("user@server.example.com", "7000,8000:udp,9000", 60)
	if err != nil {
		t.Fatalf("KnockHostIfNeeded error = %v", err)
	}
	if knocked {
		t.Error("expected knocked = false when within timeout window")
	}
	if len(mock.knocked) != 0 {
		t.Errorf("expected 0 knocks, got %d", len(mock.knocked))
	}

	// 5. A different host should still knock
	knocked, err = KnockHostIfNeeded("user@other.example.com", "7000,8000,9000", 60)
	if err != nil {
		t.Fatalf("KnockHostIfNeeded error = %v", err)
	}
	if !knocked {
		t.Error("expected knocked = true for different host")
	}
	if len(mock.knocked) != 3 {
		t.Fatalf("expected 3 knocks for other host, got %d", len(mock.knocked))
	}

	// 6. After timeout has expired, host should be knocked again
	state, _ := LoadState()
	state["server.example.com"] = time.Now().Add(-70 * time.Second)
	_ = SaveState(state)

	mock.knocked = nil
	knocked, err = KnockHostIfNeeded("user@server.example.com", "7000,8000:udp,9000", 60)
	if err != nil {
		t.Fatalf("KnockHostIfNeeded error = %v", err)
	}
	if !knocked {
		t.Error("expected knocked = true after timeout expired")
	}
	if len(mock.knocked) != 3 {
		t.Fatalf("expected 3 knocks after timeout, got %d", len(mock.knocked))
	}
}

func TestCorruptStateFileIsRecovered(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	stateFile := filepath.Join(tmpHome, ".config", "jcore", "knock-state.json")
	_ = os.MkdirAll(filepath.Dir(stateFile), 0755)
	_ = os.WriteFile(stateFile, []byte("invalid json content"), 0644)

	mock := &mockKnocker{}
	ActiveKnocker = mock
	defer func() {
		ActiveKnocker = DefaultKnocker{}
	}()

	knocked, err := KnockHostIfNeeded("user@server.example.com", "7000,8000", 20)
	if err != nil {
		t.Fatalf("expected recovery from corrupt state file, got error: %v", err)
	}
	if !knocked {
		t.Error("expected knocked = true")
	}
}
