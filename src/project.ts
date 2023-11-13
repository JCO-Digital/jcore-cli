import {
  extractArchive,
  loadChecksums,
  getFile,
  saveChecksums,
  calculateChecksum,
  getSetupFolder,
  loadJsonFile,
} from "@/utils";
import { archiveLocation, updateFolder } from "@/constants";
import { join, parse } from "path";
import { configValue, updateOptions } from "@/types";
import {
  existsSync,
  lstatSync,
  mkdirSync,
  readdirSync,
  readFileSync,
  renameSync,
  rmSync,
  writeFileSync,
} from "fs";
import { jcoreRuntimeData, jcoreSettingsData } from "@/settings";
import { logger } from "@/logger";
import { execSync } from "child_process";
import { checkFolders } from "@/commands/doctor";

const defaultOptions = {
  force: false,
  target: [],
} as updateOptions;

export async function updateFiles(options: updateOptions = defaultOptions) {
  const updatePath = join(jcoreRuntimeData.workDir, updateFolder);

  if (!jcoreSettingsData.projectName || jcoreRuntimeData.workDir === "/") {
    return Promise.reject("Not a project.");
  }

  try {
    const buffer = await getFile(archiveLocation);
    await extractArchive(buffer, updatePath);
    logger.verbose("Unzipped");

    const checksums = loadChecksums();

    logger.debug("Cleaning up legacy folders.");
    rmSync(join(jcoreRuntimeData.workDir, "Vagrantfile"), { recursive: false, force: true });
    rmSync(join(jcoreRuntimeData.workDir, ".vagrant"), { recursive: true, force: true });
    rmSync(join(jcoreRuntimeData.workDir, "config"), { recursive: true, force: true });
    rmSync(join(jcoreRuntimeData.workDir, "provisioning"), {
      recursive: true,
      force: true,
    });

    if (options.target.length === 0) {
      // Remove old setup folder if updating all files.
      logger.verbose("Remove old setup folder.");
      rmSync(join(jcoreRuntimeData.workDir, ".config"), { recursive: true, force: true });
    }

    // Move updated project files to project folder.
    moveFiles(updatePath, jcoreRuntimeData.workDir, "", checksums, options);
    saveChecksums(checksums);

    logger.verbose("Clean up remaining files.");
    rmSync(updatePath, { recursive: true, force: true });
  } catch (reason) {
    throw "Unable to extract file " + reason;
  }
}

/**
 * Moves source to destination, merging with existing structure, overwriting files with checksum validation.
 * @param sourceDir Source Folder
 * @param destinationDir Destination Folder
 * @param path Relative path
 * @param checksums File checksum map, or null to skip checksums.
 * @param options Some options passed to the command.
 */
export function moveFiles(
  sourceDir: string,
  destinationDir: string,
  path: string,
  checksums: Map<string, string>,
  options: updateOptions
) {
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
      moveFiles(sourceDir, destinationDir, filePath, checksums, options);
    } else {
      // Current path is a file.
      if (options.target.length === 0 || options.target.includes(filePath)) {
        // Only run if no target given, or file is in target list.
        const fileInfo = getFileInfo(destinationDir, filePath, checksums, options);

        if (fileInfo.overwrite) {
          // File should be overwritten.
          const source = join(sourceDir, filePath);
          const destination = join(destinationDir, fileInfo.target);
          if (fileInfo.replace.length) {
            logger.verbose(`Updating ${filePath} with replacement string.`);
            replaceInFile(source, fileInfo.replace, destination);
          } else {
            logger.verbose(`Updating ${filePath}.`);
            renameSync(source, destination);
          }
          checksums.set(fileInfo.target, calculateChecksum(destination));
        } else {
          logger.warn("Skipping " + fileInfo.target);
        }
      }
    }
  }
}

