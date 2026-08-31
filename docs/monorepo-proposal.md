# JCore Monorepo & Integration Overhaul Proposal

This document outlines the plan to merge `jcore-cli` and `wordpress-container` into a single monorepo and redesign their integration.

## Current State & Issues

- **Distributed logic**: The CLI contains logic for project management, while the `wordpress-container` contains the project skeleton, Docker setup, and shell scripts.
- **Fragile Updates**: The CLI updates projects by downloading a `release.zip` from GitHub. This is slow, requires internet access, and can fail if the release is mismatched.
- **Configuration Duplication**: Configuration is handled by the CLI (TOML) and then passed to the container via `.env` files or other means.
- **Version Mismatch**: It's easy for a new CLI version to be incompatible with an older container template, or vice versa, without a clear versioning link.

## Proposed Monorepo Structure

```text
jcore/
├── cli/              # Redesigned Go CLI
├── container/        # Project skeleton and Docker setup (formerly wordpress-container)
│   ├── base/         # Shared files (docker-compose, nginx, etc.)
│   ├── templates/    # Specific project templates (jcore3, blank, etc.)
│   └── scripts/      # Helper scripts (potentially moved into CLI)
├── docs/             # Unified documentation
└── Makefile          # Global build and task orchestration
```

## Integration Redesign (Go Version)

### 1. Embedded Assets
Using Go's `//go:embed` directive, the entire `container/` directory can be bundled into the `jcore` binary.
- **Benefit**: Zero-latency project initialization and updates. No internet required for `init` or `update`.
- **Consistency**: The CLI version is always guaranteed to work with the bundled templates.

### 2. Unified Configuration
Instead of the CLI just writing a `.env` file, it can act as the single source of truth for configuration.
- The CLI parses `jcore.toml`.
- It manages the Docker environment directly using the Docker SDK or more robust `docker compose` calls.
- It can dynamically generate Docker Compose overrides or Nginx configs based on the TOML settings, reducing the need for complex shell scripts.

### 3. CLI-Native Commands
Many tasks currently handled by shell scripts in `.config/scripts/` should be moved into the CLI:
- `importdb`, `importmedia`, `importplugins` can become `jcore pull db`, `jcore pull media`, etc., implemented in Go.
- This allows for better error handling, progress bars, and platform independence.

### 4. Overhauling the "Container"
The `wordpress-container` should be simplified to be a "skeleton":
- **Remove logic**: Move script logic to the CLI.
- **Standardize structure**: Ensure a consistent layout that the CLI can easily manage.
- **Template focus**: Make it easier to add new templates by simply adding a directory in `container/templates/`.

## Benefits of the Monorepo

1.  **Simplified CI/CD**: A single pipeline can test both the CLI and the container templates together.
2.  **Atomic Changes**: A single commit/PR can update a template and the CLI logic that supports it.
3.  **Easier Contribution**: Contributors only need to clone one repository to work on the entire JCore ecosystem.
4.  **Robust Versioning**: Release versions of the `jcore` binary will naturally include the matching "container" version.

## Next Steps

1.  **Repository Migration**: Merge the two repositories into a new monorepo structure.
2.  **Go CLI Scaffolding**: Initialize the Go project in `cli/` and set up `cobra` and `viper`.
3.  **Asset Embedding**: Implement the `init` command using `go:embed` to write the project skeleton.
4.  **Logic Migration**: Identify the most critical shell scripts in `.config/scripts/` and rewrite them as Go commands.
