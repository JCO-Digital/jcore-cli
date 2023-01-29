import {readFile} from 'fs/promises';
import * as process from "process";
import type {JcoreSettings} from "@/types";
import {join, parse} from "path";
import {homedir} from "os";
import {existsSync} from 'fs';
import {config} from '@/package.json';

// Default settings.
export const settings = {
    nodePath: "",
    execPath: "",
    exec: "",
    path: process.cwd(),
    mode: 'foreground',
    branch: config.branch,
    theme: 'jcore2-child',
    debug: 0,
    logLevel: 2,
} as JcoreSettings;

const globalConfig = join(homedir(), '.config/jcore/config');

export async function readSettings() {
    // Find the project base path.
    while (settings.path.length > 1 && !existsSync(join(settings.path, "config.sh"))) {
        // Go up one level and try again.
        settings.path = parse(settings.path).dir;
    }
    // Check if we are in a project.
    settings.inProject = settings.path.length > 1;
    if (settings.inProject) {
        settings.name = parse(settings.path).base;
    }

    let values = new Map();
    values.set('name', settings.name);
    if (existsSync(globalConfig)) {
        // Read global settings if they exist.
        await readFile(globalConfig, 'utf8').then(data => {
            values = parseSettings(data, values);
        });
    }

    if (settings.inProject) {
        // Read project settings if in project.
        await readFile(join(settings.path, '/config.sh'), 'utf8').then(data => {
            values = parseSettings(data, values);
        });
    }

    for (let [key, value] of values) {
        setSetting(key, value);
    }


    if (!settings.name) {
        // If name is not set, use folder name.
        settings.name = parse(settings.path).base;
    }
}

export function writeSettings() {

}

function parseSettings(data: string, values: Map<string, string>): Map<string, string> {
    // Remove all comments to make matching more straight forward.
    for (let match of data.matchAll(/ *#.*$/gm)) {
        data = data.replace(match[0], '');
    }

    // Look for all BASH variable assignments. TODO handle BASH arrays.
    for (let match of data.matchAll(/^([A-Z_]+)=(.+)$/gm)) {
        // Remove wrapping double quotes.
        let value = match[2].replace(/^[" ]+|[" ]+$/gm, '');
        // Look for all references to BASH variables.
        for (let varMatch of value.matchAll(/\$([A-Z_]+)/gm)) {
            const key = varMatch[1].toLowerCase();
            if (values.has(key)) {
                // If variable exists in map, substitute variable for value.
                const str = values.get(key);
                if (str) {
                    value = value.replace(varMatch[0], str);
                }
            }
        }
        // Assign value to map.
        values.set(match[1].toLowerCase(), value);
    }
    return values;
}

function setSetting(key: string, value: string) {
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
        case 'name':
            settings.name = value;
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