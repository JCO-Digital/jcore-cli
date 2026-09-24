# JCore CLI

JCore CLI is a tool designed to manage WordPress development environments. It simplifies the process of setting up, running, and maintaining WordPress projects using Docker.

This is the Go rewrite of the CLI. The previous TypeScript implementation is kept for reference in [`legacy-ts/`](legacy-ts/).

## Project Structure

- `cmd/jcore`: Entry point for the application.
- `internal/cmd`: Cobra command definitions.
- `internal/config`: Configuration management using Viper.
- `internal/project`: Project management logic.
- `internal/docker`: Docker orchestration logic.
- `internal/container`: Monorepo assets (embedded project skeleton and templates).

## Installation

### Automated install script (recommended)

For Linux (including WSL) and macOS (Apple Silicon), you can install the latest release directly via:

```bash
curl -fsSL https://raw.githubusercontent.com/JCO-Digital/jcore-cli/main/install.sh | sh
```

Or using `wget`:

```bash
wget -qO- https://raw.githubusercontent.com/JCO-Digital/jcore-cli/main/install.sh | sh
```

The script:

1. Installs the latest `jcore` binary to `~/.local/bin/jcore` (or `~/bin/jcore` if that is in your `PATH`, preferring `~/.local/bin` if both exist).
2. Checks for older `jcore` installations elsewhere in your `PATH`, automatically removing them if user-writable or providing instructions if administrative permissions are required.
3. Automatically generates and installs shell completions for **Bash**, **Zsh**, and **Fish**.
4. Alerts you if the chosen installation directory is not currently in your `PATH` and displays instructions for adding it.

### Build from source

If you prefer building locally:

#### Prerequisites

- Go 1.24 or later.

#### Building

```bash
make build
```

The binary will be created in `bin/jcore`.

To install the binary to `~/.local/bin` and install shell completions:

```bash
make install
```

### Running

```bash
jcore --help
```

## Development

This project uses [Cobra](https://github.com/spf13/cobra) for command-line parsing and [Viper](https://github.com/spf13/viper) for configuration.
