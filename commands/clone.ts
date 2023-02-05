import {cmdData} from "@/types";
import {readSettings, settings, writeSettings} from "@/settings";
import {existsSync, mkdirSync} from "fs";
import {logger} from "@/logger";
import {execSync} from "child_process";
import {join, parse} from "path";
import process from "process";
import {childPath, jcorePath} from "@/constants";

export function cloneProject(data: cmdData) {
    let source = data.target[0];
    if (data.target[1]) {
        settings.name = data.target[1];
    }
    if (data.target[0].search(/^[a-zA-Z0-9_-]+$/) !== -1) {
        // Target is just name of the project. Needs to be extended.
        source = 'git@bitbucket.org:jcodigital/' + source + '.git'
        if (!settings.name) {
            // Set argument as project name if second argument not given.
            settings.name = data.target[0];
        }
    } else if (!settings.name) {
        // If full git path and not second argument given, read project name from git path.
        const path = parse(data.target[0]);
        settings.name = path.name;
    }

    // Set project path.
    settings.path = join(process.cwd(), settings.name);
    if (existsSync(settings.path)) {
        logger.warn("Project path exists: " + settings.path);
    } else {
        logger.info("Clone Project: " + settings.name);

        const options = {
            cwd: process.cwd(),
            stdio: [0, 1, 2],
        };

        try {
            execSync('git clone ' + source + ' ' + settings.path, options);
        } catch (reason) {
            logger.error('Clone Failed');
            return;
        }

        // After clone settings need to be read again.
        readSettings()
            .then(setupProject);
    }
}

function setupProject () {
    const options = {
        cwd: settings.path,
        stdio: [0, 1, 2],
    };

    // Initialize submodules.
    try {
        execSync('git submodule update --init', options);
    } catch (reason) {
        logger.error('Submodules Failed');
        return;
    }

    // TODO Find out if this is controversial, but it doesn't commit automatically, so should be fine.
    if (settings.branch) {
        try {
            // Switch jcore submodule to branch.
            options.cwd = join(settings.path, jcorePath);
            if (existsSync(options.cwd)) {
                execSync('git switch ' + settings.branch, options);
            }

            // Switch jcore child submodule to branch.
            options.cwd = join(settings.path, childPath);
            if (existsSync(options.cwd)) {
                execSync('git switch ' + settings.branch, options);
            }
        } catch (reason) {
            logger.error('Branch Switch Failed');
            return;
        }
    }

    logger.info('Clone complete.');
    logger.info('Enter project folder with "cd ' + settings.path + '"');
}
