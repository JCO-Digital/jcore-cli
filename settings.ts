import {readFile, access} from 'fs/promises';
import * as process from "process";
import type {JcoreSettings} from "@/types";

const homedir = require('os').homedir();

export async function readSettings(): Promise<JcoreSettings> {
    const globalConfig = homedir + '/.config/jcore/config';

    // Default settings.
    const settings = {
        path: process.cwd(),
        mode: 'foreground',
        debug: 0
    } as JcoreSettings;

    // Find the project base path.
    while (settings.path.length && !await fileExists(settings.path, "config.sh")) {
        settings.path = settings.path.replace(/\/?[^/]+$/gm, '');
    }

    await readFile(globalConfig, 'utf8').then(data => {
        for (let [key,value] of parseSettings(data)) {
            setSetting(settings,key,value);
        }
    });

    await readFile(settings.path + '/config.sh', 'utf8').then(data => {
        for (let [key,value] of parseSettings(data)) {
            setSetting(settings,key,value);
        }
    });

    return settings;
}

function parseSettings(data: string): Map<string, string> {
    // Create new HashMap for values.
    const values = new Map();
    // Look for all BASH variable assignments. TODO handle BASH arrays.
    for (let match of data.matchAll(/^([A-Z_]+)=(.+)$/gm)) {
        // Remove wrapping " characters.
        let value = match[2].replace(/^"|"$/gm, '');
        // Look for all references to BASH variables.
        for (let varMatch of value.matchAll(/\$([A-Z_]+)/gm)) {
            const key = varMatch[1].toLowerCase();
            if (values.has(key)) {
                // If variable exists in map, substitute variable for value.
                value = value.replace(varMatch[0],values.get(key));
            }
        }
        // Assign value to map.
        values.set(match[1].toLowerCase(), value);
    }
    return values;
}

/*
 * Simple wrapper that returns boolean promise whether a file exists of not.
 */
async function fileExists(path: string, file: string): Promise<boolean> {
    try {
        await access(path.replace(/\/+$/gm, '') + '/' + file);
        return true;
    } catch {
        return false;
    }
}

function setSetting (settings: JcoreSettings, key: string, value: string) {
    switch (key) {
        case 'path':
            settings.path = value;
            break;
        case 'mode':
            settings.mode = value;
            break;
        case 'debug':
            settings.debug = Number(value);
            break;
        case 'theme':
            settings.theme = value;
            break;
        case 'branch':
            settings.branch = value;
            break;
        case 'plugin_install':
            settings.plugins = value;
            break;
    }
}