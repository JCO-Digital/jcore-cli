package config

// AppVersion is the CLI version. It is set at build time via
// -ldflags "-X github.com/JCO-Digital/jcore/internal/config.AppVersion=..."
// and defaults to "dev" for local builds.
var AppVersion = "dev"
