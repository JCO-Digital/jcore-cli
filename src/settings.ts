import * as process from "process";
import { join, parse } from "path";
import { homedir } from "os";
import { existsSync, readFileSync, unlinkSync, writeFileSync } from "fs";
import { config, version } from "../package.json";
import { fetchVersion, loadJsonFile } from "@/utils";
import { logger } from "@/logger";
import type { jcoreSettings } from "@/types";
import { dataSchema, jcoreData } from "@/types";

// Default settings.
export const jcoreSettingsData = {
  nodePath: "",
  execPath: "",
  exec: "",
  path: process.cwd(),
  mode: "foreground",
  branch: "",
  theme: "jcore2-child",
  debug: false,
  plugins: "remote",
  install: true,
  logLevel: 2,
  domain: "",
  local: "",
} as jcoreSettings;

export const jcoreDataData = {
  version: version,
  latest: "",
  lastCheck: 0,
} as jcoreData;

const globalConfig = join(homedir(), ".config/jcore/config.json");
const globalData = join(homedir(), ".config/jcore/data.json");
const globalConfigLegacy = join(homedir(), ".config/jcore/config");

export async function readSettings() {
  // Find the project base path.
  while (
    jcoreSettingsData.path.length > 1 &&
    !existsSync(join(jcoreSettingsData.path, "config.json")) &&
    !existsSync(join(jcoreSettingsData.path, "config.sh"))
  ) {
    // Go up one level and try again.
    jcoreSettingsData.path = parse(jcoreSettingsData.path).dir;
  }
  // Check if we are in a project.
  jcoreSettingsData.inProject = jcoreSettingsData.path.length > 1;
  if (jcoreSettingsData.inProject) {
    // Get default name from path.
    jcoreSettingsData.name = parse(jcoreSettingsData.path).base;
  }

  // Read global app data.
  readData();

  // Convert old format to new.
  convertGlobalSettings();

  if (jcoreSettingsData.inProject) {
    // Read project settings if in project.
    convertProjectSettings();
  }

  versionCheck()
    .then(() => {
      logger.debug("Version check done.");
    })
    .catch((reason) => {
      logger.warn(reason);
    });

  if (!jcoreSettingsData.name) {
    // If name is not set, use folder name.
    jcoreSettingsData.name = parse(jcoreSettingsData.path).base;
  }
  config;
  if (!jcoreSettingsData.inProject && !jcoreSettingsData.branch) {
    jcoreSettingsData.branch = config.branch;
  }
}

function readData() {
  const values = loadJsonFile(globalData);
  if (values.latest !== undefined) {
    jcoreDataData.latest = values.latest;
  }
  if (values.lastCheck !== undefined) {
    jcoreDataData.lastCheck = values.lastCheck;
  }
}

function writeData(data = {}) {
  writeFileSync(globalData, JSON.stringify(jcoreDataData, null, 2), "utf8");
}

export function writeGlobalSettings(settings = {}) {
  const values = loadJsonFile(globalConfig);
  writeFileSync(globalConfig, JSON.stringify(Object.assign(values, settings), null, 2), "utf8");
}

export function writeSettings(settings = {}, _global: boolean = false) {
  if (_global) {
    // Call global settings save.
    writeGlobalSettings(settings);
  } else if (jcoreSettingsData.inProject) {
    const localConfig = join(jcoreSettingsData.path, "config.json");

    const values = loadJsonFile(localConfig);
    writeFileSync(localConfig, JSON.stringify(Object.assign(values, settings), null, 2));
  }
}

async function versionCheck() {
  const now = Date.now();
  if (now - jcoreDataData.lastCheck > 60 * 60 * 1000) {
    jcoreDataData.lastCheck = now;
    logger.debug("Doing Version check");
    return fetchVersion().then((newVersion) => {
      jcoreDataData.latest = newVersion;
      writeData();
    });
  }
}

/*
 * Legacy functions.
 */

