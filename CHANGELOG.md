# Changelog

## v3.18.0 (2026-09-02)

#### Features

- config: hide project-only settings from config edit outside a project (7fe3500)

### v3.17.1 (2026-09-02)

#### Bug Fixes

- update: handle redundant v-prefix in version tags (10924e5)

## vv3.17.0 (2026-09-02)

## v3.17.0 (2026-09-02)

#### Features

- run: activate configured WordPress theme automatically (73988c1)
- init: seed project domains during initialization (ecc2a0f)
- project: pre-create bind-mounted folders before container startup (da4c707)
- config: add defaults scope to list command (bbdf75a)
- start: add pre-flight checks before starting environment (b4c048d)
- init: support automated installation of GitHub release plugins (a31907d)
- init: implement interactive project initialization (be79a62)
- cli: Implement project scaffolding and theme/block creation (8f6e656)
- project: add dependency installation and config editor (6c44322)
- ci: implement automated binary releases and self-updates (331e8b2)
- project: add interactive update and improved template deployment (f40c2c1)

#### Bug Fixes

- update: ignore git describe suffix in version comparison (2077828)

#### Refactor

- move Go CLI to repo root, archive TypeScript CLI in legacy-ts/ (5088d8f)

#### Build System

- composer: update jcore/ydin to version 4 (726833a)

### Misc
- Add keywords to package.json (2f343dc)

### v3.16.3 (2026-01-16)

#### Bug Fixes

- Update GitHub repository URLs (5ddf05b)

### v3.16.2 (2026-01-16)

#### Continuous Integration

- Change to OIDC Publish (f47c48c)

### v3.16.1 (2025-11-18)

#### Bug Fixes

- theme path regex to match both single and double quotes in create project. (3e2d2f1)
- Update regex to match both single and double quotes in search patterns (e6e7bf4)

## v3.16.0 (2025-11-05)

#### Features

- Block composer installs in order to not break mainWP / WP-cli workflow. (34af90d)

### v3.15.1 (2025-10-13)

#### Bug Fixes

- path to jcore and bash (fa5eab8)

## v3.15.0 (2025-06-24)

#### Features

- Add pnpm support for updates (36b8464)

### v3.14.3 (2025-06-24)

#### Bug Fixes

- Add pnpm-workspace.yaml file generation (29177b7)

### v3.14.2 (2025-06-16)

#### Bug Fixes

- Only allow block/user creation inside projects (cd0979b)

#### Continuous Integration

- Publish to NPM on CLI build (a9ec0af)
- publish test (9c7ffaa)

### v3.14.1 (2025-06-15)

#### Continuous Integration

- Add publish workflow (f8a0181)

## v3.14.0 (2025-06-15)

#### Features

- Create user with wp-cli after project creation (4228899)
- Add create user command (e9f06c1)

#### Bug Fixes

- Add debug logging to the error handler (484c96e)
- Use /usr/bin/bash in shell command (c836de7)

### v3.13.2 (2025-06-12)

#### Documentation

- Add repository, bugs, and homepage fields to package.json (7ac65a8)

#### Build System

- Add globals dependency to package.json and pnpm-lock.yaml (52b1ef8)
- Update dependencies and ignore files (81486d1)
- Add .npmignore and update package.json for npm publishing (4d137ba)

### v3.13.1 (2025-06-09)

#### Bug Fixes

- missing comma in theme path replacement string (7d7147b)
- Set lohko default block select from template lohko flag. (25b3ce4)

#### Refactor

- error handling to use errorHandler utility (105d235)

## v3.13.0 (2025-06-04)

#### Features

- Improve checksum error handling and update project creation logic (e0cfae3)
- Add interactive selection of Lohko blocks during install (9a5c797)

#### Refactor

- error handling to use errorHandler utility (07813bb)

#### Build System

- Remove @sentry/node and related packages (3294efb)
- Upgrade TypeScript and typescript-eslint to latest versions (855dfb0)

### v3.12.1 (2025-06-04)

### Misc
- Prevent hanging by resuming response stream on error (18444cd)

