import * as process from "process";
import { join, parse } from "path";
import { homedir } from "os";
import { existsSync, writeFileSync } from "fs";
import { config, version } from "../package.json";
import { fetchVersion, loadJsonFile } from "@/utils";
import { logger } from "@/logger";
import { type jcoreData, settingsSchema } from "@/types";
import { forbiddenSettings, projectSettings } from "@/constants";
import {
  convertGlobalSettings,
  convertProjectSettings,
  projectConfigLegacyFilename,
} from "@/legacy";

// Default settings.
export const jcoreSettingsData = settingsSchema.parse({
  path: process.cwd(),
  mode: "foreground",
  theme: "jcore2-child",
  wpImage: "jcodigital/wordpress:latest",
});

export const jcoreDataData = {
  version: version,
  latest: "",
  lastCheck: 0,
} as jcoreData;

const projectConfigFilename = "jcore.json";
const localConfigFilename = ".localConfig.json";
const globalConfig = join(homedir(), ".config/jcore/config.json");
const globalData = join(homedir(), ".config/jcore/data.json");

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
  convertGlobalSettings(globalConfig);
  // Read global settings.
  readProjectSettings();

  if (jcoreSettingsData.inProject) {
    // Convert project settings if in project.
    convertProjectSettings(projectConfigFilename);

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
  const projectConfig = join(jcoreSettingsData.path, projectConfigFilename);
  const localConfig = join(jcoreSettingsData.path, localConfigFilename);
  // Make a copy of the current settings object.
  const data = Object.assign({}, jcoreSettingsData);

  // Add global settings to the object.
  const globalValues = loadJsonFile(globalConfig);
  Object.assign(data, globalValues);
  if (jcoreSettingsData.inProject) {
    // Add local settings if in project.
    const localValues = loadJsonFile(localConfig);
    Object.assign(data, localValues);

    // Add project settings if in project.
    const projectValues = loadJsonFile(projectConfig);
    Object.assign(data, projectValues);
  }
  const result = settingsSchema.safeParse(data);
  if (result.success) {
    // Safe parse the resulting data into a new settings object.
    Object.assign(jcoreSettingsData, result.data);
  } else {
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

export function updateSetting(key: string, value: string | number | boolean, _global = false) {
  if (forbiddenSettings.includes(key)) {
    logger.debug(`Setting ${key} is not allowed!`);
    return false;
  }
  const projectConfig = join(jcoreSettingsData.path, projectConfigFilename);
  const localConfig = join(jcoreSettingsData.path, localConfigFilename);

  if (projectSettings.includes(key)) {
    if (!jcoreSettingsData.inProject) {
      logger.error("Project setting, but not in Project!");
      return false;
    }
    if (setConfigValue(key, value, projectConfig)) {
      logger.info(`Project setting ${key} updated to ${value}`);
    }
  } else if (_global) {
    if (setConfigValue(key, value, globalConfig)) {
      logger.info(`Global setting ${key} updated to ${value}`);
    }
  } else {
    if (!jcoreSettingsData.inProject) {
      logger.error("Not in Project. Use -g for global setting.");
      return false;
    }
    if (setConfigValue(key, value, localConfig)) {
      logger.info(`Local setting ${key} updated to ${value}`);
    }
  }
  return true;
}

function setConfigValue(key: string, value: string | number | boolean, file: string) {
  try {
    const settings: Record<string, string | number | boolean> = {};
    settings[key] = value;
    const values = loadJsonFile(file);
    writeFileSync(file, JSON.stringify(Object.assign(values, settings), null, 2));
  } catch (e) {
    logger.error(`Updating ${key} failed.`);
    return false;
  }
  return true;
}
