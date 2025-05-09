import AdmZip from "adm-zip";
import { https } from "follow-redirects";
import {
  copyFileSync,
  existsSync,
  lstatSync,
  mkdirSync,
  readdirSync,
  readFileSync,
  writeFileSync,
} from "fs";
import Mustache from "mustache";
import { join } from "path";
import { logger } from "./logger";

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
 * Loads a JSON file from disk, and return the JSON object.
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
 * Gets the path to the container folder from a zipfile, if file only contains one folder.
 * If zip file contains more than one folder, return the containing folder.
 *
 * @param {string} path - The path the zip file was extracted to.
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