## v3.12.0 (2025-06-04)

#### Features

- Check for lohko and offer to install it if missing. (0424172)

#### Bug Fixes

- Run composer install only if composer.json exists (7e63703)

### v3.11.3 (2025-05-30)

#### Bug Fixes

- Adjust project and theme creation (382bffd)

### v3.11.2 (2025-05-16)

#### Bug Fixes

- Rename watch target to dev and add makefile replacements (ccd1432)
- Use tabs for JSON indentation (9f3945e)

#### Refactor

- Move file system helpers to fileHelpers.ts (fdd0c8e)
- Add checksum and env file generation (22aa8db)
- Move checksum functions to separate module (bd73675)

### v3.11.1 (2025-05-08)

#### Refactor

- Create theme and block commands (ffbf4dd)

## v3.11.0 (2025-05-08)

#### Refactor

- Use mustache templates for block creation (f0a87c1)

### v3.10.1 (2025-05-08)

#### Bug Fixes

- Use correct block slug in generated files (c32fb27)

#### Refactor

- Refactor theme name sanitization to use slugify util (d6c5693)

#### Build System

- Disallow Javascript files in Typescript compilation (a559321)

### Doc
- Document `create` command in README (0c25e05)

## v3.10.0 (2025-05-07)

#### Features

- Configure copied block template (bb803d3)
- Add command to create Lohko blocks (d8e456a)
- Add file extraction utility. (3ec4fac)

#### Documentation

- Clarified readme and added troubleshooting steps. (533b922)
- Added documentation and tooling for documentation. (d1fd99e)

#### Build System

- Configure eslint with typescript-eslint recommended rules (3e3aee3)

### Doc
- Add GPL-2.0 license file (5b45349)

### v3.9.1 (2025-01-31)

#### Bug Fixes

- config: Clarified some scope issues. (3126f7a)
- add setting for projectDefault and fix settings logic. (3660e77)

## v3.9.0 (2025-01-30)

#### Features

- switch clone extender to github from bitbucket. (913dcd4)

## v3.8.0 (2025-01-14)

#### Bug Fixes

- code cleanup (b2fef4d)

## v3.7.0 (2024-11-21)

#### Features

- Add wpVersion management to settings. (fa55491)

### v3.6.1 (2024-07-29)

### Misc
- Formatting (ade7022)
- Added log message (1212b5a)

## v3.6.0 (2024-07-29)

### Misc
- Use makefile (a775036)

### v3.5.1 (2024-06-07)

### Misc
- Formatting (383817d)
- Project update fixes (403e4de)

## v3.5.0 (2024-06-05)

### Misc
- Branch specific settings. (c948b80)

### v3.4.1 (2024-05-15)

### Misc
- Pnpm configuration (16e232d)

## v3.4.0 (2024-05-15)

#### Refactor

- Switch to pnpm (20f873d)

#### Styles

- formatting (6a356b0)

## v3.3.0 (2024-04-11)

#### Features

- added ability to specify file to import from when pulling database (ed742b9)

#### Maintenance

- Make the base help command and debugging info better. (68541bf)

### v3.2.1 (2024-01-28)

#### Maintenance

- Make the base help command and debugging info better. (7daf272)

## v3.2.0 (2024-01-26)

#### Features

- Added migrate command. (21c1075)
- basic working version of convert command. (2c416b3)

#### Bug Fixes

- Better type check. (2499ca1)

#### Styles

- Biome formatting fixes. (728146d)
- Biome added to project (f5c2ae7)

## v3.1.0 (2023-12-29)

#### Features

- Do not delete .config folder to force checksum validation. (dbb6609)

### v3.0.2 (2023-12-13)

#### Tests

- Check if docker service is running. Implements: #3 (e7bdd1f)

#### Maintenance

- run prettier (f504a02)

### v3.0.1 (2023-12-04)

#### Bug Fixes

- Filenames given as options to update now work. Fixes #2 (7cfb620)

## v3.0.0 (2023-12-03)

#### Bug Fixes

