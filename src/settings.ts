import * as process from "process";
import { join, parse } from "path";
import { homedir } from "os";
import { existsSync, readFileSync, writeFileSync } from "fs";
import { config, version } from "../package.json";
import { calculateChecksum, fetchVersion, loadChecksums, saveChecksums } from "@/utils";
import { replaceInFile } from "@/project";
import { logger } from "@/logger";
import type { jcoreSettings } from "@/types";
import { dataSchema, jcoreData, projectData } from "@/types";

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
  projects: [],
  version: version,
  latest: "",
  lastCheck: 0,
} as jcoreData;

const values = new Map() as Map<string, string | string[]>;

const globalConfig = join(homedir(), ".config/jcore/config");
const globalData = join(homedir(), ".config/jcore/data.json");

export async function readSettings() {
  // Find the project base path.
  while (
    jcoreSettingsData.path.length > 1 &&
    !existsSync(join(jcoreSettingsData.path, "config.sh"))
  ) {
    // Go up one level and try again.
    jcoreSettingsData.path = parse(jcoreSettingsData.path).dir;
  }
  // Check if we are in a project.
  jcoreSettingsData.inProject = jcoreSettingsData.path.length > 1;
  if (jcoreSettingsData.inProject) {
    jcoreSettingsData.name = parse(jcoreSettingsData.path).base;
  }

  readData();

  values.set("name", jcoreSettingsData.name);
  if (existsSync(globalConfig)) {
    // Read global settings if they exist.
    const data = readFileSync(globalConfig, "utf8");
    parseSettings(data);
  }

  if (jcoreSettingsData.inProject) {
    // Read project settings if in project.
    const data = readFileSync(join(jcoreSettingsData.path, "/config.sh"), "utf8");
    parseSettings(data);
  }

  populateSetting();

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

  if (!jcoreSettingsData.inProject && !jcoreSettingsData.branch) {
    jcoreSettingsData.branch = config.branch;
  }
}

function readData() {
  if (existsSync(globalData)) {
    // Read global settings if they exist.
    const json = readFileSync(globalData, "utf8");
    try {
      const data = JSON.parse(json);
      const parsed = dataSchema.safeParse(data);
      if (parsed.success) {
        populateData(parsed.data);
      }
    } catch (error) {
      logger.warn("Data parsing failed.");
    }
  }
}

function writeData() {
  writeFileSync(globalData, JSON.stringify(jcoreDataData), "utf8");
}

export function startProject() {
  cleanProjects();

  let found = false;
  for (const project of jcoreDataData.projects) {
    if (project.path === jcoreSettingsData.path) {
      found = true;
      project.running = true;
      project.started = Date.now();
    }
  }
  if (!found) {
    const project = {
      name: jcoreSettingsData.name,
      path: jcoreSettingsData.path,
      running: true,
      started: Date.now(),
    } as projectData;
    jcoreDataData.projects.push(project);
  }

  writeData();
}

export function stopProject() {
  for (const project of jcoreDataData.projects) {
    if (project.path === jcoreSettingsData.path) {
      project.running = false;
    }
  }

  cleanProjects();

  writeData();
}

function cleanProjects() {
  const paths: string[] = [];
  const remove: number[] = [];
  for (let index = 0; index < jcoreDataData.projects.length; index++) {
    const project = jcoreDataData.projects[index];
    if (!paths.includes(project.path)) {
      if (existsSync(project.path)) {
        // Project ok.
        paths.push(project.path);
      } else {
        // Remove old project.
        remove.push(index);
        logger.debug(`Folder ${project.path} doesn't exist, removing.`);
      }
    } else {
      // TODO Remove duplicate.
      remove.push(index);
      logger.debug(`Duplicate folder ${project.path}, removing.`);
    }
  }
  for (const index of remove) {
    jcoreDataData.projects.splice(index, 1);
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

export function writeGlobalSettings() {
  const setValues = [
    { key: "mode", value: jcoreSettingsData.mode },
    { key: "debug", value: jcoreSettingsData.debug.toString() },
    { key: "loglevel", value: jcoreSettingsData.logLevel.toString() },
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
  if (jcoreSettingsData.inProject) {
    // Save project settings.
    const setValues = [
      { key: "name", value: jcoreSettingsData.name },
      { key: "theme", value: jcoreSettingsData.theme },
      { key: "install", value: jcoreSettingsData.install ? "true" : "false" },
    ];

    const configReplace = [];
    const packageReplace = [];
    for (const row of setValues) {
      const key = row.key.toUpperCase();
      configReplace.push({
        search: new RegExp(`^#?${key}="[^"]*" *$`, "m"),
        replace: `${key}="${row.value}"`,
      });
      packageReplace.push({
        search: new RegExp(`"${row.key}" *: *"[^"]*"`, "m"),
        replace: `"${row.key}": "${row.value}"`,
      });
    }

    const files = [
      {
        name: "config.sh",
        replace: configReplace,
      },
      {
        name: "package.json",
        replace: packageReplace,
      },
    ];

    const checksums = loadChecksums();
    for (const file of files) {
      const filePath = join(jcoreSettingsData.path, file.name);
      const checksum = calculateChecksum(filePath);
      replaceInFile(filePath, file.replace);
      if (checksum === checksums.get(file.name)) {
        logger.debug("Checksums Match");
        checksums.set(file.name, calculateChecksum(filePath));
      }
    }
    saveChecksums(checksums);
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
          jcoreSettingsData.path = value;
          break;
        case "mode":
          jcoreSettingsData.mode = value;
          break;
        case "debug":
          jcoreSettingsData.debug = value === "1" || value === "true";
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
        case "plugin_install":
          jcoreSettingsData.plugins = value;
          break;
        case "install":
          jcoreSettingsData.install = value === "1" || value === "true";
          break;
      }
    } else {
      if (key === "domains") {
        const parts = value[0].split(";");
        jcoreSettingsData.domain = parts[0];
        jcoreSettingsData.local = parts[1] + ".localhost";
      }
    }
  }
}

function populateData(data: jcoreData) {
  jcoreDataData.projects = data.projects;
  jcoreDataData.lastCheck = data.lastCheck;
  jcoreDataData.latest = data.latest;
}
