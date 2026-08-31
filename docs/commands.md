# JCore CLI Commands

This document describes the available commands in JCore CLI.

## `init`
Creates a new JCore project. It will prompt for:
- Project Name
- Template (e.g., `jcore3`)
- Branch
- Domain settings

It sets up the project directory, initializes a Git repository, and creates the necessary configuration files.

## `start`
Starts the WordPress environment for the current project.
- Runs `docker compose up`.
- If `mode` is `foreground` (default), it stays in the foreground.
- Use `--install` flag to force re-running the project finalization (updating files from template).

## `stop`
Stops the WordPress environment for the current project.
- Runs `docker compose stop`.

## `attach`
Attaches to the logs of the running containers.
- Runs `docker compose logs -f`.

## `shell`
Opens a bash shell inside the `wordpress` container.
- Runs `docker compose exec wordpress /bin/bash`.

## `run <command>`
Runs a specific command inside the `wordpress` container.
- Example: `jcore run "wp plugin list"`

## `pull [plugins|db|media|all]`
Pulls data from the remote environment to the local environment.
- Defaults to `plugins` and `db` if no target is specified.
- Executes import scripts inside the `wordpress` container.
- Can take a `--dbfile <filename>` flag to import a specific SQL file.

## `clone <repository> [name]`
Clones an existing JCore project from a Git repository.
- If only a name is given, it uses the `projectDefault` setting to construct the Git URL.
- Automatically initializes submodules and runs project setup.

## `update`
Updates the current project files from the template.
- You can specify specific targets to update.

## `update self`
Updates the JCore CLI itself to the latest version.
- Supports both `pnpm` global updates and direct binary replacement.

## `config`
Manages configuration settings.
- `jcore config list [active|global|project|local|all]`: Lists settings.
- `jcore config set <key> <value>`: Sets a configuration value.
    - Special pseudo-setters:
        - `wpe <name>`: Sets up WP Engine remote settings.
        - `php <version>`: Sets the WordPress PHP image version.
- `jcore config unset <key>`: Removes a configuration setting.
- Supports `--global`, `--project`, and `--local` flags to specify the configuration scope.

## `checksum`
Manages file checksums to track changes in core files.
- `jcore checksum list`: Lists files and their checksum status (OK, Changed, Missing).
- `jcore checksum set <file1> <file2> ...`: Sets the current checksum for the specified files.

## `doctor`
Checks the system for potential issues.
- Verifies that necessary folders exist and have correct permissions.
- Checks if required external commands (like `docker`, `git`) are installed and available.

## `migrate`
Migrates a legacy JCore project to the current format.

## `create`
- `jcore create block`: Prompts to create a new Gutenberg block using Lohko templates.
- `jcore create user`: Prompts to create a new WordPress user in the running environment.

## `status`
Shows which JCore projects are currently running.

## `clean [all|docker]`
- `jcore clean`: Cleans containers and volumes for the current project.
- `jcore clean all`: Cleans all non-running JCore projects and prunes Docker.
- `jcore clean docker`: Prunes Docker containers, images, volumes, and networks.
