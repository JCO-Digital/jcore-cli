import * as process from "process";
import { join, parse } from "path";
import { homedir } from "os";
import { existsSync, readFileSync, writeFileSync } from "fs";
import { config, version } from "../package.json";
import { fetchVersion, loadJsonFile } from "@/utils";
import { logger } from "@/logger";
import { type jcoreData, settingsSchema } from "@/types";

// Default settings.
export const jcoreSettingsData = settingsSchema.parse({
  path: process.cwd(),
  mode: "foreground",
  theme: "jcore2-child",
});

export const jcoreDataData = {
  version: version,
  latest: "",
  lastCheck: 0,
} as jcoreData;

const projectConfigFilename = "jcore.json";
const projectConfigLegacyFilename = "config.sh";
const globalConfig = join(homedir(), ".config/jcore/config.json");
const globalData = join(homedir(), ".config/jcore/data.json");
const globalConfigLegacy = join(homedir(), ".config/jcore/config");

export async function readSettings() {
  // Find the project base path.
  while (
    jcoreSettingsData.path.length > 1 &&
    !existsSync(join(jcoreSettingsData.path, projectConfigFilename)) &&
    !existsSync(join(jcoreSettingsData.path, projectConfigLegacyFilename))
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
  // Read global settings.
  readProjectSettings();

  if (jcoreSettingsData.inProject) {
    // Convert project settings if in project.
    convertProjectSettings();

    if (!jcoreSettingsData.branch) {
      // Set default branch if not set.
      jcoreSettingsData.branch = config.branch;
    }
  }

  versionCheck()
    .then(() => {
      logger.debug("Version check done.");
    })
    .catch((reason) => {
      logger.warn(reason);
    });
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

function readProjectSettings() {
  const localConfig = join(jcoreSettingsData.path, projectConfigFilename);
  // Make a copy of the current settings object.
  const data = Object.assign({}, jcoreSettingsData);

  // Add global settings to the object.
  const globalValues = loadJsonFile(globalConfig);
  Object.assign(data, globalValues);
  if (jcoreSettingsData.inProject) {
    // Add local settings if in project.
    const localValues = loadJsonFile(localConfig);
    Object.assign(data, localValues);
  }
  const result = settingsSchema.safeParse(data);
  if (result.success) {
    // Safe parse the resulting data into a new settings object.
    Object.assign(jcoreSettingsData, result.data);
  } else {
    console.error(result.error);
    logger.error("Invalid data in settings file.");
  }
}

function writeData() {
  writeFileSync(globalData, JSON.stringify(jcoreDataData, null, 2), "utf8");
}

export function writeGlobalSettings(settings = {}) {
  const values = loadJsonFile(globalConfig);
  writeFileSync(globalConfig, JSON.stringify(Object.assign(values, settings), null, 2), "utf8");
}

export function writeSettings(settings = {}, _global = false) {
  if (_global) {
    // Call global settings save.
    writeGlobalSettings(settings);
  } else if (jcoreSettingsData.inProject) {
    const localConfig = join(jcoreSettingsData.path, projectConfigFilename);

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
  if (!existsSync(globalConfig) && existsSync(globalConfigLegacy)) {
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
    } catch (e) {
      logger.error("Global settings conversion failed.");
    }
  }
}

function convertProjectSettings() {
  const localConfig = join(jcoreSettingsData.path, projectConfigFilename);
  const localConfigLegacy = join(jcoreSettingsData.path, projectConfigLegacyFilename);
  if (!existsSync(localConfig) && existsSync(localConfigLegacy)) {
    try {
      const values = new Map() as Map<string, string | string[]>;

      values.set("name", jcoreSettingsData.name);

      // Read and parse config.sh.
      const data = readFileSync(localConfigLegacy, "utf8");
      parseSettings(values, data);

      let newDomain = "";
      let newLocal = "";
      const domains: string[] = [];
      const replace = [];
      const domainsValue = values.get("domains");
      if (domainsValue instanceof Array) {
        for (const domain of domainsValue) {
          const parts = domain.split(";");
          const local = `${parts[1]}.localhost`;
          replace.push(["//" + parts[0], "//" + local]);
          if (!domains.includes(local)) {
            domains.push(local);
          }
          if (newDomain === "") {
            newDomain = parts[0];
            newLocal = local;
          }
        }
      }

      const newValues = {
        name: values.get("name"),
        theme: values.get("theme"),
        remoteHost: values.get("remotehost"),
        remotePath: values.get("remotepath"),
        replace,
        domains,
        domain: newDomain,
        local: newLocal,
        dbExclude: values.get("db_exclude"),
        pluginExclude: values.get("plugin_exclude"),
        pluginGit: values.get("plugin_git"),
        pluginInstall: values.get("plugin_install"),
        install: values.get("install") === "true",
      };
      const localConfig = join(jcoreSettingsData.path, projectConfigFilename);
      const config = loadJsonFile(localConfig);

      writeFileSync(
        localConfig,
        JSON.stringify(Object.assign(newValues, config), null, 2),
        "utf-8"
      );
    } catch (e) {
      logger.error("Error convertion project settings.");
    }
  }
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
