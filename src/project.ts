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
import { updateOptions } from "@/types";
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
import { jcoreSettingsData } from "@/settings";
import { logger } from "@/logger";
import { execSync } from "child_process";
import { checkFolders } from "@/commands/doctor";
import { string } from "zod";

const defaultOptions = {
  force: false,
  target: [],
} as updateOptions;

export async function updateFiles(options: updateOptions = defaultOptions) {
  const updatePath = join(jcoreSettingsData.path, updateFolder);

  if (!jcoreSettingsData.name || jcoreSettingsData.path === "/") {
    return Promise.reject("Not a project.");
  }

  try {
    let buffer = await getFile(archiveLocation);
    await extractArchive(buffer, updatePath);
    logger.verbose("Unzipped");

    const checksums = loadChecksums();

    console.debug(options);

    const files = {
      "config.sh": {
        force: false,
        checksum: true,
        replace: [
          {
            search: /#?NAME="[^"]*"/gm,
            replace: 'NAME="' + jcoreSettingsData.name + '"',
          },
          {
            search: /#?THEME="[^"]*"/gm,
            replace: 'THEME="' + jcoreSettingsData.theme + '"',
          },
          {
            search: /#?BRANCH="[^"]*"/gm,
            replace: 'BRANCH="' + jcoreSettingsData.branch + '"',
          },
        ],
      },
      "readme.md": {
        force: true,
        checksum: true,
        replace: [
          {
            search: "# WordPress Container",
            replace:
              "# " +
              jcoreSettingsData.name.charAt(0).toUpperCase() +
              jcoreSettingsData.name.slice(1),
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
            search: /"name" *: *"[^"]*"/gm,
            replace: `"name": "${jcoreSettingsData.name}"`,
          },
          {
            search: /"theme" *: *"[^"]*"/gm,
            replace: `"theme": "${jcoreSettingsData.theme}"`,
          },
        ],
      },
      "docker-compose.yml": {
        force: true,
        checksum: true,
        replace: [
          {
            search: "- docker.localhost",
            replace: "- " + jcoreSettingsData.name + ".localhost",
          },
        ],
      },
      "composer.json": {
        force: true,
        checksum: true,
      },
    };

    /*
    for (const file of files) {
      const source = join(updatePath, file.source ?? file.name);
      const destination = join(jcoreSettingsData.path, file.name);
      // Check if file in project has been modified, and thus automatic update should be skipped.
      const matching = calculateChecksum(destination) === checksums.get(file.name);
      if (matching) {
        logger.verbose("Matching Checksum: " + file.name);
      }

      if (matching || file.force || !existsSync(destination)) {
        replaceInFile(source, file.replace, destination);
        // Calculate new checksum for file.
        checksums.set(file.name, calculateChecksum(destination));
        logger.verbose("Updated " + file.name);
      } else {
        logger.error("Skipping " + file.name);
      }
      // Delete the file to avoid having to exclude it from the copy.
      rmSync(source);
    }
     */

    logger.debug("Cleaning up legacy folders.");
    rmSync(join(jcoreSettingsData.path, "config"), { recursive: true, force: true });
    rmSync(join(jcoreSettingsData.path, "provisioning"), {
      recursive: true,
      force: true,
    });

    if (options.target.length === 0) {
      // Remove old setup folder if updating all files.
      logger.verbose("Remove old setup folder.");
      rmSync(join(jcoreSettingsData.path, ".config"), { recursive: true, force: true });
    }

    // Move updated project files to project folder.
    moveFiles(updatePath, jcoreSettingsData.path, "", checksums, files);
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
 * @param files
 */
export function moveFiles(
  sourceDir: string,
  destinationDir: string,
  path: string,
  checksums: Map<string, string>,
  files: Object
) {
  console.debug(files);
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
      moveFiles(sourceDir, destinationDir, filePath, checksums, files);
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

export function finalizeProject(install = true): boolean {
  const options = {
    cwd: jcoreSettingsData.path,
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
      replace: "proxy_pass    https://" + jcoreSettingsData.domain + ";",
    },
  ]);

  // Manage php.ini & debug setting.
  const replace = [];
  if (jcoreSettingsData.debug) {
    replace.push({
      search: /xdebug.mode=.*$/gm,
      replace: "xdebug.mode=develop,debug",
    });
  }
  replaceInFile(
    join(jcoreSettingsData.path, "php.ini"),
    replace,
    join(jcoreSettingsData.path, ".jcore/php.ini")
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
      if (existsSync(join(jcoreSettingsData.path, "package-lock.json"))) {
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
  const values = loadJsonFile(join(jcoreSettingsData.path, "defaults.json"));

  values.project_name = jcoreSettingsData.name;
  values.upstream_domain = jcoreSettingsData.domain;
  values.local_domain = jcoreSettingsData.local;
  values.plugin_install = jcoreSettingsData.plugins;
  values.domains = jcoreSettingsData.domains;
  values.replace = jcoreSettingsData.replace;
  values.remotehost = jcoreSettingsData.remoteHost ?? `${jcoreSettingsData.name}@${jcoreSettingsData.name}.ssh.wpengine.net`;
  values.remotepath = jcoreSettingsData.remotePath ?? `/sites/${jcoreSettingsData.name}`;

  console.log(values);
  console.log(jcoreSettingsData);

  let env = "";
  for (let key in values) {
    const value = values[key];
    if (value instanceof Array) {
      env += `${key.toUpperCase()}=${createEnvVariable(value)}\n`;
    } else {
      env += `${key.toUpperCase()}=${value}\n`;
    }
  }

  writeFileSync(join(jcoreSettingsData.path, ".env"), env);
}

function createEnvVariable(value: Array<Array<string> | string>): string {
  const output: any[] = [];
  if (value.length && typeof value[0] === "string") {
    output.push(...value);
  } else {
    value.forEach((row) => {
      if (row instanceof Array) {
        output.push(row.join(","));
      }
    });
  }
  return output.join(" ");
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
  }
}
