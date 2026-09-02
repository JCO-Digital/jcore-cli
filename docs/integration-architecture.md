# JCore Integration Architecture

This document describes how the JCore CLI and the WordPress Container currently interact and how that interaction will change in the redesign.

## Current Interaction Flow

1.  **Project Initialization (`jcore init`)**:
    - CLI downloads `release.zip` from `wordpress-container` repo.
    - CLI extracts files into the project directory.
    - CLI reads `templates.toml` (remote) to present template options.
    - CLI copies template-specific files from the extracted `templates/` folder.
    - CLI initializes a git repo.

2.  **Configuration Bridge (`jcore start` / `createEnv`)**:
    - The CLI maintains a `jcoreSettingsData` object (from `jcore.toml`, etc.).
    - The container provides `env-values.toml`.
    - The CLI merges these and generates a `.env` file.
    - CamelCase keys (e.g., `remoteDomain`) are converted to SNAKE_CASE (e.g., `REMOTE_DOMAIN`).
    - Docker Compose reads this `.env` file to configure the containers (WordPress, Nginx, etc.).

3.  **Project Finalization (`finalizeProject`)**:
    - CLI renders templates (like `nginx/site.conf`) using string replacement.
    - CLI sets executable permissions on scripts in `.config/scripts/`.
    - CLI runs `pnpm install` and `composer install` on the host.
    - CLI triggers `docker compose pull` and `docker compose up`.

4.  **Remote Sync (`jcore pull`)**:
    - CLI executes scripts *inside* the `wordpress` container (e.g., `docker compose exec wordpress .config/scripts/importdb`).
    - These scripts rely on the environment variables passed via the `.env` file.

## Redesign Goals (Go Version)

### 1. Direct Docker Orchestration
Instead of relying on a generated `.env` file and a shell-based `docker compose` call, the Go CLI can use the Docker Go SDK or a more controlled `docker-compose` execution.
- **Benefit**: Better error handling, real-time status updates, and no "leaky" `.env` files that might contain secrets in plain text.

### 2. Native Template Rendering
The Go version will use `text/template` for all configuration files (Nginx, PHP, Docker Compose).
- **Benefit**: Much more powerful than simple string replacement. Supports conditionals, loops, and complex logic.

### 3. Script-to-Command Migration
Critical logic in shell scripts will be ported to Go:
- **Database Import**: The CLI can handle downloading the SQL file, preparing it (string replacements), and piping it into the database container.
- **Plugin Management**: The CLI can manage the `wp-content/plugins` directory directly, handling Git submodules or direct downloads more reliably than a shell script.

### 4. Unified Asset Management
By using `go:embed`, the project "skeleton" (the parts of `wordpress-container` that aren't template-specific) and the templates themselves are part of the CLI binary.
- This eliminates the need for the `release.zip` and the complex `moveFiles` logic in `project.ts`.
- Initialization becomes an atomic `jcore init` that writes the required files to disk.
