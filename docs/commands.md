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
  `jcore pull plugins`' remote sync doesn't delete them. If the template's
  catalog entry also sets `lohko = true` (e.g. `jcore3`), this same step
  downloads and installs the Lohko plugin into
  `wp-content/plugins/lohko` (see `jcore create block` below).
- Initializes a git repository, writes `jcore.toml` with `projectName`
  (and `branch`/`theme`, if set), and commits the initial scaffold (`git
add -A && git commit`). Also seeds `localDomain` (defaulting to
  `<slugified-project-name>.localhost`) and `domains` (defaulting to just
  that `localDomain`) if neither is already set — these back the local TLS
  cert and the `wordpress`/`web` containers' hostname(s). Left alone on a
  re-run of `init` in an existing project, so a customized value survives.
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
- Several projects can run at the same time, behind the shared proxy (see
  `proxy`). `start` refuses only if this project would collide with
  another one:
  - another checkout with the same directory name exists elsewhere (both
    would share the same containers and database volume), or
  - a running project already serves one of this project's domains
    (`localDomain` / `domains`).
- Generates `.jcore/compose.proxy.yml`, a compose override that drops the
  project's host ports and puts it behind the proxy, and points compose at
  it with `COMPOSE_FILE` in `.env` (so plain `docker compose` commands in
  the project directory include it too). It also adds a small `loopback`
  service that routes WordPress's requests to its own site (wp-cron, REST,
  multisite subsites) to nginx.
- Starts the proxy if it isn't running. If an older jcore's project still
  holds ports 80/443, it offers to stop that project first.
- Defines `AUTOMATIC_UPDATER_DISABLED` in `.jcore/wordpress/wp-config.php`,
  since wp-cron now really runs locally and a production database brings
  its plugin auto-update settings along.
- Runs `docker compose up`, and prints the site, Adminer
  (`http://adminer.<localDomain>`) and mail (`http://mail.<localDomain>`)
  URLs.
- `--force`/`-f` is deprecated and does nothing.
- If `mode` is `foreground` (default), it stays in the foreground.
- Before starting, it also installs host-side dependencies: a Makefile's
  `install` target if one exists, otherwise npm/pnpm (from `package.json`)
  and Composer (from `composer.json`) packages, then `docker compose pull`
  to refresh images — unless the `install` setting is `false`. Use
  `--install`/`-i` to force this even when `install` is disabled.
- Also parses `docker-compose.yml` itself and creates any project-relative
  bind-mount folder it declares (e.g. `./wp-content`, `./vendor`, or any
  custom one a service adds) that doesn't already exist, before starting
  containers — otherwise Docker creates a missing mount point itself as
  root the moment a container starts, leaving it unwritable by build
  scripts running as a normal user. Named volumes, absolute/`~`-relative
  paths, and anything already present (a folder or a file) are left alone.
- Activates the configured `theme` (`wp theme activate`) once WordPress is
  actually reachable, running concurrently with the containers coming up
  (retrying for up to 2 minutes, since `docker compose up` blocks in the
  foreground until stopped) — a no-op if that theme is already active, so
  it never fights a theme you deliberately switched via wp-admin. If
  WordPress never becomes reachable in time, it warns and leaves you to
  activate it yourself with `jcore run "wp theme activate <slug>"`.

## `stop`

Stops the current project. With `--all`/`-a`, stops every running JCore
project. Run outside a project without `--all`, it asks which running
project to stop. The shared proxy keeps running (see `proxy stop`).

## `restart`

Restarts the current project. It runs every step `start` takes before
bringing containers up (pre-flight checks, `.env` and config generation,
clash checks, the proxy, dependencies), so configuration changes are picked
up, and only then stops the project's containers and runs
`docker compose up` again. If any of those steps fail, the project is left
running as it was. Other running projects are left alone.

Accepts the same `--detached` and `--install`/`-i` flags as `start`, and
like `start` stays in the foreground unless `mode` is `background`. A
foreground `start` in another terminal ends when its containers stop.

