import * as process from "process";
import { join, parse } from "path";
import { homedir } from "os";
import { existsSync, writeFileSync } from "fs";
import { config, version } from "../package.json";
import { fetchVersion, loadJsonFile } from "@/utils";
import { logger } from "@/logger";
import { type jcoreData, settingsSchema } from "@/types";
import { configScope, forbiddenSettings, projectSettings } from "@/constants";
import {
  convertGlobalSettings,
  convertProjectSettings,
  projectConfigLegacyFilename,
} from "@/legacy";
import chalk from "chalk";
import { formatValue } from "@/commands/config";

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
  // Make a copy of the current settings object.
  const data = Object.assign({}, jcoreSettingsData);

  // Add global settings to the object.
  getConfig(configScope.GLOBAL, data);
  if (jcoreSettingsData.inProject) {
    // Add project settings if in project.
    getConfig(configScope.PROJECT, data);

    // Add local settings if in project.
    getConfig(configScope.LOCAL, data);
  }
  const result = settingsSchema.safeParse(data);
  if (result.success) {
    // Safe parse the resulting data into a new settings object.
    Object.assign(jcoreSettingsData, result.data);
  } else {
    logger.error("Invalid data in settings file.");
  }
}

export function getConfig(scope: configScope = configScope.GLOBAL, data = {}) {
  switch (scope) {
    case configScope.GLOBAL:
      return Object.assign(data, loadJsonFile(globalConfig));
    case configScope.PROJECT:
      return Object.assign(data, loadJsonFile(join(jcoreSettingsData.path, projectConfigFilename)));
    case configScope.LOCAL:
      return Object.assign(data, loadJsonFile(join(jcoreSettingsData.path, localConfigFilename)));
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

export function updateSetting(
  key: string,
  value: null | string | number | boolean,
  scope: configScope
) {
  if (forbiddenSettings.includes(key)) {
    logger.debug(`Setting ${key} is not allowed!`);
    return false;
  }

  if (!jcoreSettingsData.inProject && scope !== configScope.GLOBAL) {
    if (scope === configScope.PROJECT) {
      logger.error("Project setting, but not in Project!");
    } else {
      logger.error("Not in Project. Use -g for global setting.");
    }
    return false;
  }

  if (scope === configScope.PROJECT && !projectSettings.includes(key)) {
    logger.debug(`Project settings doesn't include ${key}, switching to local`);
    scope = configScope.LOCAL;
  }

  switch (scope) {
    case configScope.GLOBAL:
      if (setConfigValue(key, value, globalConfig)) {
        updateInfo(key, value, scope);
        return true;
      }
      break;
    case configScope.LOCAL:
      if (setConfigValue(key, value, join(jcoreSettingsData.path, localConfigFilename))) {
        updateInfo(key, value, scope);
        return true;
      }
      break;
    case configScope.PROJECT:
      if (setConfigValue(key, value, join(jcoreSettingsData.path, projectConfigFilename))) {
        updateInfo(key, value, scope);
        return true;
      }
      break;
  }
  return false;
}

function updateInfo(key: string, value: null | string | number | boolean, scope: configScope) {
  const scopeText =
    scope === configScope.GLOBAL ? "Global" : scope === configScope.PROJECT ? "Project" : "Local";

  if (value === null) {
    logger.info(`${scopeText} setting ${chalk.green(key)} removed`);
  } else {
    logger.info(`${scopeText} setting ${chalk.green(key)} updated to ${formatValue(value)}`);
  }
}

function setConfigValue(key: string, value: null | string | number | boolean, file: string) {
  try {
    const settings: Record<string, string | number | boolean> = {};
    if (value !== null) {
      settings[key] = value;
      const values = loadJsonFile(file);
      writeFileSync(file, JSON.stringify(Object.assign(values, settings), null, 2));
    } else {
      const values = loadJsonFile(file);
      delete values[key];
      writeFileSync(file, JSON.stringify(values, null, 2));
    }
  } catch (e) {
    logger.error(`Updating ${key} failed.`);
    return false;
  }
  return true;
}
