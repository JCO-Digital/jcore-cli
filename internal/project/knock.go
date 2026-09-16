package project

import (
	"github.com/JCO-Digital/jcore/internal/knock"
	"github.com/JCO-Digital/jcore/internal/logging"
	"github.com/spf13/viper"
)

// KnockIfNeeded sends a port knock sequence to remoteHost if knockdPorts is configured
// and the host has not been knocked within knockdTimeout seconds.
func KnockIfNeeded() {
	remoteHost := viper.GetString("remoteHost")
	knockdPorts := viper.GetString("knockdPorts")
	knockdTimeout := viper.GetInt("knockdTimeout")

	if _, err := knock.KnockHostIfNeeded(remoteHost, knockdPorts, knockdTimeout); err != nil {
		logging.Warn("Port knock failed: %v", err)
	}
}
