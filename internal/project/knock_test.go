package project

import (
	"testing"

	"github.com/JCO-Digital/jcore/internal/knock"
	"github.com/spf13/viper"
)

type projectMockKnocker struct {
	knocked []knock.KnockPort
	hosts   []string
}

func (m *projectMockKnocker) KnockOne(host string, kp knock.KnockPort, timeout timeDuration) error {
	m.hosts = append(m.hosts, host)
	m.knocked = append(m.knocked, kp)
	return nil
}

type timeDuration = testing.TB // dummy to avoid unused import if needed

func TestKnockIfNeeded(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	defer viper.Reset()

	viper.Set("remoteHost", "user@my-server.example.com")
	viper.Set("knockdPorts", "7000,8000:udp,9000")
	viper.Set("knockdTimeout", 20)

	// Verify KnockIfNeeded runs without panicking or erroring
	KnockIfNeeded()
}
