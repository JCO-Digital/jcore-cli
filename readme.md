# JCORE CLI
This is a helper app for running jcore and other WordPress projects.

Because of the nature of this utility, it runs mostly synchronously. As such it might seem like "bad code", but it helps keep the codebase clean. It's OK to use promises as well, but optimally try to keep functions synchronous if all other things being equal. 

## Code formatting and linting
jcore-cli uses quite strict linting and formatting checking, if you commit something with invalid formatting, the drone build will fail. You can test everything with `npm test`  before you commit, and if formatting needs fixing, run `npm run format`. That fixes everything up.

## Testing the code locally
You can have esbuild continuously build the executable for you with `npm run watch`. But to properly test it, you need to run `source add-to-path.sh`, this will temporarily prepend your build directory to your PATH variable. Note that this only work for the shell you ran the command from, and goes away if you close the shell. Leave the version as it is during development commits.

## Publishing the code
You can commit and push your commits on main branch to your hearts content, the CI will not publish the executable unless you tag the release. You can create a realease by running `npm version patch|minor|major`, depending on what number you want to bump. The project follows Semantic Versioning, so:
 * Major should be used for breaking changes.
 * Minor should be used when new features are added.
 * Patch is used for bugfixes and smaller changes.