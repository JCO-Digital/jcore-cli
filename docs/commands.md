# JCore CLI Commands

This document describes the available commands in JCore CLI.

## Global flags

Every command accepts:
- `--verbose`/`-v`: print more output (e.g. which config files were loaded).
- `--debug`/`-d`: print everything.
- `--quiet`/`-q`: print only errors.
- `--loglevel <n>`: set the numeric log level directly (0=error, 1=warn,
  2=info (default), 3=http, 4=verbose, 5=debug, 6=silly), overriding
  `-v`/`-d`/`-q`.

With none of these passed, the effective level comes from the persisted
`logLevel` setting (default `2`/info). A flag always wins over that
setting for the current invocation only.

## `init [name]`
Creates a new JCore project in a new directory (a sibling of the current
one), named after the project (slugified — `projectName` itself keeps the
name as given). Prompts interactively for anything not already given via
the `[name]` argument or `--template`/`--branch`:
- Project name, if `[name]` wasn't given.
- `--template`/`-t`: which embedded template to scaffold, from the
  template catalog (`jcore3`, `jcore2`, `jcore1`, `blank`). If given
  explicitly, an unknown template is an error, not a fallback.
- `--branch`/`-b`: git branch of the theme/plugins to track. If not given
  and the chosen template offers more than one branch, prompts to pick
  one; otherwise defaults to the template's own default branch (e.g.
  `hurricane` for `jcore3`).
- Unless `--notheme`/`-n` is passed, it also downloads and creates a child
  theme at `wp-content/themes/<slugified-project-name>` from the template's
  theme repository (for `jcore3`, `jcore-ilme` at the chosen branch),
  rewriting the theme's own `style.css` "Theme Name:" header and the
  project's `Makefile`/`pnpm-workspace.yaml` theme path references to
  match. Requires network access.
