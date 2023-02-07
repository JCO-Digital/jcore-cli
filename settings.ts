import { readFile } from "fs/promises";
import * as process from "process";
import type { JcoreSettings } from "@/types";
import { join, parse } from "path";
import { homedir } from "os";
import { existsSync, writeFileSync } from "fs";
import { config } from "@/package.json";

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
} as JcoreSettings;

const values = new Map() as Map<string, string | string[]>;

const globalConfig = join(homedir(), ".config/jcore/config");

export async function readSettings() {
  // Find the project base path.
  while (
    settings.path.length > 1 &&
    !existsSync(join(settings.path, "config.sh"))
  ) {
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
    await readFile(globalConfig, "utf8").then((data) => {
      parseSettings(data);
    });
  }

  if (settings.inProject) {
    // Read project settings if in project.
    await readFile(join(settings.path, "/config.sh"), "utf8").then((data) => {
      parseSettings(data);
    });
  }

  populateSetting();

  if (!settings.name) {
    // If name is not set, use folder name.
    settings.name = parse(settings.path).base;
  }

  if (!settings.inProject && !settings.branch) {
    settings.branch = config.branch;
  }
}

export function writeGlobalSettings() {
  const setValues = [
    { key: "mode", value: settings.mode },
    { key: "debug", value: settings.debug.toString() },
  ];
  let data = "";
  for (let row of setValues) {
    data += row.key.toUpperCase() + "=" + row.value + "\n";
  }
  writeFileSync(globalConfig, data, "utf8");
}

export function writeSettings() {}

function parseSettings(data: string): void {
  // Remove all comments to make matching more straight forward.
  for (let match of data.matchAll(/ *#.*$/gm)) {
    data = data.replace(match[0], "");
  }

  // Look for all BASH variable assignments.
  for (let match of data.matchAll(/^([A-Z_]+)= *([^(].*)$/gm)) {
    // Assign value to map.
    values.set(match[1].toLowerCase(), cleanBashVar(match[2]));
  }
  // Look for BASH arrays.
  for (let match of data.matchAll(/^([A-Z_]+)= ?\(\s*([^)]+)\s*\)/gm)) {
    const value = [];
    for (let row of match[2].split("\n")) {
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
  for (let varMatch of value.matchAll(/\$([A-Z_]+)/gm)) {
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
  for (let [key, value] of values) {
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
