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

## Getting Started

### Prerequisites

- Go 1.22 or later.

### Building

```bash
make build
```

The binary will be created in `bin/jcore`.

### Running

```bash
./bin/jcore --help
```

## Development

This project uses [Cobra](https://github.com/spf13/cobra) for command-line parsing and [Viper](https://github.com/spf13/viper) for configuration.