- Bug in package.json substition. Removed "any" reference. (aed23e6)
- Added template and branch to legacy converter. Fixes #1 (85bfaf9)

#### Maintenance

- Some cleanup to make eslint pass without warnings. (13ebc08)
- Default template to jcore2. (6c25501)

### Misc
- Working project creation with templates. (f8b76a1)
- Parser refactor (5cf2042)
- Parser refactor. (ff25c14)

### v2.1.1 (2023-11-15)

### Misc
- Streamlining, cleanup and bugfixes. (f0800af)

## v2.1.0 (2023-11-15)

### Misc
- Prettier (3fd2c97)
- Settings work better with create project (e8771a9)
- Added line number to toml error. (2471afa)
- TOML settings format. (1559459)
- Fixed project converter and hopefully nginx proxy. (3f2a4aa)

### v2.0.2 (2023-11-10)

### Misc
- Prettier (05de9ba)
- Better formatting for replace strings (edf5a62)
- Config list settings management (98b5d55)
- Refactor env variables (28b7d10)
- Changed chalk values. (9b54d5a)
- Added option to scope config list and padded output. (096eaa9)
- Settings framework should have everything except array handler. (fe8deec)

### v2.0.1-beta.1 (2023-11-08)

### Misc
- Changed name of plugins to pluginIstall and made it work. (b3d2c41)

## v2.0.1-beta.0 (2023-11-08)

### Misc
- Separated legacy functions to legacy folder. (612f6d7)
- Code cleanup (941727d)
- Config set works for string, number and boolean. (69c293b)

### v1.0.0-beta.22 (2023-11-07)

### Misc
- Fixed self updater for different executable name. Refactored set to config set (5e4b9a7)

### v1.0.0-beta.21 (2023-11-07)

### Misc
- Clean up imports (b47eb5b)
- Add WRODPRESS_IMAGE varibale to .env (d91a607)
- Added some project cleanup and a more substainable logic for the config conversion. (7ef896a)

### v1.0.0-beta.20 (2023-11-06)

### Misc
- Version ready for testing / demo (d1a1e02)
- WIP config.sh conversion to config.json (d995958)
- WIP Settings refactor (3f5c72d)
- Checksums for files and new structure. (3ae0183)
- Working project-wide checksum (bb41032)

### v1.0.0-beta.19 (2023-10-25)

### Misc
- New release system (19ce0f8)

### v1.0.0-beta.18 (2023-10-24)

### Misc
- Forgot Permissions (9c1c84d)

### v1.0.0-beta.17 (2023-10-24)

### Misc
- New try with action/create-release (e446a71)

### v1.0.0-beta.16 (2023-10-24)

### Misc
- Artifact test (a934f91)

### v1.0.0-beta.15 (2023-10-24)

### v1.0.0-beta.14 (2023-10-24)

### Misc
- Formatting (0b4713d)
- Testing release to github (5b83878)
- Added release to github action (474a719)
- WIP Specify single file to update. (8ed7090)
- Cleaned up packages and steps. (9894cca)
- Changed to GitHubs own runner for public repo. (fa7cbb8)
- Added a basic GitHub build action. (baa23ab)

### v1.0.0-beta.13 (2023-10-16)

#### Refactor

- move the install setting to a global setting to make the setting work. (679e200)

### v1.0.0-beta.12 (2023-08-22)

#### Features

- Added ability to specify branch when initializing a project (584c7df)

#### Refactor

- attach: Added ability to attach to specific container (601cb2c)

### v1.0.0-beta.11 (2023-08-22)

#### Maintenance

- run prettier (5c3b49f)
- remove console.log call (e9ba277)

### v1.0.0-beta.10 (2023-08-22)

#### Features

- Added attach command to attach to logs. :sparkles: (5e58f34)

#### Refactor

- flags: Refactored flags parsing to be able to add arguments to flags. :recycle: (e98327f)

#### Maintenance

- run prettier (5c3a494)

### v1.0.0-beta.9 (2023-06-06)

### Misc
- Fixed non working new version announcement. (02dd127)

### v1.0.0-beta.8 (2023-06-06)