function getFileInfo(
  path: string,
  file: string,
  checksums: Map<string, string>,
  options: updateOptions
) {
  const files: Record<string, object> = {
    "readme.md": {
      force: false,
      checksum: true,
      replace: [
        {
          search: "# WordPress Container",
          replace:
            "# " +
            jcoreSettingsData.projectName.charAt(0).toUpperCase() +
            jcoreSettingsData.projectName.slice(1),
        },
      ],
    },
    "project.drone.yml": {
      target: ".drone.yml",
      force: true,
      checksum: true,
      replace: [
        {
          search: "wp-content/themes/projectname",
          replace: "wp-content/themes/" + jcoreSettingsData.theme,
        },
      ],
    },
    "package.json": {
      force: true,
      checksum: true,
      replace: [
        {
          search: '"name": "jcore",',
          replace: `"name": "${jcoreSettingsData.projectName}",`,
        },
      ],
    },
    "docker-compose.yml": {
      force: true,
      checksum: true,
      replace: [
        {
          search: "- docker.localhost",
          replace: "- " + jcoreSettingsData.projectName + ".localhost",
        },
      ],
    },
    "composer.json": {
      force: false,
      checksum: true,
    },
  };

  const fileInfo = Object.assign(
    {
      target: file, // Destination has different name.
      force: true, // Allow overwrite with force flag.
      checksum: false, // Is file allowed to be overwritten on missing checksum.
      replace: [], // String replace in file.
      overwrite: false, // Flag to tell "moveFiles" to overwrite file.
    },
    files[file] ?? {}
  );

  // Check if force flag is set and file is allowed to be forced or targeted.
  if (options.force && (fileInfo.force || options.target.includes(fileInfo.target))) {
    logger.debug(`Force overwrite ${fileInfo.target}`);
    fileInfo.overwrite = true;
    return fileInfo;
  }

  // If destination doesn't exist, set overwrite.
  const destination = join(path, fileInfo.target);
  if (!existsSync(destination)) {
    logger.debug(`File ${fileInfo.target} doesn't exist, overwriting.`);
    fileInfo.overwrite = true;
    return fileInfo;
  }

  // If checksum doesn't exist, check if checksum is needed, and return.
  const checksum = checksums.get(fileInfo.target);
  if (!checksum) {
    logger.debug(`File ${fileInfo.target} doesn't have a checksum.`);
    if (!fileInfo.checksum) {
      logger.debug("Overwriting anyway.");
      fileInfo.overwrite = true;
    }
    return fileInfo;
  }

  if (calculateChecksum(destination) === checksum) {
    // Checksum matches.
    logger.debug(`Checksum matches for ${fileInfo.target}.`);
    fileInfo.overwrite = true;
  }
  return fileInfo;
}

export function finalizeProject(install = true): boolean {
  const options = {
    cwd: jcoreRuntimeData.workDir,
    stdio: [0, 1, 2],
  };

  if (!checkFolders()) {
    return false;
  }

  // Write the .env file.
  createEnv();

  // Set nginx proxy pass.
  replaceInFile(getSetupFolder("nginx/site.conf"), [
    {
      search: /proxy_pass.*https.*;$/gm,
      replace: `proxy_pass    https://${jcoreSettingsData.remoteDomain};`,
    },
  ]);

  // Manage php.ini & debug setting.
  replaceInFile(
    join(jcoreRuntimeData.workDir, "php.ini"),
    [
      {
        search: /xdebug.mode=.*$/gm,
        replace: jcoreSettingsData.debug ? "xdebug.mode=develop,debug" : "xdebug.mode=off",
      },
    ],
    join(jcoreRuntimeData.workDir, ".jcore/php.ini")
  );

  // Set executable bits on scripts.
  try {
    logger.verbose("Setting executable bits on scripts.");
    execSync("chmod +x .config/scripts/*", options);
    execSync("chmod +x .config/*.sh", options);
  } catch (e) {
    logger.warn("chmod failed.");
  }

  if (jcoreSettingsData.install || install) {
    // Install npm packages.
    try {
      if (existsSync(join(jcoreRuntimeData.workDir, "package-lock.json"))) {
        logger.info("Installing npm packages from lock file.");
        execSync("npm ci --silent --no-fund", options);
      } else {
        logger.info("Installing npm packages.");
        execSync("npm i --silent --no-fund", options);
      }
    } catch (e) {
      logger.warn("Running npm failed.");
      return false;
    }

    // Install Composer packages.
    logger.info("Installing composer packages.");
    try {
      execSync("composer install --quiet", options);
    } catch (e) {
      logger.warn("Composer failed, maybe not installed.");
      return false;
    }

    // Update docker images.
    logger.info("Update Docker Images.");
    execSync("docker-compose pull", options);
  }
  return true;
}

function createEnv() {
  const values = loadJsonFile(join(jcoreRuntimeData.workDir, "env-values.json"));

  let env = "";
  for (const key in Object.assign(values, jcoreSettingsData)) {
    env += `${createEnvName(key)}="${createEnvVariable(values[key])}"\n`;
  }

  writeFileSync(join(jcoreRuntimeData.workDir, ".env"), env);
}

function createEnvName(name: string) {
  // Convert camelCase to UPPERCASE_SNAKE.
  return name.replace(/([A-Z])/g, "_$1").toUpperCase();
}

function createEnvVariable(value: configValue): string {
  if (value instanceof Array) {
    const output: Array<string> = [];
    value.forEach((row) => {
      output.push(row);
    });
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

interface searchReplace {
  search: string | RegExp;
  replace: string;
}

export function replaceInFile(file: string, replace: Array<searchReplace>, destination = "") {
  if (!destination) {
    // Default destination to same file.
    destination = file;
  }
  if (existsSync(file)) {
    let data = readFileSync(file, "utf8");
    for (const row of replace) {
      data = data.replace(row.search, row.replace);
    }
    const path = parse(destination);
    if (!existsSync(path.dir)) {
      mkdirSync(path.dir, { recursive: true });
    }
    writeFileSync(destination, data, "utf8");
  } else {
    logger.debug(`${file} doesn't exist.`);
  }
}