- Unless `--noplugins` is passed, it also installs the template's own list
  of plugins (`plugins` in the template catalog) directly from a GitHub
  release asset — for `jcore3`, `jcore-dynamic-archive` and `jcore-turva`,
  always their current latest release (each entry is that repo's
  `/releases/latest/download/<name>.zip` URL, which needs no version to
  keep in sync) — into `wp-content/plugins/<name>`, and adds each to
  `pluginGit` in `jcore.toml` (merged with, not replacing, whatever the
  template's own `defaults.toml` already lists there, e.g. `lohko`) so
  `jcore pull plugins`' remote sync doesn't delete them.
- Initializes a git repository, writes `jcore.toml` with `projectName`
  (and `branch`/`theme`, if set), and commits the initial scaffold (`git
  add -A && git commit`).
- Then generates `.env`, finalizes the project (`site.conf`/`php.ini`
  rendering), and installs dependencies (always, regardless of the
  `install` setting — see `start`) — all after the initial commit, since
  that per-environment output isn't part of the project's own history.

## `start`
Starts the WordPress environment for the current project.
- First runs the same folder/Docker pre-flight checks as `jcore doctor`
  (creating any missing `.jcore`/global folders, checking they're
  writable, and confirming the Docker daemon is reachable) — if either
  fails, it aborts with a clear error instead of letting a raw
  docker/compose error surface later.
- If any JCore project (this one or another) is already running, it warns
  and does nothing unless `--force`/`-f` is passed, in which case every
  other running project is stopped first (most JCore dev setups can only
  run one project at a time, due to shared host ports).
- Runs `docker compose up`.
- If `mode` is `foreground` (default), it stays in the foreground.
- Before starting, it also installs host-side dependencies: a Makefile's
  `install` target if one exists, otherwise npm/pnpm (from `package.json`)
  and Composer (from `composer.json`) packages, then `docker compose pull`
  to refresh images — unless the `install` setting is `false`. Use
  `--install`/`-i` to force this even when `install` is disabled.

## `stop`
Stops every currently running JCore project on the machine (not just the
current one, and not limited to being run from inside a project) — runs
`docker compose stop` for each.

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
- Initializes submodules, then switches the `wp-content/themes/jcore2` theme
  submodule (if present) to the cloned project's own `branch` setting.
- Reloads settings from the freshly cloned project's own config files, then
  generates `.env`, finalizes the project (`site.conf`/`php.ini`
  rendering), and installs dependencies (always, regardless of the
  `install` setting — see `start`).

## `update`
Updates the current project files from the template.
- You can specify specific targets to update.

## `update self`
Updates the JCore CLI binary itself to the latest GitHub release.
- Downloads the release asset matching the current OS/arch, verifies it against
  its detached Ed25519 signature (`<asset>.minisig`), and replaces the running
  executable in place. Aborts without touching the binary if verification fails.
- `--force` / `-f`: reinstall even if already on the latest version, and skip
  the confirmation prompt.
- Every command run does a cheap, non-blocking check for a newer release (at
  most once every 24h): if one is due, a detached background process performs
  it and records the result, so it never adds latency to the command you
  actually ran. If a newer version was found, the *next* invocation prints a
  one-line notice suggesting `jcore update self`.
- Set `JCORE_NO_UPDATE_CHECK=1` to disable this check entirely (e.g. in CI).
- On every successful run (whether it actually updated the binary or found
  you already current), it also (re)installs bash/zsh/fish completions by
  running `completion <shell>` on the resulting binary and writing the output
  to the same paths `make install-completions` uses. This is best-effort and
  never fails the update itself. Note that zsh only picks these up
  automatically if that directory is already on `$fpath` (e.g. via `make
  install-completions`, which the project doesn't set up automatically) —
  bash (with the bash-completion framework) and fish work out of the box.

## `config`
Manages configuration settings. Settings live in one of three TOML files:
global (`~/.config/jcore/config.toml`), project (`<project>/jcore.toml`), or
local (`<project>/.localConfig.toml`, meant for gitignored per-checkout
overrides). Some settings (e.g. `debug`, `mode`, `logLevel`, `template`) are
global-only and can't be set at project scope.

Any of these files can also contain a `[branch-<name>]` table to override
settings only while that git branch is checked out, e.g.:
```toml
remoteHost = "prod.example.com"

[branch-staging]
remoteHost = "staging.example.com"
```
This applies everywhere settings are read — the actual running commands,
`jcore config list`, and `jcore config edit` — not just for display. The
`config set`/`unset` CLI is hand-edit-only for branch tables (it always
targets top-level settings); `jcore config edit` can write into one, per
the rule below.

- `jcore config list [active|global|project|local|defaults|all]`: Lists
  settings. `active` (default) shows the fully merged view, with each
  value annotated with which scope it actually resolves from (`global`,
  `project`, `local`, or `default` if nothing overrides it) — plus
  `@<branch>` appended if that came from a branch override table.
  `global`/`project`/`local` show only what's explicitly set in that one
  scope's file (branch-adjusted), with `@<branch>` annotated on any value
  that specifically comes from that file's own branch table rather than
  its top level. `defaults` shows the project's own `defaults.toml` (the
  per-template file scaffolded into every project, distinct from
  `jcore.toml`) — a real resolution layer between Project and Global scope
  that isn't itself writable via `config set`/the TUI, so this is the only
  way to see it directly; outside a project it just says so. `all` shows
  every one of the above (`defaults` only inside a project) one after
  another.
- `jcore config set <key> <value>`: Sets a configuration value, coerced to
  the setting's real type (bool/int/list), not stored as a raw string. A
  bool setting accepts `true`/`yes`/`on`/`y`/`t`/`1` (case-insensitive) as
  true and anything else as false — it never errors on an unrecognized
  value.
    - Special pseudo-setters:
        - `wpe <name>`: Sets up WP Engine remote settings.
        - `php <version>`: Sets the WordPress PHP image version.
- `jcore config unset <key>`: Removes a configuration setting from the
  targeted scope's file.
- `jcore config edit`: Opens a full-screen interactive editor listing every
  known setting, its current effective value, and which scope (default,
  global, project, or local — with `@<branch>` appended if that value comes
  from a branch override) that value comes from. Settings with a fixed set
  of known values (e.g. `mode`, `pluginInstall`) are edited via an up/down
  select list of those values instead of free text; if the current value
  isn't one of them (a hand-edited or legacy value), it's shown as an extra,
  pre-selected choice so leaving it alone and pressing enter is a no-op.
  Editing (or resetting, `x`) a setting always acts on wherever its value is
  actually coming from:
  if it's already overridden somewhere — even somewhere a fresh value
  wouldn't normally be allowed, e.g. a hand-set project-level override of a
  global-only setting — the edit updates that override in place, since
  writing anywhere else wouldn't change anything (a more specific override
  would still win). A value currently coming from a branch table is edited
  in that branch's table, not the file's top level. Only a setting with no
  existing override anywhere picks a scope by category: Global for
  CLI-behavior settings, Project for everything else (Global if not inside
  a project). `/` to filter, `q` to quit.
- `--global`, `--project`, and `--local` flags specify the scope for
  `set`/`unset`. With none given, it defaults to Project when run inside a
  project, or Global otherwise.

`pluginInstall = "composer"` is deprecated (it breaks the mainWP/wp-cli
workflow) and jcore refuses to run any command other than `config`/
`completion` while it's set — fix it with `jcore config set pluginInstall
remote` (or `local`, or via `config edit`), or pass the global
`--letmebreakthings` flag to proceed anyway.

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
- `jcore create block`: If Lohko isn't installed yet, offers to install it
  (downloaded fresh from GitHub into `wp-content/plugins/lohko`) and lets
  you pick which of its bundled example blocks to keep — any left
  unselected are deleted. Then prompts for a name, a Lohko block template
  (`dynamic` or `static`), and a description, and creates the block at
  `wp-content/plugins/lohko/src/<slug>`.
- `jcore create user`: Prompts to create a new WordPress user in the running environment.

## `status`
Shows which JCore projects are currently running.

## `clean [all|docker]`
- `jcore clean`: Cleans containers and volumes for the current project.
- `jcore clean all`: Cleans all non-running JCore projects and prunes Docker.
- `jcore clean docker`: Prunes Docker containers, images, volumes, and networks.
