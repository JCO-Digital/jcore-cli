import {
  extractArchive,
  loadChecksums,
  getFile,
  saveChecksums,
  calculateChecksum,
  getSetupFolder,
  createEnv,
  getFlag,
} from "@/utils";
import { archiveLocation, updateFolder } from "@/constants";
import { join, parse } from "path";
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
import { jcoreCmdData } from "@/parser";

export async function updateFiles() {
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

    if (jcoreCmdData.target.length === 0) {
      // Remove old setup folder if updating all files.
      logger.verbose("Remove old setup folder.");
      rmSync(join(jcoreRuntimeData.workDir, ".config"), { recursive: true, force: true });
    }

    logger.debug("Move updated project files to project folder.");
    moveFiles(updatePath, jcoreRuntimeData.workDir, checksums, {
      exclude: ["templates", "package.json"],
    });
    logger.debug("Move template files to project folder.");
    moveFiles(
      join(updatePath, "templates", jcoreSettingsData.template),
      jcoreRuntimeData.workDir,
      checksums
    );
    // Save new checksums.
    saveChecksums(checksums);

    logger.verbose("Clean up remaining files.");
    rmSync(updatePath, { recursive: true, force: true });
  } catch (reason) {
    throw "Unable to extract file " + reason;
  }
}

interface fileOptions {
  path?: string;
  exclude?: Array<string>;
  include?: Array<string>;
}

/**
 * Moves source to destination, merging with existing structure, overwriting files with checksum validation.
 * @param sourceDir Source Folder
 * @param destinationDir Destination Folder
 * @param checksums File checksum map, or null to skip checksums.
 * @param options File options: path, exclude, include.
 */
export function moveFiles(
  sourceDir: string,
  destinationDir: string,
  checksums: Map<string, string>,
  options: fileOptions = {}
) {
  const opt = Object.assign(
    {
      path: "",
      include: [],
      exclude: [],
    },
    options
  );

  if (!existsSync(join(destinationDir, opt.path))) {
    // Create target if not exists.
    logger.verbose("Creating target folder: " + destinationDir);
    mkdirSync(join(destinationDir, opt.path), { recursive: true });
  }
  for (const file of readdirSync(join(sourceDir, opt.path))) {
    const filePath = join(opt.path, file);
    if (file === ".git" || opt.exclude.includes(filePath)) {
      // Skip .git folder.
      logger.debug(`Excluding ${filePath}.`);
      continue;
    }
    if (lstatSync(join(sourceDir, filePath)).isDirectory()) {
      logger.debug(`${filePath} is a folder.`);
      // Current path is a folder.
      if (!existsSync(join(destinationDir, filePath))) {
        logger.debug(`Creating ${filePath}.`);
        // Create destination folder if it doesn't exist.
        mkdirSync(join(destinationDir, filePath));
      }
      // Merge files in folder.
      moveFiles(sourceDir, destinationDir, checksums, { ...opt, path: filePath });
    } else {
      // Current path is a file.
      if (opt.include.length === 0 || opt.include.includes(filePath)) {
        logger.debug(`${filePath} is a file.`);
        // Only run if no target given, or file is in target list.
        const fileInfo = getFileInfo(destinationDir, filePath, checksums);

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

function getFileInfo(path: string, file: string, checksums: Map<string, string>) {
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
    ".drone.yml": {
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
          search: /"name": "[^"]+"/,
          replace: `"name": "${jcoreSettingsData.projectName}",`,
        },
      ],
    },
    "docker-compose.yml": {
      force: true,
      checksum: true,
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
  if (getFlag("force") && (fileInfo.force || jcoreCmdData.target.includes(fileInfo.target))) {
    logger.debug(`Force overwrite ${fileInfo.target}`);
    fileInfo.overwrite = true;
    return fileInfo;
  }

  // If destination doesn't exist, set overwrite.
  const destination = join(path, fileInfo.target);
  if (!existsSync(destination)) {
    logger.debug(`File ${fileInfo.target} doesn't exist, writing.`);
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
      search: /proxy_pass(\s*)https[^;]*;/gm,
      replace: `proxy_pass$1https://${jcoreSettingsData.remoteDomain};`,
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
    if (existsSync(join(jcoreRuntimeData.workDir, "package.json"))) {
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
    }

    if (existsSync(join(jcoreRuntimeData.workDir, "composer.json")))
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
    execSync("docker compose pull", options);
  }
  return true;
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
