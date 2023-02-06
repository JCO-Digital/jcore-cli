import {cmdData} from "@/types";
import {finaliseProject} from "@/project";
import {execSync, spawnSync} from "child_process";
import {settings} from "@/settings";
import {logger} from "@/logger";

export function start (data: cmdData) {
    finaliseProject();

    const options = {
        cwd: settings.path,
        stdio: [0, 1, 2],
    };
    if (settings.mode === 'foreground') {
        spawnSync("docker-compose", ["up"], options);
    } else {
        execSync('docker-compose up -d', options);
    }
}

export function stop (data: cmdData) {
    logger.info("Stopping Docker");

    const options = {
        cwd: settings.path,
        stdio: [0, 1, 2],
    };

    execSync("docker-compose down", options);
}
