import { createHash } from "crypto";
import {
  existsSync,
  lstatSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  writeFileSync,
  copyFileSync,
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
import Mustache from "mustache";

/**
 * Fetches a file from a given URL and returns it as a string.
 *
 * @param {string} url - The URL of the file to fetch.
 * @returns {Promise<string>} Promise that resolves to the file content as a string.
 */
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

/**
 * Fetches a file from a given URL and returns it as a Buffer.
 *
 * @param {string} url - The URL of the file to fetch.
 * @returns {Promise<Buffer>} Promise that resolves with the file content as a Buffer.
 */
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

/**
 * Fetches the current version from the package.json file.
 *
 * @returns {Promise<string>} Promise that resolves to the version string.
 */
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

/**
 * Extracts an archive (zip) from a Buffer to a specified output directory.
 *
 * @param {Buffer} buffer - The Buffer containing the zip archive.
 * @param {string} output - The directory path to extract the archive to.
 * @returns {Promise<void>} Promise that resolves when extraction is complete, or rejects on error.
 */
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

/**
 * Loads checksums from the checksum file.
 *
 * @returns {Map<string, string>} The loaded checksums.
 */
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

/**
 * Saves checksums from a Map to a JSON file.
 *
 * @param {Map<string, string>} checksums - The map of checksums to save.
 * @returns {boolean} True if saving was successful, false otherwise.
 */
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

/**
 * Compares the checksum of a file with the stored checksum.
 *
 * @param {string} file - The file to compare the checksum for.
 * @param {boolean} strict - Whether to perform a strict comparison.
 * @returns {boolean} True if the checksums match, false otherwise.
 */
export function compareChecksum(file: string, strict: boolean = true): boolean {
  const checksums = loadChecksums();
  if (!strict && checksums.get(file) === undefined) {
    // Return true for missing checksum in non-strict mode.
    return true;
  }
  return (
    checksums.get(file) ===
    calculateChecksum(join(jcoreRuntimeData.workDir, file))
  );
}

/**
 * Updates the checksum of a file.
 *
 * @param {string} file - The file to update the checksum for.
 */
export function updateChecksum(file: string) {
  const checksums = loadChecksums();
  checksums.set(file, calculateChecksum(file));
  saveChecksums(checksums);
}

/**
 * Loads a JSON file.
 *
 * @param {string} file - The file to load.
 * @returns {Record<string, jsonValue>} The loaded JSON data.
 */
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

/**
 * Calculates the checksum of a file.
 *
 * @param {string} file - The file to calculate the checksum for.
 * @returns {string} The calculated checksum.
 */
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
 * @param {string} sourceDir - Source Folder
 * @param {string} destinationDir - Destination Folder
 * @param {object} context - Template Context
 */
export function copyFiles(
  sourceDir: string,
  destinationDir: string,
  context: object = {},
) {
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
      applyFromTemplate(
        context,
        join(sourceDir, file),
        join(destinationDir, file),
      );
    }
  }
}

/**
 * Copies a file from a template or a normal file.
 *
 * @param {object} context - The context to apply to the template.
 * @param {string} templateFile - The source of the template.
 * @param {string} destinationFile - The destination file to write the template to.
 * @returns {boolean} True if the template was applied, false otherwise.
 */
export function applyFromTemplate(
  context: object,
  templateFile: string,
  destinationFile: string = "",
): boolean {
  const ending = ".mustache";

  /*
   * Assign sourceFile without ending, and templateFile with ending.
   * Regardless of which is originally given.
   */
  let sourceFile = templateFile;
  if (templateFile.endsWith(ending)) {
    sourceFile = templateFile.slice(0, -9);
  } else {
    templateFile += ending;
  }

  if (!destinationFile) {
    // Assign destinationFile if not given.
    destinationFile = sourceFile;
  } else if (destinationFile.endsWith(ending)) {
    // Remove ending from destinationFile if it has it.
    destinationFile = destinationFile.slice(0, -9);
  }

  try {
    if (existsSync(templateFile)) {
      const data = readFileSync(templateFile, "utf8");
      writeFileSync(destinationFile, Mustache.render(data, context));
    } else if (existsSync(sourceFile)) {
      copyFileSync(sourceFile, destinationFile);
    } else {
      return false;
    }
  } catch (reason) {
    return false;
  }
  return true;
}