## `proxy [start|stop|status|logs]`

Manages the shared proxy (Traefik) that lets several projects run at once.
It owns host ports 80/443 and routes each request to the right project by
domain: `https://<domain>` and `https://*.<domain>` are passed through
(TLS included) to the project's own nginx, and
`http://adminer.<localDomain>` / `http://mail.<localDomain>` to its Adminer
and mail catcher. Plain `http://` site requests are redirected to https.
Its dashboard is at `http://traefik.localhost`.

`start` starts it automatically; it then keeps running, also across
reboots, until `jcore proxy stop`. Its compose file is written to
`~/.config/jcore/proxy/`, and it joins projects over the shared
`jcore-proxy` Docker network.

- `jcore proxy start`: Starts the proxy (offering to stop an older jcore's
  project holding ports 80/443).
- `jcore proxy stop`: Stops it. Projects keep running but are unreachable
  until it's started again.
- `jcore proxy status`: Shows whether it's running.
- `jcore proxy logs`: Follows its logs.

## `attach`

Attaches to the logs of the running containers.

- Runs `docker compose logs -f`.

## `shell`

Opens a bash shell inside the `wordpress` container.

- Runs `docker compose exec wordpress /bin/bash`.

## `run <command>`

Runs a specific command inside the `wordpress` container.

- Example: `jcore run "wp plugin list"`

## `pull [plugins|db|media|themes|all]`

Pulls data from the remote environment to the local environment.

- Defaults to `plugins` and `db` if no target is specified.
- Database, plugin, media, and theme pulls run natively on the host: fetches the database via host SSH and pipes it directly into MariaDB in the `db` container, and syncs plugins, uploads, and themes via host `rsync`.
- When pulling media or themes, interactively prompts with a checklist to select which folders to sync.
- Can take a `--legacy` flag to fall back to the legacy in-container import scripts (`.config/scripts/import*`).
- Can take a `--dbfile <filename>` flag to import a specific SQL file.

## `sync <id|slug>`
Pushes **one** local post to the remote (`remoteHost`/`remotePath`, including
any `[branch-<name>]` override for the current git branch).
- Syncs content, title, excerpt, featured image and page template. The post's
  status is never changed on update.
- The remote post is matched by post type + slug. If none exists, it's created
  as a **draft**. If several match, the command aborts.
- Referenced media is uploaded (`rsync --ignore-existing`, so existing remote
  files are never overwritten) and registered in the remote media library
  (`wp media import --skip-copy`, so URLs stay the same). Attachment IDs in
  blocks and `wp-image-N` classes are remapped, and `localDomain` URLs are
  rewritten to `remoteDomain`. If the remote has a *different* file at the same
  path, the command aborts.
- Always shows a summary and asks for confirmation. If the remote post was
  modified after the local one, or after the last `jcore pull db`, you have to
  type the slug to overwrite it.
- Before an update, the remote post and its meta are saved to
  `.jcore/sync-backups/`. Afterwards the content is read back and compared with
  what was sent.
- Runs entirely inside the `wordpress` container, using the jcore SSH key
  (`~/.config/jcore/ssh`) and wp-cli on the remote. Multisite isn't supported.
- Flags:
  - `--type <post_type>`: narrow a slug lookup.
  - `--lang <slug>`: narrow a slug that several Polylang languages share
    (e.g. `--lang en`).
  - `--dry-run`: show the plan without changing anything.
  - `--meta`: also copy custom post meta (ACF etc.). Prints a warning and asks
    for extra confirmation. IDs in ACF image, file, gallery, post-object,
    relationship and page-link fields are remapped. Other IDs stored in meta
    are **not**.
  - `--as <login>`: remote user to write as (default: the first administrator).
    Writing as a real user keeps WordPress from stripping markup.
  - `--menu <slug|name|id>`: sync a classic navigation menu instead of a post
    (see below).

