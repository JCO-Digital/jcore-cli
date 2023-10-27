import { https } from "follow-redirects";
import AdmZip from "adm-zip";
import { join } from "path";
import { createHash } from "crypto";
import { checksumFile, scriptLocation } from "@/constants";
import {
  copyFileSync,
  existsSync,
  lstatSync,
  mkdirSync,
  readdirSync,
  readFileSync,
  renameSync,
  writeFileSync
} from "fs";
import { jcoreSettingsData } from "@/settings";
import { logger } from "@/logger";
import { cmdData } from "@/types";

export function getFileString(url: string): Promise<string> {
  return new Promise((resolve, reject) => {
    https.get(url)
      .on("response", function(response) {
        if (response.statusCode === 200) {
          let body = "";
          response.on("data", function(chunk) {
            body += chunk;
          });
          response.on("end", function() {
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
    https.get(url).on("response", function(response) {
      if (response.statusCode === 200) {
        const data: Buffer[] = [];

        response
          .on("data", function(chunk) {
            data.push(chunk);
          })
          .on("end", function() {
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
      .catch(() => {
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
  try {
    const json = readFileSync(join(jcoreSettingsData.path, checksumFile), "utf8");
    const data = JSON.parse(json);
    return new Map(Object.entries(data));
  } catch {
    return new Map<string, string>();
  }
}

export function saveChecksums(checksums: Map<string, string>): boolean {
  try {
    const object = Object.fromEntries(checksums);
    const json = JSON.stringify(object);
    writeFileSync(join(jcoreSettingsData.path, checksumFile), json, "utf8");
    return true;
  } catch {
    return false;
  }
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
    logger.verbose("Creating target folder: " + destinationDir);
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


/**
 * Moves source to destination, merging with existing structure, overwriting files with checksum validation.
 * @param sourceDir Source Folder
 * @param destinationDir Destination Folder
 * @param path Relative path
 * @param checksums File checksum map, or null to skip checksums.
 */
export function moveFiles(sourceDir: string, destinationDir: string, path: string, checksums: Map<string, string>) {
  if (!existsSync(join(destinationDir, path))) {
    // Create target if not exists.
    logger.verbose("Creating target folder: " + destinationDir);
    mkdirSync(join(destinationDir, path), { recursive: true });
  }
  for (const file of readdirSync(join(sourceDir, path))) {
    if (file === ".git") {
      // Skip .git folder.
      continue;
    }
    const filePath = join(path, file);
    if (lstatSync(join(sourceDir, filePath)).isDirectory()) {
      // Current path is a folder.
      if (!existsSync(join(destinationDir, filePath))) {
        // Create destination folder if it doesn't exist.
        mkdirSync(join(destinationDir, filePath));
      }
      // Merge files in folder.
      moveFiles(sourceDir, destinationDir, filePath, checksums);
    } else {
      // Current path is a file.
      const source = join(sourceDir, filePath);
      const destination = join(destinationDir, filePath);
      if (!existsSync(destination) || calculateChecksum(destination) === checksums.get(filePath)) {
        // Destination matches checksum or doesn't exist.
        renameSync(source, destination);
        checksums.set(filePath, calculateChecksum(join(destinationDir, path, file)));
      } else {
        logger.error("Skipping " + filePath);
      }
    }
  }
}

export function isProject(project = true): boolean {
  if (!project && jcoreSettingsData.inProject) {
    logger.error("\nAlready in project.");
  }
  if (project && !jcoreSettingsData.inProject) {
    logger.error("\nNot in a project.");
  }
  return jcoreSettingsData.inProject === project;
}

export function nameToFolder(name: string): string {
  return name.toLowerCase().replace(/[^a-z]/, "-");
}

export function getFlagValue(cmd: cmdData, name: string): false | any {
  return cmd.flags.has(name) ? cmd.flags.get(name) : false;
}