### Misc
- Simple status command working. (c09158e)
- Added status command (56dd83c)
- Added some build and testing instructions to readme. (6093f75)

### v1.0.0-beta.7 (2023-05-23)

#### Bug Fixes

- Fixed issue with running in foreground mode not actually running :bug: (f3d6572)

### v1.0.0-beta.6 (2023-05-22)

### Misc
- Formatting and linting (341f926)
- Slightly better doctor. (2363a58)

### v1.0.0-beta.5 (2023-05-18)

### Misc
- Working docker compose project management and clean command. (1507f36)
- Use docker compose for project listing. (1122667)
- Project tracking code. (73487bd)
- Added Zod and basic data handling. (17aecbb)

### v1.0.0-beta.4 (2023-05-12)

### Misc
- Test commit (47f20b2)
- Added check for running projects, and changed some texts. (56eab74)
- Fixes for set command (e9abd9f)

### v1.0.0-beta.3 (2023-02-13)

### Misc
- Added setting for install on start. (f643e51)

### v1.0.0-beta.2 (2023-02-13)

### Misc
- Major bug fix, plus smaller things. (eda3c21)

### v1.0.0-beta.1 (2023-02-12)

### Misc
- Added verbose mode to help flag. (4bebd9e)

## v1.0.0-beta.0 (2023-02-12)

### Misc
- Beta release. (180d47f)
- Versioning. (cc1e584)
- Prettier (a2baa68)
- Ready for Beta? (4cb0128)
- Added checksum command. (9e2349b)
- Update checksums when changing settings. (de235a8)
- Child update package (63b3053)
- More error handling. (99f238a)
- Code cleanup, and implementation of child command. (148daf6)
- Formatting (9e48436)

### v0.99.24 (2023-02-12)

### Misc
- Build & test scripts. (c1fe06b)
- Code formatting (f5b227b)

### v0.99.23 (2023-02-12)

### Misc
- Self update check. (5726abc)

### v0.99.22 (2023-02-09)

### Misc
- Cleanup (2e7d675)

### v0.99.21 (2023-02-09)

### Misc
- Added patch command (a662213)

### v0.99.20 (2023-02-09)

### Misc
- Forgot to minify the build. (6c038eb)
- Added --no-fund for shorter messages in drone. (e0998df)
- Doctor (Who?) (9b85218)

### v0.99.19 (2023-02-08)

### Misc
- Set tags: target (a08aeff)

### v0.99.18 (2023-02-08)

### Misc
- Prettier (041ebc6)
- Drone to only deploy tags. (85e3774)
- Formatting (c4f5ec5)
- More prettier (472c363)
- More linting. (0ea0f91)
- Linting. (fe0cadb)
- Fixed formatting (b1a7823)
- Reorganized files. (53111f4)
- Fixed some stuff. Added prettier. (deed9c9)
- Added set, and better start/stop commands. (2ea785a)
- Added clone command (18f097a)
- Testing commit script (2571ae1)

### v0.99.9 (2023-01-30)

### Misc
- Mostly working init. (741de3b)

### v0.99.8 (2023-01-29)

### Misc
- Cleanup and init added (985401d)
- jcore pull (d3f92f4)
- Fixes and tweaks (f544a12)
- Update finished. (d71286e)
- Update checksums. (bd1f846)
- Settings and update fixes (64b2eba)
- Trying to figure out the file functions. (8620bce)
- Update fetch (2000ac0)

### v0.99.3 (2023-01-16)

### Misc
- Updater version check (54d8f33)
- Dev version for version tracking. (29562d1)
- Working self updater. (5061454)
- Update WIP (91a7b01)
- File access test. (d9467d1)
- Typo (9a00686)
- Testing adding package for version info. (7242d73)
- Different deploy image (3fc6956)
- Added ignores for node_modules (oops) (150ac7f)
- Wrong command (93fdd95)
- Testing drone deploy (57e1a89)
- Basics are done. (6216870)
- Added help function. (f655dec)
- Parser (d4492df)
- Commands and flags (babe8b6)
- Initial commit (6bf6aa5)

