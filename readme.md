# JCORE CLI
This is a helper app for running jcore and other WordPress projects.

Because of the nature of this utility, it runs mostly synchronously. As such it might seem like "bad code", but it helps keep the codebase clean. It's OK to use promises as well, but optimally try to keep functions synchronous if all other things being equal. 

## Code formatting and linting
jcore-cli uses quite strict linting and formatting checking, if you commit something with invalid formatting, the drone build will fail. You can test everything with `npm test`  before you commit, and if formatting needs fixing, run `npm run prettier`. That fixes everything up.

## Testing the code locally
You can have esbuild continuously build the executable for you with `npm run watch`. But to properly test it, you need to run `source add-to-path.sh`, this will temporarily prepend your build directory to your PATH variable. Note that this only work for the shell you ran the command from, and goes away if you close the shell. Leave the version as it is during development commits.

## Publishing the code
You can commit and push your commits on main branch to your hearts content, drone will not publish the executable unless you tag the release. Drone will still build, but this is only done to test if the build passes. While it is in beta, there's a script to tag the release called `beta-release.sh`. It just runs the command `npm version prerelease --preid=beta` but it's easier to remember.
I will maybe make a script for non beta releases as well, or just list the commands here.