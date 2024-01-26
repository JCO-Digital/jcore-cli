import { createHash } from "crypto";
import {
  copyFileSync,
  existsSync,
  lstatSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  writeFileSync,
} from "fs";
import { join } from "path";
import { checksumFile, scriptLocation } from "@/constants";
import { logger } from "@/logger";
import { jcoreCmdData } from "@/parser";
import { jcoreRuntimeData, jcoreSettingsData } from "@/settings";
import { configValue, jsonValue } from "@/types";
import AdmZip from "adm-zip";
import { https } from "follow-redirects";
import process from "process";
import { TomlError, parse as tomlParse } from "smol-toml";
import { ZodError } from "zod";

export function getFileString(url: string): Promise<string> {
  logger.debug(`Getting file ${url} as string.`);
  return new Promise((resolve, reject) => {
    https
      .get(url)
      .on("response", (response) => {
        if (response.statusCode === 200) {
          let body = "";
          response.on("data", (chunk) => {
            body += chunk;
          });
          response.on("end", () => {
            resolve(body);
          });
        } else {
          reject(`Call failed with code: ${response.statusCode}`);
        }
      })
      .on("error", () => {
        reject(`Fetch from ${url} failed!`);
      });
  });
}

export function getFile(url: string): Promise<Buffer> {
  return new Promise<Buffer>((resolve, reject) => {
    https.get(url).on("response", (response) => {
      if (response.statusCode === 200) {
        const data: Buffer[] = [];

        response
          .on("data", (chunk) => {
            data.push(chunk);
          })
          .on("end", () => {
            //at this point data is an array of Buffers
            //so Buffer.concat() can make us a new Buffer
            //of all of them together
            resolve(Buffer.concat(data));
          });
      } else {
        reject(response.statusCode);
      }
    });
  });
}

export function fetchVersion(): Promise<string> {
  return new Promise((resolve, reject) => {
    getFileString(join(scriptLocation, "package.json"))
      .then((json) => {
        logger.debug("File contents fetched.");
        try {
          const info = JSON.parse(json);
          if (typeof info.version === "string") {
            resolve(info.version);
          }
          reject("Version in version fetch is of wrong format.");
        } catch (e) {
          reject("JSON error in version fetch.");
        }
      })
      .catch((e) => {
        logger.error(e.toString());
        reject("Network error in version fetch.");
      });
  });
}

export function extractArchive(buffer: Buffer, output: string): Promise<void> {
  return new Promise((resolve, reject) => {
    try {
      const zip = new AdmZip(buffer);
      zip.extractAllTo(output);

      resolve();
    } catch (e) {
      reject(e);
    }
  });
}

export function loadChecksums(): Map<string, string> {
  const data = loadJsonFile(join(jcoreRuntimeData.workDir, checksumFile));
  const checksums = new Map<string, string>();
  for (const [key, value] of Object.entries(data)) {
    if (typeof value === "string") {
      checksums.set(key, value);
    }
  }
  return checksums;
}

export function saveChecksums(checksums: Map<string, string>): boolean {
  try {
    const object = Object.fromEntries(checksums);
    const json = JSON.stringify(object, null, 2);
    writeFileSync(join(jcoreRuntimeData.workDir, checksumFile), json, "utf8");
    return true;
  } catch {
    return false;
  }
}

export function compareChecksum(file: string, strict = true): boolean {
  const checksums = loadChecksums();
  if (!strict && checksums.get(file) === undefined) {
    // Return true for missing checksum in non-strict mode.
    return true;
  }
  return checksums.get(file) === calculateChecksum(file);
}

export function loadJsonFile(file: string): Record<string, jsonValue> {
  if (existsSync(file)) {
    try {
      const json = readFileSync(file, "utf8");
      return JSON.parse(json);
    } catch {
      logger.error(`JSON parse error in file ${file}`);
      process.exit();
    }
  }
  return {};
}