#### Polylang
When Polylang (free or Pro) is active, it's used through its PHP API with
`wp eval`.
- The remote post is matched by type, slug **and language**. If the local post
  has a language, the command aborts when:
  - Polylang isn't active on the remote,
  - that language isn't set up there, or
  - a remote post with the slug has no language.
- A new post is given the local post's language, and its slug is saved again
  afterwards so Polylang Pro's shared slugs apply. If WordPress still changes
  the slug, you're told.
- The post is linked to the remote versions of its local translations, matched
  by slug and language. The remote's existing translation groups are merged,
  never unlinked. If a translation is already linked to other posts in a
  conflicting way, it's left alone with a warning. Translations that aren't on
  the remote yet are linked once you sync them.
- With Polylang media translation enabled, an existing attachment in the same
  language is preferred, and new attachments get the post's language.
- If the remote's Polylang settings copy data between translations (taxonomies,
  meta, featured image…), the summary warns that linked translations may
  change as well.

#### ACF
- Image, file and gallery values inside ACF block `data` are uploaded and
  remapped like other media. Post-object, relationship and page-link values
  are mapped to the remote post by slug, and by language under Polylang. A
  linked post that's missing on the remote aborts the sync, with the
  `jcore sync <id>` to run first.
- With `--meta`, the same remapping applies to ACF fields in post meta.
- Taxonomy and user fields are copied as-is (a warning lists them).

#### Gravity Forms
- `gravityforms/form` blocks and `[gravityform id="…"]` shortcodes are remapped
  to the remote form with the exact same title. If the form is missing on the
  remote, or several have that title, the sync aborts. Import missing forms
  with Forms → Import/Export first.

### `sync --menu <slug|name|id>`
Pushes **one** classic menu (`register_nav_menus` style) to the remote.
- The remote menu is matched by slug, falling back to name. If there is none,
  it's created.
- Items linking to pages or posts are mapped to the remote by post type + slug,
  and term links by taxonomy + slug. Custom link URLs are rewritten to
  `remoteDomain`. If any linked page or term is missing on the remote, the
  command aborts before changing anything and lists them (for pages, it prints
  the `jcore sync <id>` command to run first).
- Items that match an existing remote item are updated in place, and only the
  fields that differ are changed. New items are added, nested under the right
  parents.
- Remote items that aren't in the local menu are listed in red and removed only
  if you confirm a second prompt (default No).
- Theme locations the local menu uses are assigned on the remote only if
  they're free. A location that already shows another menu is left alone.
- Before an existing menu is changed, it's saved to `.jcore/sync-backups/`,
  along with every remote menu's locations. If a remote item was modified
  after your last `jcore pull db`, you have to type the menu slug to continue.
- With Polylang, linked pages and terms are matched in the same language. Theme
  locations are assigned per language through Polylang's own settings, only
  where the slot is free, and the backup includes Polylang's location map.
- Not synced: custom meta on menu items (e.g. mega-menu or ACF fields; you get
  a warning if there is any). Block-theme `wp_navigation` menus aren't
  supported.
- Works with `--dry-run` and `--as`. `--type`, `--lang` and `--meta` don't
  apply.

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
  actually ran. If a newer version was found, the _next_ invocation prints a
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
  from a branch override) that value comes from. Run outside a project, it
  only lists global-only settings (e.g. `mode`, `logLevel`, `template`) —
  project-eligible settings (`remoteDomain`, `wpDbName`, etc.) are omitted
  entirely, since they wouldn't make sense to write into the global config. Settings with a fixed set
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
- Checks that Docker Compose is 2.24 or newer (needed by the proxy override).
- Reports whether the shared proxy is running, and what holds ports 80/443
  if it isn't.

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

Shows whether the shared proxy is running, and each running JCore project
with its site, Adminer and mail URLs.

## `clean [all|docker]`

- `jcore clean`: Cleans containers and volumes for the current project.
- `jcore clean all`: Cleans all non-running JCore projects and prunes Docker.
- `jcore clean docker`: Prunes Docker containers, images, volumes, and networks.
