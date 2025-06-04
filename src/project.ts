import { execSync } from "child_process";
import {
  existsSync,
  lstatSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  renameSync,
  rmSync,
  writeFileSync,
} from "fs";
import { join, parse } from "path";
import { checkDocker, checkFolders } from "@/commands/doctor";
import { archiveLocation, updateFolder } from "@/constants";
import { logger } from "@/logger";
import { jcoreRuntimeData, jcoreSettingsData } from "@/settings";
import {
  calculateChecksum,
  compareChecksum,
  loadChecksums,
  saveChecksums,
  updateChecksum,
} from "@/checksums";
import { createEnv } from "./env";
import { extractArchive, getFile } from "@/fileHelpers";
import { getFlag, getSetupFolder } from "@/utils";
import { errorHandler } from "@/utils";

export async function updateFiles(include: Array<string> = []) {
  const updatePath = join(jcoreRuntimeData.workDir, updateFolder);

  if (!jcoreSettingsData.projectName || jcoreRuntimeData.workDir === "/") {
    return Promise.reject("Not a project.");
  }

  try {
    const buffer = await getFile(archiveLocation);
    await extractArchive(buffer, updatePath);
    logger.verbose("Unzipped");

    const checksums = loadChecksums();

    for (const file of ["Vagrantfile", ".vagrant", "config", "provisioning"]) {
      const filePath = join(jcoreRuntimeData.workDir, file);
      if (existsSync(filePath)) {
        logger.info(`Cleaning up legacy files / folders: ${file}`);
        rmSync(filePath, {
          recursive: true,
          force: true,
        });
      }
    }

    if (getFlag("force") && include.length === 0) {
      // Remove old setup folder if updating all files with "force" flag.
      logger.info("Remove old setup folder.");
      rmSync(join(jcoreRuntimeData.workDir, ".config"), {
        recursive: true,
        force: true,
      });
    }

    logger.debug("Move updated project files to project folder.");
    moveFiles(updatePath, jcoreRuntimeData.workDir, checksums, {
      include,
      exclude: ["templates", "package.json"],
    });
    logger.debug("Move template files to project folder.");
    moveFiles(
      join(updatePath, "templates", jcoreSettingsData.template),
      jcoreRuntimeData.workDir,
      checksums,
      { include },
    );
    // Save new checksums.
    saveChecksums(checksums);

    logger.verbose("Clean up remaining files.");
    rmSync(updatePath, { recursive: true, force: true });

    // Finalize files again.
    finalizeProject(false, false);
  } catch (reason) {
    throw `Unable to extract file ${reason}`;
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
  options: fileOptions = {},
) {
  const opt = Object.assign(
    {
      path: "",
      include: [],
      exclude: [],
    },
    options,
  );

  if (!existsSync(join(destinationDir, opt.path))) {
    // Create target if not exists.
    logger.verbose(`Creating target folder: ${destinationDir}`);
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
      moveFiles(sourceDir, destinationDir, checksums, {
        ...opt,
        path: filePath,
      });
    } else {
      // Current path is a file.
      if (opt.include.length === 0 || opt.include.includes(filePath)) {
        logger.debug(`${filePath} is a file.`);
        // Only run if no target given, or file is in target list.
        const fileInfo = getFileInfo(
          destinationDir,
          filePath,
          checksums,
          options.include,
        );

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
          logger.warn(`Skipping ${fileInfo.target}`);
        }
      }
    }
  }
}

