import type {cmdData} from "@/types";
import { spawnSync } from "child_process";
import {join} from "path";
import {settings} from "@/settings";
import {logger} from "@/logger";

export function pull (data: cmdData) {

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

export function executeCommand (data: cmdData) {
    logger.verbose("Executing command on docker");

    const options = {
        cwd: settings.path,
        stdio: [0, 1, 2],
    };

    spawnSync("docker-compose", ["exec", "wordpress", "/bin/bash"], options);
}