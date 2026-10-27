# jcore-cli

Go CLI for JCore WordPress development environments. The archived
TypeScript CLI lives in `legacy-ts/`.

## Git workflow

- Work on a feature branch and open a pull request; don't commit straight
  to `main` or `beta`.
- Use conventional commit subjects with a scope, e.g. `fix(clean): …`,
  `feat(install): …`, `refactor(container): …`.

### Releases are automatic

`.github/workflows/release.yml` runs foonver on every push:

- a push to `main` cuts a release;
- a push to `beta` cuts a prerelease (`vX.Y.Z-beta.N`).

So merging a pull request publishes it. foonver picks the version number.

### Which branch to target

- **One contained fix or feature:** branch from `main`, PR against `main`.
- **A bigger sprint with many pull requests:** branch from `beta`, PR
  against `beta`. The sprint ships as betas of the next minor version
  (e.g. `v4.5.0-beta.N` after `v4.4.1`), unless we know up front that it
  contains a breaking change, in which case it targets the next major.
  Agree on that before starting. After the first merge, check that the
  beta tag foonver created is the intended version.
- When the sprint is done, release it by merging `beta` into `main`.

### Before starting a sprint on `beta`

`beta` must be rebased on `main` first. Check it:

```sh
git fetch origin
git rev-list --count origin/beta..origin/main   # commits beta is missing from main
git cherry -v origin/main origin/beta           # beta-only (unreleased) commits
```

- **No beta-only commits:** bring `beta` up to `main` (a fast-forward:
  `git push origin origin/main:beta`), then start.
- **Beta-only commits (an unreleased beta):** stop and decide manually with
  the team. Release them to `main` first, carry them into the sprint by
  rebasing `beta` on `main`, or drop them. Never decide this alone.
  Rebasing or resetting `beta` rewrites a shared branch and needs a force
  push, so it always needs explicit confirmation.
