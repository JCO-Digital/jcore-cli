import { get } from "https";
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
  writeFileSync,
} from "fs";
import { settings } from "@/settings";
import { logger } from "@/logger";

export function getFileString(url: string): Promise<string> {
  return new Promise((resolve, reject) => {
    get(url)
      .on("response", function (response) {
        if (response.statusCode === 200) {
          let body = "";
          response.on("data", function (chunk) {
            body += chunk;
          });
          response.on("end", function () {
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
    get(url).on("response", function (response) {
      if (response.statusCode === 200) {
        const data: Buffer[] = [];

        response
          .on("data", function (chunk) {
            data.push(chunk);
          })
          .on("end", function () {
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
    const json = readFileSync(join(settings.path, checksumFile), "utf8");
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
    writeFileSync(join(settings.path, checksumFile), json, "utf8");
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
 * Moves or copies source to destination, merging with existing structure, overwriting files.
 * @param sourceDir Source Folder
 * @param destinationDir Destination Folder
 * @param copy Copies files if true, moves files if false.
 */
export function mergeFiles(sourceDir: string, destinationDir: string, copy = false) {
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
      if (!existsSync(join(destinationDir, file))) {
        mkdirSync(join(destinationDir, file));
      }
      mergeFiles(join(sourceDir, file), join(destinationDir, file), copy);
    } else {
      if (copy) {
        copyFileSync(join(sourceDir, file), join(destinationDir, file));
      } else {
        renameSync(join(sourceDir, file), join(destinationDir, file));
      }
    }
  }
}

export function isProject(project = true): boolean {
  if (!project && settings.inProject) {
    logger.error("\nAlready in project.");
  }
  if (project && !settings.inProject) {
    logger.error("\nNot in a project.");
  }
  return settings.inProject === project;
}

export function nameToFolder(name: string): string {
  return name.toLowerCase().replace(/[^a-z]/, "-");
}
