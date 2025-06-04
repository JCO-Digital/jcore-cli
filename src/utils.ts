import { join } from "path";
import { scriptLocation } from "@/constants";
import { logger } from "@/logger";
import { jcoreCmdData } from "@/parser";
import { jcoreRuntimeData } from "@/settings";
import { TomlError } from "smol-toml";
import { ZodError } from "zod";
import { getFileString } from "./fileHelpers";

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
        } catch (error) {
          if (error instanceof Error) {
            logger.error(error.message);
          }
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
  } else if (error instanceof Error) {
    logger.error(`Error in file ${file}: ${error.message}`);
  } else {
    console.error(error);
  }
}

/**
 * Handles errors by logging them with an optional prefix.
 *
 * @param {unknown} error - The error to handle.
 * @param {string} [prefix=""] - An optional prefix to prepend to the error message.
 */
export function errorHandler(error: unknown, prefix: string = "") {
  if (error instanceof Error) {
    let errorMessage = error.message;
    if (prefix) {
      errorMessage = `${prefix}: ${errorMessage}`;
    }
    logger.error(errorMessage);
  } else {
    console.error(error);
  }
}
