import { readFile } from "fs/promises";
import * as process from "process";
import { join, parse } from "path";
import { homedir } from "os";
import { existsSync, readFileSync, writeFileSync } from "fs";
import { config, version } from "../package.json";
import { fetchVersion } from "@/utils";
import { replaceInFile } from "@/project";
import { logger } from "@/logger";

interface jcoreSettings {
  nodePath: string;
  execPath: string;
  exec: string;
  inProject: boolean;
  path: string;
  mode: string;
  debug: number;
  name: string;
  theme: string;
  branch: string;
  plugins: string;
  logLevel: number;
  domain: string;
  local: string;
  version: string;
  latest: string;
  lastCheck: number;
}

// Default settings.
export const settings = {
  nodePath: "",
  execPath: "",
  exec: "",
  path: process.cwd(),
  mode: "foreground",
  branch: "",
  theme: "jcore2-child",
  debug: 0,
  logLevel: 2,
  domain: "",
  local: "",
  version: version,
  lastCheck: 0
} as jcoreSettings;

const values = new Map() as Map<string, string | string[]>;

const globalConfig = join(homedir(), ".config/jcore/config");

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

  values.set("name", settings.name);
  if (existsSync(globalConfig)) {
    // Read global settings if they exist.
    const data = readFileSync(globalConfig, "utf8");
    parseSettings(data);
  }

  if (settings.inProject) {
    // Read project settings if in project.
    const data = readFileSync(join(settings.path, "/config.sh"), "utf8");
    parseSettings(data);
  }

  populateSetting();

  versionCheck().then(() => {
    logger.debug("Version check done.");
  }).catch(reason => {
    logger.warn(reason);
  });

  if (!settings.name) {
    // If name is not set, use folder name.
    settings.name = parse(settings.path).base;
  }

  if (!settings.inProject && !settings.branch) {
    settings.branch = config.branch;
  }
}

async function versionCheck() {
  const now = Date.now();
  if (now - settings.lastCheck > 60 * 60 * 1000) {
    settings.lastCheck = now;
    console.debug("Doing Version check");
    return fetchVersion().then((newVersion) => {
      settings.latest = newVersion;
      writeGlobalSettings();
    });
  }
}

export function writeGlobalSettings() {
  const setValues = [
    { key: "mode", value: settings.mode },
    { key: "debug", value: settings.debug.toString() },
    { key: "loglevel", value: settings.logLevel.toString() },
    { key: "latest", value: settings.latest },
    { key: "last_check", value: settings.lastCheck }
  ];
  let data = "";
  for (const row of setValues) {
    data += row.key.toUpperCase() + "=" + row.value + "\n";
  }
  writeFileSync(globalConfig, data, "utf8");
}

export function writeSettings() {
  // Call global settings save.
  writeGlobalSettings();
  if (settings.inProject) {
    // Save project settings.
    const setValues = [
      { key: "name", value: settings.name },
      { key: "theme", value: settings.theme }
    ];
    let replace = [];
    for (const row of setValues) {
      const key = row.key.toUpperCase();
      replace.push({
        search: new RegExp(`^#?${key}="[^"]*" *$`, "m"),
        replace: `${key}="${row.value}"`
      });
    }

    replaceInFile(join(settings.path, "config.sh"), replace);
  }
}

function parseSettings(data: string): void {
  // Remove all comments to make matching more straight forward.
  for (const match of data.matchAll(/ *#.*$/gm)) {
    data = data.replace(match[0], "");
  }

  // Look for all BASH variable assignments.
  for (const match of data.matchAll(/^([A-Z_]+)= *([^(].*)$/gm)) {
    // Assign value to map.
    values.set(match[1].toLowerCase(), cleanBashVar(match[2]));
  }
  // Look for BASH arrays.
  for (const match of data.matchAll(/^([A-Z_]+)= ?\(\s*([^)]+)\s*\)/gm)) {
    const value = [];
    for (const row of match[2].split("\n")) {
      const text = cleanBashVar(row);
      // Don't add empty lines to array.
      if (text) {
        value.push(text);
      }
    }
    // Assign array to map.
    values.set(match[1].toLowerCase(), value);
  }
}

function cleanBashVar(text: string): string {
  // Remove wrapping double quotes.
  let value = text.replace(/^["' ]+|["' ]+$/gm, "");

  // Look for all references to BASH variables.
  for (const varMatch of value.matchAll(/\$([A-Z_]+)/gm)) {
    const key = varMatch[1].toLowerCase();
    if (values.has(key)) {
      // If variable exists in map, substitute variable for value.
      const str = values.get(key);
      if (typeof str === "string") {
        value = value.replace(varMatch[0], str);
      }
    }
  }
  return value;
}

function populateSetting() {
  for (const [key, value] of values) {
    if (typeof value === "string") {
      switch (key) {
        case "path":
          settings.path = value;
          break;
        case "mode":
          settings.mode = value;
          break;
        case "debug":
          settings.debug = Number(value);
          break;
        case "name":
          settings.name = value;
          break;
        case "theme":
          settings.theme = value;
          break;
        case "branch":
          settings.branch = value;
          break;
        case "plugin_install":
          settings.plugins = value;
          break;
        case "latest":
          settings.latest = value;
          break;
        case "last_check":
          settings.lastCheck = Number(value);
          break;
      }
    } else {
      if (key === "domains") {
        const parts = value[0].split(";");
        settings.domain = parts[0];
        settings.local = parts[1] + ".localhost";
      }
    }
  }
}
