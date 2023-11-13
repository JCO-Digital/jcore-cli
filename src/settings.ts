import * as process from "process";
import { join, parse } from "path";
import { parse as tomlParse, stringify as tomlStringify, TomlError } from "smol-toml";
import { homedir } from "os";
import { existsSync, readFileSync, writeFileSync, unlinkSync } from "fs";
import { config, version } from "../package.json";
import { fetchVersion, loadJsonFile } from "@/utils";
import { logger } from "@/logger";
import { configValue, type jcoreData, runtimeSchema, settingsSchema } from "@/types";
import { configScope, projectSettings } from "@/constants";
import { convertGlobalSettings, convertProjectSettings, projectConfigLegacyFilename } from "@/legacy";
import chalk from "chalk";
import { formatValue } from "@/commands/config";
import { ZodError } from "zod";

// Runtime settings.
export const jcoreRuntimeData = runtimeSchema.parse({
  workDir: process.cwd()
});

// Default settings.
export const jcoreSettingsData = settingsSchema.parse({
  mode: "foreground",
  theme: "jcore2-child",
  wpImage: "jcodigi/wordpress:latest"
});

export const jcoreDataData = {
  version: version,
  latest: "",
  lastCheck: 0
} as jcoreData;

const projectConfigFilename = "jcore.toml";
const localConfigFilename = ".localConfig.toml";
const defaultConfigFilename = "defaults.toml";
const globalConfig = join(homedir(), ".config/jcore/config.toml");
const globalData = join(homedir(), ".config/jcore/data.json");

export async function readSettings() {
  // Find the project base path.
  while (
    jcoreRuntimeData.workDir.length > 1 &&
    !existsSync(join(jcoreRuntimeData.workDir, projectConfigFilename)) &&
    !existsSync(join(jcoreRuntimeData.workDir, projectConfigLegacyFilename))
    ) {
    // Go up one level and try again.
    jcoreRuntimeData.workDir = parse(jcoreRuntimeData.workDir).dir;
  }
  // Check if we are in a project.
  jcoreRuntimeData.inProject = jcoreRuntimeData.workDir.length > 1;
  if (jcoreRuntimeData.inProject) {
    // Get default name from path.
    jcoreSettingsData.projectName = parse(jcoreRuntimeData.workDir).base;
  }

  // Read global app data.
  readData();

  // Convert old format to new.
  convertGlobalSettings(globalConfig);
  // Read global settings.
  readProjectSettings();

  if (jcoreRuntimeData.inProject) {
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

function readProjectSettings() {
  // Make a copy of the current settings object.
  const data = Object.assign({}, jcoreSettingsData);

  // Add default settings.
  getConfig(configScope.DEFAULT, data);

  // Add global settings to the object.
  getConfig(configScope.GLOBAL, data);

  // Add project settings if in project.
  getConfig(configScope.PROJECT, data);

  // Add local settings if in project.
  getConfig(configScope.LOCAL, data);

  const result = settingsSchema.safeParse(data);
  if (result.success) {
    // Safe parse the resulting data into a new settings object.
    Object.assign(jcoreSettingsData, result.data);
  } else {
    logger.error("Invalid data in settings file.");
    process.exit();
  }
}

export function getConfig(scope: configScope = configScope.GLOBAL, data = {}) {
  return Object.assign(data, loadConfigFile(getScopeConfigFile(scope)));
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

function writeData() {
  writeFileSync(globalData, JSON.stringify(jcoreDataData, null, 2), "utf8");
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

export function updateSetting(key: string, value: null | configValue, requestedScope: configScope) {
  const scope = validateScope(key, requestedScope);
  const configFile = getScopeConfigFile(scope);

  if (setConfigValue(key, value, configFile)) {
    updateInfo(key, value, scope);
    return true;
  }
  return false;
}

function getScopeConfigFile(scope: configScope) {
  switch (scope) {
    case configScope.DEFAULT:
      return join(jcoreRuntimeData.workDir, defaultConfigFilename);
    case configScope.GLOBAL:
      return globalConfig;
    case configScope.LOCAL:
      return join(jcoreRuntimeData.workDir, localConfigFilename);
    case configScope.PROJECT:
      return join(jcoreRuntimeData.workDir, projectConfigFilename);
  }
  return "";
}

function validateScope(key: string, scope: configScope) {
  if (!jcoreRuntimeData.inProject && scope !== configScope.GLOBAL) {
    if (scope === configScope.PROJECT) {
      logger.error("Project setting, but not in Project!");
    } else {
      logger.error("Not in Project. Use -g for global setting.");
    }
    return configScope.INVALID;
  }

  if (scope === configScope.PROJECT && !projectSettings.includes(key)) {
    logger.debug(`Project settings doesn't include ${key}, switching to local`);
    return configScope.LOCAL;
  }
  return scope;
}

function updateInfo(key: string, value: null | configValue, scope: configScope) {
  const scopeText =
    scope === configScope.GLOBAL ? "Global" : scope === configScope.PROJECT ? "Project" : "Local";

  if (value === null) {
    logger.info(`${scopeText} setting ${chalk.green(key)} removed`);
  } else {
    logger.info(`${scopeText} setting ${chalk.green(key)} updated to ${formatValue(value)}`);
  }
}

function setConfigValue(key: string, value: null | configValue, file: string) {
  try {
    const settings: Record<string, configValue> = {};
    const values = loadConfigFile(file);
    if (value === null) {
      delete values[key];
    } else {
      settings[key] = value;
    }
    saveConfigFile(file, Object.assign(values, settings));
  } catch (e) {
    logger.error(`Updating ${key} failed.`);
    return false;
  }
  return true;
}

function loadConfigFile(file: string): Record<string, configValue> {
  const info = parse(file);
  const jsonFile = file.replace(info.ext, ".json");

  if (existsSync(file)) {
    try {
      const toml = readFileSync(file, "utf8");
      const parsed = tomlParse(toml);
      return settingsSchema.partial().parse(parsed);
    } catch (error) {
      if (error instanceof TomlError) {
        logger.error(`TOML error in file ${file} on line ${error.line}`);
        logger.debug(error.message);
      } else if (error instanceof ZodError) {
        logger.error(`Settings parse error in file ${file}`);
        for (const issue of error.issues) {
          logger.error(`Property [${issue.path.join(".")}]: ${issue.message}`);
        }
      } else {
        console.log(error);
      }
      process.exit();
    }
  } else if (existsSync(jsonFile)) {
    try {
      const json = readFileSync(jsonFile, "utf8");
      const parsed = JSON.parse(json);
      return settingsSchema.partial().parse(parsed);
    } catch {
      logger.error(`JSON parse error in file ${jsonFile}`);
      process.exit();
    }
  }
  return {};
}

export function saveConfigFile(file: string, data: Record<string, configValue | undefined>) {
  try {
    const info = parse(file);
    const jsonFile = file.replace(info.ext, ".json");
    const dataString = tomlStringify(data);
    writeFileSync(file, dataString, "utf8");
    if (existsSync(jsonFile)) {
      unlinkSync(jsonFile);
    }
  } catch {
    logger.error(`Error in writing config file: ${file}`);
    process.exit();
  }
}