function convertGlobalSettings() {
  if (existsSync(globalConfigLegacy)) {
    // Read global settings if they exist.
    try {
      const values = new Map() as Map<string, string | string[]>;
      const data = readFileSync(globalConfigLegacy, "utf8");
      parseSettings(values, data);

      writeFileSync(
        globalConfig,
        JSON.stringify(
          {
            mode: values.get("mode"),
            debug: values.get("debug"),
            logLevel: Number(values.get("loglevel")),
            install: values.get("install"),
          },
          null,
          2
        )
      );
      unlinkSync(globalConfigLegacy);
    } catch (e) {
      logger.error("Global settings conversion failed.");
    }
  }
}

function convertProjectSettings() {
  const values = new Map() as Map<string, string | string[]>;

  values.set("name", jcoreSettingsData.name);

  // TODO Fix this to use config.json.
  const data = readFileSync(join(jcoreSettingsData.path, "/config.sh"), "utf8");
  parseSettings(values, data);

  console.log(values);

}

function parseSettings(values: Map<string, string | string[]>, data: string): void {
  // Remove all comments to make matching more straight forward.
  for (const match of data.matchAll(/ *#.*$/gm)) {
    data = data.replace(match[0], "");
  }

  // Look for all BASH variable assignments.
  for (const match of data.matchAll(/^([A-Z_]+)= *([^(].*)$/gm)) {
    // Assign value to map.
    values.set(match[1].toLowerCase(), cleanBashVar(values, match[2]));
  }
  // Look for BASH arrays.
  for (const match of data.matchAll(/^([A-Z_]+)= ?\(\s*([^)]+)\s*\)/gm)) {
    const value = [];
    for (const row of match[2].split("\n")) {
      const text = cleanBashVar(values, row);
      // Don't add empty lines to array.
      if (text) {
        value.push(text);
      }
    }
    // Assign array to map.
    values.set(match[1].toLowerCase(), value);
  }
}

function cleanBashVar(values: Map<string, string | string[]>, text: string): string {
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

function populateSetting(values: Map<string, string | string[]>) {
  for (const [key, value] of values) {
    if (typeof value === "string") {
      switch (key) {
        case "plugin_install":
          jcoreSettingsData.plugins = value;
          break;
        case "debug":
          jcoreSettingsData.debug = value === "1" || value === "true";
          break;
        case "install":
          jcoreSettingsData.install = value === "1" || value === "true";
          break;
        case "path":
          jcoreSettingsData.path = value;
          break;
        case "mode":
          jcoreSettingsData.mode = value;
          break;
        case "name":
          jcoreSettingsData.name = value;
          break;
        case "theme":
          jcoreSettingsData.theme = value;
          break;
        case "branch":
          jcoreSettingsData.branch = value;
          break;
        case "remotehost":
          jcoreSettingsData.remoteHost = value;
          break;
        case "remotepath":
          jcoreSettingsData.remotePath = value;
          break;
      }
    } else {
      if (key === "domains") {
        jcoreSettingsData.domains = [];
        jcoreSettingsData.replace = [];
        value.forEach((domain) => {
          const parts = domain.split(";");
          const upstream = parts[0];
          const local = parts[1] + ".localhost";
          if (!jcoreSettingsData.domains.includes(local)) {
            jcoreSettingsData.domains.push(local);
          }
          jcoreSettingsData.replace.push([upstream, local]);
          if (!jcoreSettingsData.domain) {
            jcoreSettingsData.domain = upstream;
          }
          if (!jcoreSettingsData.local) {
            jcoreSettingsData.local = local;
          }
        });
      } else if (key === "db_exclude") {
        jcoreSettingsData.dbExclude = value;
      } else if (key === "plugin_exclude") {
        jcoreSettingsData.pluginExclude = value;
      } else if (key === "plugin_git") {
        jcoreSettingsData.pluginGit = value;
      }
    }
  }
}

function populateData(data: jcoreData) {
  jcoreDataData.lastCheck = data.lastCheck;
  jcoreDataData.latest = data.latest;
}
