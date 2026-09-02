# JCore CLI Overview

JCore CLI is a tool designed to manage WordPress development environments. It simplifies the process of setting up, running, and maintaining WordPress projects using Docker.

## Core Concepts

- **Project-based**: Most commands expect to be run within a JCore project directory (identified by the presence of a `jcore.toml` file).
- **Docker-centric**: It leverages Docker Compose to orchestrate containers for WordPress, databases, and other services.
- **Hierarchical Configuration**: Settings can be defined at multiple levels (Default, Global, Project, Local), allowing for flexible overrides.
- **Template-driven**: It can create new projects based on predefined templates (currently `jcore3` is the default).

## Architecture

The CLI is built with TypeScript and Node.js. It uses the following key components:

- **Parser**: Processes command-line arguments and flags.
- **Settings Manager**: Loads and merges configuration from various TOML files.
- **Command Dispatcher**: Routes execution to specific command handlers based on the input.
- **Docker Wrapper**: Executes `docker compose` commands to manage the environment.
- **File Helpers**: Utilities for file system operations, template rendering (using Mustache), and zip handling.

### Configuration Hierarchy

1. **Default**: Hardcoded defaults or `jcore.default.toml` in the project.
2. **Global**: User-wide settings in `~/.config/jcore/config.toml`.
3. **Project**: Project-specific settings in `jcore.toml`.
4. **Local**: Local overrides in `jcore.local.toml` (usually git-ignored).

### Key Technologies

- **TypeScript**: For type-safe development.
- **esbuild**: For fast bundling.
- **Zod**: For configuration schema validation.
- **smol-toml**: For parsing and writing TOML files.
- **Docker Compose**: For container orchestration.
- **Mustache**: For template rendering.