function getFileInfo(
  path: string,
  file: string,
  checksums: Map<string, string>,
  include: Array<string> = [],
) {
  const files: Record<string, object> = {
    "readme.md": {
      force: false,
      checksum: true,
      replace: [
        {
          search: "# WordPress Container",
          replace: `# ${jcoreSettingsData.projectName
            .charAt(0)
            .toUpperCase()}${jcoreSettingsData.projectName.slice(1)}`,
        },
      ],
    },
    ".drone.yml": {
      force: true,
      checksum: true,
      replace: [
        {
          search: "wp-content/themes/projectname",
          replace: join("wp-content/themes", jcoreSettingsData.theme),
        },
      ],
    },
    "package.json": {
      force: true,
      checksum: true,
      replace: [
        {
          search: /"name": "[^"]+"/,
          replace: `"name": "${jcoreSettingsData.projectName}"`,
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
    Makefile: {
      force: true,
      checksum: true,
      replace: [
        {
          search: /theme :=.*/,
          replace: `theme := ${join("wp-content/themes", jcoreSettingsData.theme)}`,
        },
      ],
    },

    ".github/workflows/deploy.yml": {
      force: true,
      checksum: true,
      replace: [
        {
          search: /wp-content\/themes\/[^/]+\/:wp-content\/themes\/[^/]+\/,/,
          replace: `${join("wp-content/themes", jcoreSettingsData.theme)}/:${join("wp-content/themes", jcoreSettingsData.theme)}/`,
        },
      ],
    },
  };

  const fileInfo = Object.assign(
    {
      target: file, // Destination has different name.
      force: true, // Allow overwrite with force flag.
      checksum: false, // If false allow overwrite on missing checksum.
      replace: [], // String replace in file.
      overwrite: false, // Flag to tell "moveFiles" to overwrite file.
    },
    files[file] ?? {},
  );

  // Overwrite all targeted files.
  if (include.includes(fileInfo.target)) {
    logger.debug(`File ${fileInfo.target} targeted, overwriting.`);
    fileInfo.overwrite = true;
    return fileInfo;
  }

  // Check if force flag is set and file is allowed to be forced or targeted.
  if (getFlag("force") && fileInfo.force) {
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

  // Calculate the checksum normally and compare it.
  if (calculateChecksum(destination) === checksum) {
    // Checksum matches.
    logger.debug(`Checksum matches for ${fileInfo.target}.`);
    fileInfo.overwrite = true;
  }
  return fileInfo;
}

export function finalizeProject(install = true, pull = true): boolean {
  const options = {
    cwd: jcoreRuntimeData.workDir,
    stdio: [0, 1, 2],
  };

  if (!checkFolders() || !checkDocker()) {
    return false;
  }

  // Write the .env file.
  createEnv();

  // Set nginx proxy pass.
  const siteConf = getSetupFolder("nginx/site.conf", false, true);
  const checksum = compareChecksum(siteConf, false);
  replaceInFile(join(jcoreRuntimeData.workDir, siteConf), [
    {
      search: /proxy_pass(\s*)https[^;]*;/gm,
      replace: `proxy_pass$1https://${jcoreSettingsData.remoteDomain};`,
    },
  ]);
  if (checksum) {
    updateChecksum(siteConf);
  }

  // Manage php.ini & debug setting.
  replaceInFile(
    join(jcoreRuntimeData.workDir, "php.ini"),
    [
      {
        search: /xdebug.mode=.*$/gm,
        replace: jcoreSettingsData.debug
          ? "xdebug.mode=develop,debug"
          : "xdebug.mode=off",
      },
    ],
    join(jcoreRuntimeData.workDir, ".jcore/php.ini"),
  );

  // Set executable bits on scripts.
  try {
    logger.verbose("Setting executable bits on scripts.");
    execSync("chmod +x .config/scripts/*", options);
    execSync("chmod +x .config/*.sh", options);
  } catch (error) {
    errorHandler(error, "chmod failed");
  }

  if (jcoreSettingsData.install || install) {
    if (existsSync(join(jcoreRuntimeData.workDir, "Makefile"))) {
      // If Makefile exists, run make only.
      try {
        logger.info("Makefile found, running 'make install'.");
        execSync("make install", options);
      } catch (error) {
        errorHandler(error, "Running make failed");
        return false;
      }
    } else {
      // Install npm packages.
      if (existsSync(join(jcoreRuntimeData.workDir, "package.json"))) {
        try {
          execSync("pnpm --version || corepack enable", options);
          if (
            existsSync(join(jcoreRuntimeData.workDir, "package-lock.json")) &&
            !existsSync(join(jcoreRuntimeData.workDir, "pnpm-lock.yaml"))
          ) {
            logger.info("Installing npm packages from lock file.");
            execSync("npm ci --silent --no-fund", options);
          } else {
            logger.info("Installing pnpm packages.");
            execSync("pnpm i", options);
          }
        } catch (error) {
          errorHandler(error, "Running pnpm failed.");
          return false;
        }
      }

      if (existsSync(join(jcoreRuntimeData.workDir, "composer.json"))) {
        // Install Composer packages.
        logger.info("Installing composer packages.");
        try {
          execSync("composer install --quiet", options);
        } catch (error) {
          errorHandler(error, "Composer failed, maybe not installed.");
          return false;
        }
      }

      // Update docker images.
      if (pull) {
        logger.info("Update Docker Images.");
        execSync("docker compose pull", options);
      }
    }
  }
  return true;
}

interface searchReplace {
  search: string | RegExp;
  replace: string;
}

export function replaceInFile(
  file: string,
  replace: Array<searchReplace>,
  destination = file,
) {
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
