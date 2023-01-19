import type {cmdData, JcoreSettings} from "@/types";
import { spawnSync } from "child_process";

export default function (cmd: cmdData, settings: JcoreSettings) {
    console.log("Starting Docker");

    const options = {
        cwd: settings.path,
        stdio: [0, 1, 2],
    };

    spawnSync("docker-compose", ["up"], options);
}

export function stop (cmd: cmdData, settings: JcoreSettings) {
    console.log("Stopping Docker");

    const options = {
        cwd: settings.path,
        stdio: [0, 1, 2],
    };

    spawnSync("docker-compose", ["down"], options);
}

export function executeCommand (cmd: cmdData, settings: JcoreSettings) {
    console.log("Executing command on docker");

    const options = {
        cwd: settings.path,
        stdio: [0, 1, 2],
    };

    spawnSync("docker-compose", ["exec", "wordpress", "/bin/bash"], options);
}