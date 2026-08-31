# JCore CLI Configuration

JCore CLI uses TOML files for configuration. Settings are merged from several locations in a specific order of priority.

## Configuration Files

- **Default**: `jcore.default.toml` (in the project directory).
- **Global**: `~/.config/jcore/config.toml`.
- **Project**: `jcore.toml` (in the project directory).
- **Local**: `jcore.local.toml` (in the project directory, usually ignored by git).

## Settings Reference

| Key | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `branch` | string | `""` | The default git branch for the project or submodules. |
| `template` | string | `"jcore3"` | The project template to use. |
| `dbExclude` | array | `[]` | Tables to exclude from database imports/exports. |
| `dbPrefix` | string | `"wp_"` | WordPress database table prefix. |
| `debug` | boolean | `false` | Enable CLI debug logging. |
| `domains` | array | `[]` | List of domains for the project. |
| `install` | boolean | `true` | Whether to run installation/setup steps on start. |
| `localDomain` | string | `""` | The domain used for local development. |
| `logLevel` | number | `2` | Logging verbosity (0: error, 1: warn, 2: info, 3: verbose, 4: debug). |
| `mode` | string | `"foreground"` | Docker run mode (`foreground` or `background`). |
| `pluginExclude` | array | `[]` | Plugins to exclude from syncing. |
| `pluginGit` | array | `[]` | Plugins to be managed via Git submodules. |
| `pluginInstall` | string | `"remote"` | Plugin installation mode (`remote` or `local`). |
| `pluginLocal` | array | `[]` | Plugins to be kept only locally. |
| `projectDefault` | string | `"git@github.com:JCO-Digital/{name}.git"` | Default Git URL pattern for cloning. |
| `projectName` | string | `""` | Name of the project. |
| `remoteDomain` | string | `""` | The domain of the remote/production site. |
| `remoteHost` | string | `""` | SSH host for the remote environment. |
| `remotePath` | string | `""` | File path on the remote host. |
| `replace` | array | `[]` | String replacements to perform during database import. |
| `theme` | string | `""` | The name of the active WordPress theme. |
| `wpDebug` | boolean | `true` | Enable `WP_DEBUG` in WordPress. |
| `wpDebugDisplay` | boolean | `true` | Enable `WP_DEBUG_DISPLAY` in WordPress. |
| `wpDebugLog` | boolean | `false` | Enable `WP_DEBUG_LOG` in WordPress. |
| `wpImage` | string | `"jcodigi/wordpress:latest"` | Docker image to use for WordPress. |
| `wpVersion` | string | `"latest"` | WordPress version to install. |

## Branch-Specific Settings

You can define settings that only apply to a specific Git branch by using a TOML table with the prefix `branch-`.

Example:
```toml
projectName = "my-project"

[branch-develop]
wpImage = "jcodigi/wordpress:php8.1"
```
When on the `develop` branch, `wpImage` will be overridden.