export function calculateChecksum(file: string): string {
  try {
    const data = readFileSync(file, "utf8");
    return createHash("sha256").update(data).digest("hex");
  } catch (e) {
    return "";
  }
}

/**
 * Copies source to destination, merging with existing structure, overwriting files.
 * @param sourceDir Source Folder
 * @param destinationDir Destination Folder
 */
export function copyFiles(sourceDir: string, destinationDir: string) {
  if (!existsSync(destinationDir)) {
    // Create target if not exists.
    logger.verbose(`Creating target folder: ${destinationDir}`);
    mkdirSync(destinationDir, { recursive: true });
  }
  for (const file of readdirSync(sourceDir)) {
    if (file === ".git") {
      // Skip .git folder.
      continue;
    }
    if (lstatSync(join(sourceDir, file)).isDirectory()) {
      // Current path is a folder.
      if (!existsSync(join(destinationDir, file))) {
        // Create destination folder if it doesn't exist.
        mkdirSync(join(destinationDir, file));
      }
      // Merge files in folder.
      copyFiles(join(sourceDir, file), join(destinationDir, file));
    } else {
      // Current path is a file.
      copyFileSync(join(sourceDir, file), join(destinationDir, file));
    }
  }
}

export function isProject(project = true): boolean {
  if (!project && jcoreRuntimeData.inProject) {
    logger.error("\nAlready in project.");
  }
  if (project && !jcoreRuntimeData.inProject) {
    logger.error("\nNot in a project.");
  }
  return jcoreRuntimeData.inProject === project;
}

export function nameToFolder(name: string): string {
  return name.toLowerCase().replace(/[^a-z]/, "-");
}

export function getFlag(name: string): boolean {
  if (jcoreCmdData.flags.has(name)) {
    const value = jcoreCmdData.flags.get(name);
    if (typeof value === "boolean") {
      return value;
    }
  }
  return false;
}

export function getFlagString(name: string): string | undefined {
  if (jcoreCmdData.flags.has(name)) {
    const value = jcoreCmdData.flags.get(name);
    if (typeof value === "string") {
      return value;
    }
  }
  return undefined;
}

export function getFlagNumber(name: string): number | undefined {
  if (jcoreCmdData.flags.has(name)) {
    const value = jcoreCmdData.flags.get(name);
    if (typeof value === "number") {
      return value;
    }
  }
  return undefined;
}

export function getSetupFolder(appendPath = "", inContainer = false): string {
  const path = inContainer ? "/project" : jcoreRuntimeData.workDir;
  return join(path, ".config", appendPath);
}

export function parseErrorHandler(error: unknown, file: string) {
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
}

export function createEnv() {
  const file = join(jcoreRuntimeData.workDir, "env-values.toml");
  if (existsSync(file)) {
    try {
      const toml = readFileSync(file, "utf8");
      const values = tomlParse(toml) as Record<string, configValue>;

      let env = "";
      for (const key in Object.assign(values, jcoreSettingsData)) {
        const value = values[key];
        if (key === "replace") {
          const defaultRow = `//${jcoreSettingsData.remoteDomain}|//${jcoreSettingsData.localDomain}`;
          if (Array.isArray(value) && !value.includes(defaultRow)) {
            value.push(defaultRow);
          }
        }
        env += `${createEnvName(key)}="${createEnvVariable(value)}"\n`;
      }

      writeFileSync(join(jcoreRuntimeData.workDir, ".env"), env);
    } catch (error) {
      parseErrorHandler(error, file);
      process.exit();
    }
  }
}

function createEnvName(name: string) {
  // Convert camelCase to UPPERCASE_SNAKE.
  return name.replace(/([A-Z])/g, "_$1").toUpperCase();
}

function createEnvVariable(value: configValue): string {
  if (Array.isArray(value)) {
    const output: Array<string> = [];
    for (const row of value) {
      output.push(row);
    }
    return output.join(" ");
  }
  switch (typeof value) {
    case "string":
      return value;
    case "number":
      return value.toString();
    case "boolean":
      return value ? "true" : "false";
  }
}
