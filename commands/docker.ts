import type {cmdData, JcoreSettings} from "@/types";
import { spawnSync } from "child_process";
import {echo} from "@/utils";
import {join} from "path";
import {flags} from "@/constants";

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

export function pull (data: cmdData, settings: JcoreSettings) {
    echo(data);
    echo(settings)

    const options = {
        cwd: settings.path,
        stdio: [0, 1, 2],
    };

    if (data.flags.includes('plugins') || data.flags.includes('all')) {
        spawnSync("docker-compose", ["exec", "wordpress", join("/project/.config/scripts",'importplugins')], options);
    }
    if (data.flags.includes('db') || data.flags.includes('all')) {
        spawnSync("docker-compose", ["exec", "wordpress", join("/project/.config/scripts",'importdb')], options);
    }
    if (data.flags.includes('plugins') || data.flags.includes('db') || data.flags.includes('all')) {
        spawnSync("docker-compose", ["exec", "wordpress", join("/project/.config/scripts",'installplugins')], options);
    }
    if (data.flags.includes('media') || data.flags.includes('all')) {
        spawnSync("docker-compose", ["exec", "wordpress", join("/project/.config/scripts",'importmedia')], options);
    }
}

export function executeCommand (cmd: cmdData, settings: JcoreSettings) {
    console.log("Executing command on docker");

    const options = {
        cwd: settings.path,
        stdio: [0, 1, 2],
    };

    spawnSync("docker-compose", ["exec", "wordpress", "/bin/bash"], options);
}