/**
 * Gets the unzipped folder.
 *
 * @param {string} path - The path to the unzipped folder.
 * @returns {string} The path to the unzipped folder.
 */
export function getUnzippedFolder(path: string): string {
  const contents = readdirSync(path);
  if (contents.length === 1) {
    return join(path, contents[0]);
  }
  return path;
}

/**
 * Checks if the current directory is a project.
 *
 * @param {boolean} project - Whether to check if the current directory is a project.
 * @returns {boolean} True if the current directory is a project, false otherwise.
 */
export function isProject(project: boolean = true): boolean {
  if (!project && jcoreRuntimeData.inProject) {
    logger.error("\nAlready in project.");
  }
  if (project && !jcoreRuntimeData.inProject) {
    logger.error("\nNot in a project.");
  }
  return jcoreRuntimeData.inProject === project;
}

/**
 * Converts text to a slug.
 *
 * @param {string} text - The text to slugify.
 * @returns {string} The slugified text.
 */
export function slugify(text: string): string {
  return text
    .trim()
    .toLowerCase()
    .replace(/[\s_]+/g, "-")
    .replace(/[^a-z0-9-]/g, "")
    .replace(/-+/g, "-")
    .replace(/^-+|-+$/g, "");
}

/**
 * Gets a flag.
 *
 * @param {string} name - The name of the flag to get.
 * @returns {boolean} The value of the flag.
 */
export function getFlag(name: string): boolean {
  if (jcoreCmdData.flags.has(name)) {
    const value = jcoreCmdData.flags.get(name);
    if (typeof value === "boolean") {
      return value;
    }
  }
  return false;
}

/**
 * Gets a flag as a string.
 *
 * @param {string} name - The name of the flag to get.
 * @returns {string | undefined} The value of the flag as a string, or undefined if the flag is not a string.
 */
export function getFlagString(name: string): string | undefined {
  if (jcoreCmdData.flags.has(name)) {
    const value = jcoreCmdData.flags.get(name);
    if (typeof value === "string") {
      return value;
    }
  }
  return undefined;
}

/**
 * Gets a flag as a number.
 *
 * @param {string} name - The name of the flag to get.
 * @returns {number | undefined} The value of the flag as a number, or undefined if the flag is not a number.
 */
export function getFlagNumber(name: string): number | undefined {
  if (jcoreCmdData.flags.has(name)) {
    const value = jcoreCmdData.flags.get(name);
    if (typeof value === "number") {
      return value;
    }
  }
  return undefined;
}

/**
 * Gets the setup folder.
 *
 * @param {string} appendPath - The path to append to the setup folder.
 * @param {boolean} inContainer - Whether the setup folder is in a container.
 * @param {boolean} relative - Whether the setup folder is relative.
 * @returns {string} The path to the setup folder.
 */
export function getSetupFolder(
  appendPath: string = "",
  inContainer: boolean = false,
  relative: boolean = false,
): string {
  const relPath = join(".config", appendPath);
  if (relative) {
    return relPath;
  }
  const path = inContainer ? "/project" : jcoreRuntimeData.workDir;
  return join(path, relPath);
}

/**
 * Gets the project folder.
 *
 * @param {string} appendPath - The path to append to the project folder.
 * @returns {string} The path to the project folder.
 */
export function getProjectFolder(appendPath: string = ""): string {
  return join(jcoreRuntimeData.workDir, ".jcore", appendPath);
}

/**
 * Parses the error handler.
 *
 * @param {unknown} error - The error to parse.
 * @param {string} file - The file the error occurred in.
 */
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

/**
 * Creates the .env file.
 */
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

/**
 * Creates the env name.
 *
 * @param {string} name - The name of the env.
 * @returns {string} The created env name.
 */
function createEnvName(name: string) {
  // Convert camelCase to UPPERCASE_SNAKE.
  return name.replace(/([A-Z])/g, "_$1").toUpperCase();
}

/**
 * Creates the env variable.
 *
 * @param {configValue} value - The value of the config.
 * @returns {string} The created env variable.
 */
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
