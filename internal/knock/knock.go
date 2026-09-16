package knock

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/JCO-Digital/jcore/internal/logging"
)

// KnockPort represents a single port and protocol to knock.
type KnockPort struct {
	Port     int
	Protocol string // "tcp" or "udp"
}

// Default timeouts and delays for port knocking.
const (
	DefaultDialTimeout        = 100 * time.Millisecond
	DefaultDelayBetweenKnocks = 100 * time.Millisecond
	DefaultPostKnockDelay     = 300 * time.Millisecond
	DefaultTimeoutSeconds     = 60
)

var (
	stateMu sync.Mutex
	// ActiveKnocker is used to execute individual knocks. Replaced in tests.
	ActiveKnocker Knocker = DefaultKnocker{}
)

// Knocker is an interface for sending a single port knock.
type Knocker interface {
	KnockOne(host string, kp KnockPort, timeout time.Duration) error
}

// DefaultKnocker sends real network packets for port knocking.
type DefaultKnocker struct{}

// KnockOne attempts a connection to host on kp.Port using kp.Protocol.
func (d DefaultKnocker) KnockOne(host string, kp KnockPort, timeout time.Duration) error {
	addr := net.JoinHostPort(host, strconv.Itoa(kp.Port))
	if kp.Protocol == "udp" {
		conn, err := net.DialTimeout("udp", addr, timeout)
		if err == nil {
			_, _ = conn.Write([]byte{0})
			_ = conn.Close()
		}
		return nil
	}

	// TCP: attempt connection which sends a SYN packet
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err == nil {
		_ = conn.Close()
	}
	return nil
}

// ParsePorts parses a knockd port specification string into a slice of KnockPort.
// Supports comma-, semicolon-, or whitespace-delimited entries of port[:proto] or port[/proto].
// Protocol defaults to "tcp" if omitted.
func ParsePorts(spec string) ([]KnockPort, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}

	tokens := strings.FieldsFunc(spec, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	})

	var result []KnockPort
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		var portStr, protoStr string
		if strings.Contains(token, ":") {
			parts := strings.SplitN(token, ":", 2)
			portStr, protoStr = parts[0], parts[1]
		} else if strings.Contains(token, "/") {
			parts := strings.SplitN(token, "/", 2)
			portStr, protoStr = parts[0], parts[1]
		} else {
			portStr = token
			protoStr = "tcp"
		}

		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("invalid port %q in knock sequence %q", portStr, spec)
		}

		proto := strings.ToLower(strings.TrimSpace(protoStr))
		if proto != "tcp" && proto != "udp" {
			return nil, fmt.Errorf("invalid protocol %q (must be tcp or udp) in knock sequence %q", protoStr, spec)
		}

		result = append(result, KnockPort{
			Port:     port,
			Protocol: proto,
		})
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no valid ports in knock sequence %q", spec)
	}

	return result, nil
}

// ExtractHost extracts the hostname or IP address from an SSH host specification
// (e.g. "user@example.com", "user@example.com:22", "example.com", "[2001:db8::1]:22").
func ExtractHost(remoteHost string) string {
	host := strings.TrimSpace(remoteHost)
	if host == "" {
		return ""
	}
	if idx := strings.Index(host, "@"); idx != -1 {
		host = host[idx+1:]
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return strings.Trim(h, "[]")
	}
	return strings.Trim(host, "[]")
}

// State maps hostnames to the timestamp of their last knock.
type State map[string]time.Time

// StatePath returns the file path for the persisted knock state JSON file.
func StatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "jcore", "knock-state.json"), nil
}

// LoadState reads the persisted knock state from disk.
func LoadState() (State, error) {
	path, err := StatePath()
	if err != nil {
		return State{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, nil
		}
		return State{}, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, nil
	}
	return state, nil
}

// SaveState persists the knock state to disk.
func SaveState(state State) error {
	path, err := StatePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// SendKnockSequence sends the knock sequence to host using knocker.
func SendKnockSequence(host string, ports []KnockPort, knocker Knocker, delayBetween, postDelay time.Duration) error {
	if len(ports) == 0 || host == "" {
		return nil
	}

	for i, kp := range ports {
		if i > 0 && delayBetween > 0 {
			time.Sleep(delayBetween)
		}
		_ = knocker.KnockOne(host, kp, DefaultDialTimeout)
	}

	if postDelay > 0 {
		time.Sleep(postDelay)
	}
	return nil
}

// KnockHostIfNeeded checks whether remoteHost needs a port knock sequence based on
// knockdPorts, knockdTimeout (in seconds), and the last knock time recorded for the host.
// Returns true if a knock was performed, or false if skipped (not configured or within timeout).
func KnockHostIfNeeded(remoteHost, knockdPorts string, timeoutSeconds int) (bool, error) {
	if strings.TrimSpace(knockdPorts) == "" {
		return false, nil
	}

	host := ExtractHost(remoteHost)
	if host == "" {
		return false, nil
	}

	ports, err := ParsePorts(knockdPorts)
	if err != nil {
		return false, fmt.Errorf("parsing knockdPorts: %w", err)
	}

	if timeoutSeconds <= 0 {
		timeoutSeconds = DefaultTimeoutSeconds
	}
	timeoutDuration := time.Duration(timeoutSeconds) * time.Second

	stateMu.Lock()
	defer stateMu.Unlock()

	state, _ := LoadState()
	if lastKnock, ok := state[host]; ok {
		if time.Since(lastKnock) < timeoutDuration {
			logging.Verbose("Host %s was knocked %v ago (< %d s timeout), skipping port knock", host, time.Since(lastKnock).Round(time.Second), timeoutSeconds)
			return false, nil
		}
	}

	logging.Info("Sending port knock to %s...", host)
	if err := SendKnockSequence(host, ports, ActiveKnocker, DefaultDelayBetweenKnocks, DefaultPostKnockDelay); err != nil {
		return false, err
	}

	if state == nil {
		state = make(State)
	}
	state[host] = time.Now()
	_ = SaveState(state)

	return true, nil
}
