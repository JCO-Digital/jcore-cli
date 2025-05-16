import { execSync } from "child_process";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "fs";
import { homedir } from "os";
import { join, parse } from "path";
import { formatValue } from "@/commands/config";
import {
  configScope,
  defaultConfigFilename,
  localConfigFilename,
  projectConfigFilename,
  projectSettings,
} from "@/constants";
import { convertGlobalSettings, projectConfigLegacyFilename } from "@/legacy";
import { logger } from "@/logger";
import parser, { jcoreCmdData } from "@/parser";
import {
  configValue,
  type jcoreData,
  runtimeSchema,
  settingsSchema,
} from "@/types";
import { loadJsonFile } from "./fileHelpers";
import { fetchVersion, parseErrorHandler } from "@/utils";
import chalk from "chalk";
import * as process from "process";
import { parse as tomlParse, stringify as tomlStringify } from "smol-toml";
import { version } from "../package.json";

// Runtime settings.
export const jcoreRuntimeData = runtimeSchema.parse({
  workDir: process.cwd(),
});

// Default settings.
export const jcoreSettingsData = settingsSchema.parse({
  mode: "foreground",
  theme: "jcore2-child",
  wpImage: "jcodigi/wordpress:latest",
  wpVersion: "latest",
});

export const jcoreDataData = {
  version: version,
  latest: "",
  lastCheck: 0,
} as jcoreData;

const globalConfigFolder = join(homedir(), ".config/jcore");
const globalConfig = join(globalConfigFolder, "config.toml");
const globalData = join(globalConfigFolder, "data.json");

export async function readSettings() {
  parser(process.argv);

  // Set initial logLevel for settings function.
  jcoreSettingsData.logLevel = jcoreCmdData.logLevel;

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

    // Get current Git Branch.
    jcoreRuntimeData.branch = readCurrentGitBranch();
  }

  // Read global app data.
  readData();

  // Convert old format to new.
  convertGlobalSettings(globalConfig);

  if (
    jcoreRuntimeData.inProject &&
    !existsSync(join(jcoreRuntimeData.workDir, projectConfigFilename))
  ) {
    // We are in a legacy project.
    if (jcoreCmdData.cmd !== "migrate") {
      logger.warn(
        "Legacy project detected, migrate the project with 'jcore migrate'.",
      );
    }
    jcoreRuntimeData.inProject = false;
  }

  // Read global settings.
  readProjectSettings();

  if (jcoreCmdData.logLevel !== 2) {
    // If commandline log level is set, overwrite saved level.
    jcoreSettingsData.logLevel = jcoreCmdData.logLevel;
  }

  versionCheck()
    .then(() => {
      logger.debug("Version check done.");
    })
    .catch((reason) => {
      logger.warn(reason);
    });
}

function readCurrentGitBranch() {
  const options = {
    cwd: jcoreRuntimeData.workDir,
  };
  try {
    return execSync("git branch --show-current", options).toString().trim();
  } catch (e) {
    logger.error("Can't read git branch.");
  }
  return "";
}

export function readProjectSettings() {
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

  if (!data.template) {
    data.template = "jcore2";
  }

  try {
    const result = settingsSchema.parse(data);
    // Safe parse the resulting data into a new settings object.
    Object.assign(jcoreSettingsData, result);
  } catch (error) {
    logger.error("Invalid data in settings file.");
    process.exit();
  }
}

export function getConfig(scope: configScope = configScope.GLOBAL, data = {}) {
  return Object.assign(data, loadConfigFile(getScopeConfigFile(scope)));
}

function readData() {
  logger.debug("Reading data file.");
  const values = loadJsonFile(globalData);
  if (typeof values.latest === "string") {
    jcoreDataData.latest = values.latest;
  }
  if (typeof values.lastCheck === "number") {
    jcoreDataData.lastCheck = values.lastCheck;
  }
}

function writeData() {
  if (!existsSync(globalConfigFolder)) {
    mkdirSync(globalConfigFolder);
  }
  writeFileSync(globalData, JSON.stringify(jcoreDataData, null, "\t"), "utf8");
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

export function setConfigValue(
  key: string,
  value: configValue,
  requestedScope: configScope,
) {
  const scope = validateScope(key, requestedScope);
  if (scope === configScope.INVALID) {
    return false;
  }
  const configFile = getScopeConfigFile(scope);
  const settings: Record<string, configValue> = {};
  settings[key] = value;
  if (updateConfigValues(settings, configFile)) {
    logger.info(
      `${getScopeText(scope)} setting ${chalk.green(
        key,
      )} updated to ${formatValue(value)}`,
    );
    return true;
  }
  return false;
}

export function deleteSetting(key: string, requestedScope: configScope) {
  const scope = validateScope(key, requestedScope);
  const configFile = getScopeConfigFile(scope);

  try {
    const values = loadConfigFile(configFile);
    delete values[key];
    saveConfigFile(configFile, values);
    logger.info(`${getScopeText(scope)} setting ${chalk.green(key)} removed`);
  } catch (e) {
    logger.error(`Deleting ${key} failed.`);
    return false;
  }
  return true;
}

export function getScopeConfigFile(scope: configScope) {
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

function validateScope(key: string, requestedScope: configScope) {
  const scope = projectSettings.includes(key)
    ? configScope.PROJECT
    : configScope.GLOBAL;

  if (!jcoreRuntimeData.inProject) {
    if (scope === configScope.PROJECT) {
      logger.error("Project setting, but not in Project!");
      return configScope.INVALID;
    }
    if (requestedScope === configScope.LOCAL) {
      logger.error("Not in project, but local setting requested.");
      return configScope.INVALID;
    }
  }

  if (requestedScope === configScope.PROJECT && scope === configScope.GLOBAL) {
    logger.error(
      `Global setting ${key} can't be set in project, use --global or --local.`,
    );
    return configScope.INVALID;
  }
  return requestedScope;
}

function getScopeText(scope: configScope) {
  switch (scope) {
    case configScope.DEFAULT:
      return "Default";
    case configScope.GLOBAL:
      return "Global";
    case configScope.PROJECT:
      return "Project";
    case configScope.LOCAL:
      return "Local";
    default:
      return "Unknown";
  }
}

export function updateConfigValues(
  settings: Record<string, configValue>,
  file: string,
) {
  try {
    const values = loadConfigFile(file);
    saveConfigFile(file, Object.assign(values, settings));
  } catch (e) {
    logger.error("Updating of settings failed.");
    return false;
  }
  return true;
}

function loadConfigFile(file: string): Record<string, configValue> {
  if (existsSync(file)) {
    try {
      const toml = readFileSync(file, "utf8");
      const parsed = tomlParse(toml);
      const branch = `branch-${jcoreRuntimeData.branch}`;
      if (typeof parsed[branch] === "object") {
        Object.assign(parsed, parsed[branch]);
      }
      return settingsSchema.partial().parse(parsed);
    } catch (error) {
      parseErrorHandler(error, file);
      process.exit();
    }
  }
  return {};
}

export function saveConfigFile(
  file: string,
  data: Record<string, configValue | undefined>,
) {
  try {
    const dataString = tomlStringify(data);
    writeFileSync(file, dataString, "utf8");
  } catch {
    logger.error(`Error in writing config file: ${file}`);
    process.exit();
  }
}
