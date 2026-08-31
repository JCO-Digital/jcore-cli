package config

type Config struct {
	ProjectName  string   `mapstructure:"projectName"`
	Theme        string   `mapstructure:"theme"`
	Branch       string   `mapstructure:"branch"`
	RemoteDomain string   `mapstructure:"remoteDomain"`
	LocalDomain  string   `mapstructure:"localDomain"`
	Domains      []string `mapstructure:"domains"`
	WpImage      string   `mapstructure:"wpImage"`
	WpVersion    string   `mapstructure:"wpVersion"`
	Debug        bool     `mapstructure:"debug"`
	LogLevel     int      `mapstructure:"logLevel"`
}

func DefaultConfig() Config {
	return Config{
		Theme:     "jcore-ilme",
		WpImage:   "jcodigi/wordpress:latest",
		WpVersion: "latest",
		LogLevel:  2,
	}
}
