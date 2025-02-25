# JCORE CLI

This is a helper app for running jcore and other WordPress projects.

Because of the nature of this utility, it runs mostly synchronously. As such it might seem like "bad code", but it helps keep the codebase clean. It's OK to use promises as well, but optimally try to keep functions synchronous if all other things being equal.

## Code formatting and linting

jcore-cli uses quite strict linting and formatting checking, if you commit something with invalid formatting, the drone build will fail. You can test everything with `npm test` before you commit, and if formatting needs fixing, run `npm run format`. That fixes everything up.

## Testing the code locally

You can have esbuild continuously build the executable for you with `npm run watch`. But to properly test it, you need to run `source add-to-path.sh`, this will temporarily prepend your build directory to your PATH variable. Note that this only work for the shell you ran the command from, and goes away if you close the shell. Leave the version as it is during development commits.

## Publishing the code

You can commit and push your commits on main branch to your hearts content, the CI will not publish the executable unless you tag the release. You can create a realease by running `npm version patch|minor|major`, depending on what number you want to bump. The project follows Semantic Versioning, so:

- Major should be used for breaking changes.
- Minor should be used when new features are added.
- Patch is used for bugfixes and smaller changes.

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for the latest changes.

## Commands

The JCORE CLI provides a set of commands to help manage your WordPress projects. Here's a breakdown of each command and its usage:

### `attach`

Attach to the logs of all containers.

- **Usage:** `jcore attach [<container>]`
  - `<container>`: An optional specific container to attach to. If left empty, attaches to all containers.

### `checksum`

Manages file checksums. This is used to check which files have been changed manually and should not be overwritten automatically.

- **Usage:** `jcore checksum <list | set> [...]`
  - `list`: Lists all checksums and whether they match the current file contents.
  - `set [<filename>]`: Calculates and sets checksums for the given files. If no filenames are provided, it will re-calculate checksums for all files.

### `clean`

Delete image/container/temp files.

- **Usage:** `jcore clean [docker | all]`
  - `docker`: Clean dangling images, containers, and volumes.
  - `all`: Cleans up after all projects (may delete non-JCORE Docker data).

### `clone`

Clones a project from a Git repository and sets everything up.

- **Usage:** `jcore clone <projectname> [<target_directory>]`
  - `<projectname>`: The name or full Git URL of the project to clone.
  - `<target_directory>`: (Optional) The directory to clone the project into. Defaults to the project name.

### `migrate`

Migrates a legacy project to the new container format.

- **Usage:** `jcore migrate`

### `doctor`

Checks the status of the environment.

- **Usage:** `jcore doctor`

### `init`

Creates a new project.

- **Usage:** `jcore init <projectname> [--template <template_name>] [--notheme]`
  - `<projectname>`: The name of the new project.
  - `--template <template_name>`: (Optional) The template to use for the project.
  - `--notheme`: (Optional) Skip installing the default theme.

### `pull`

Syncs content from upstream (database and/or plugins).

- **Usage:** `jcore pull [db | plugins | media | all]`
  - `db`: Sync the database.
  - `plugins`: Sync plugins.
  - `media`: Sync media files.
  - `all`: Sync everything (database, plugins, and media).

### `run`

Runs a command in the WordPress container.

- **Usage:** `jcore run <command>`
  - `<command>`: The command to execute in the container.

### `config`

Set/list options in the config file.

- **Usage:** `jcore config <list | set | unset> [...]`
  - `list`: Lists all settings.
  - `set <key> <value>`: Sets a config value.
  - `unset <key>`: Removes a setting, likely returning it to the default.

### `shell`

Opens a shell in the WordPress container.

- **Usage:** `jcore shell`

### `status`

Shows information about running projects.

- **Usage:** `jcore status`

### `start`

Installs dependencies and starts the container.

- **Usage:** `jcore start [--force]`
  - `--force`: Stops any other running JCORE projects first.

### `stop`

Shutdown the container.

- **Usage:** `jcore stop`

### `update`

Updates the project or the CLI itself.

- **Usage:** `jcore update [self | <filename>]`
  - `self`: Updates the CLI.
  - `<filename>`: Updates only the selected files. If no filename is provided updates all the files.

## Options

The following options can be used with the JCORE CLI:

- `--help, -h`: Display help information.
- `--global, -g`: Write settings globally.
- `--local, -l`: Write settings locally.
- `--template, -t <template_name>`: Set template to use.
- `--branch, -b <branch_name>`: Specify branch name.
- `--verbose, -v`: Print more text.
- `--debug, -d`: Print everything.
- `--quiet, -q`: Print only errors.
- `--loglevel, -p <level>`: Set numeric log level.
- `--install, -i`: Installs dependencies.
- `--notheme, -n`: Doesn't install theme on init command.
- `--force, -f`: Overwrites existing files.
