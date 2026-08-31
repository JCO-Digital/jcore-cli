package constants

type ExternalCommand struct {
	Name    string
	Version string
}

var ProjectFolders = []string{
	".jcore/wordpress",
	".jcore/ssl",
	".jcore/sql",
}

var GlobalFolders = []string{
	".config/jcore/ssl",
	".config/jcore/ssh",
}

const ChecksumFile = ".file.checksums.json"

var ExternalCommands = []ExternalCommand{
	{Name: "docker", Version: "-v"},
	{Name: "docker-compose", Version: "-v"},
	{Name: "git", Version: "--version"},
	{Name: "composer", Version: "-V"},
	{Name: "pnpm", Version: "-v"},
}
