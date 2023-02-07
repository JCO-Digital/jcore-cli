import {
  extractArchive,
  loadChecksums,
  getFile,
  saveChecksums,
  calculateChecksum,
  mergeFiles,
} from "@/utils";
import { archiveLocation, updateFolder } from "@/constants";
import { join, parse } from "path";
import { rename } from "fs/promises";
import { updateOptions } from "@/types";
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "fs";
import { settings } from "@/settings";
import { logger } from "@/logger";
import { execSync } from "child_process";

const defaultOptions = {
  drone: false,
  package: false,
  build: false,
  composer: false,
  docker: false,
} as updateOptions;

export function updateFiles(options: updateOptions = defaultOptions) {
  const updatePath = join(settings.path, updateFolder);

  if (!settings.name || settings.path === "/") {
    return Promise.reject("Not a project.");
  }

  return getFile(archiveLocation)
    .then((buffer) => extractArchive(buffer, updatePath))
    .then(async () => {
      logger.verbose("Unzipped");

      const checksums = await loadChecksums(settings);

      const files = [
        {
          name: "config.sh",
          force: false,
          replace: [
            {
              search: /#?NAME="[^"]*"/,
              replace: 'NAME="' + settings.name + '"',
            },
            {
              search: /#?THEME="[^"]*"/,
              replace: 'THEME="' + settings.theme + '"',
            },
            {
              search: /#?BRANCH="[^"]*"/,
              replace: 'BRANCH="' + settings.branch + '"',
            },
          ],
        },
        {
          name: ".drone.yml",
          force: options.drone ?? false,
          source: "project.drone.yml",
          replace: [
            {
              search: "wp-content/themes/projectname",
              replace: "wp-content/themes/" + settings.theme,
            },
          ],
        },
        {
          name: "package.json",
          force: options.package ?? false,
          replace: [
            {
              search: /"name":.*$/,
              replace: '"name": "' + settings.name + '"',
            },
            {
              search: /"theme":.*$/,
              replace: '"theme": "' + settings.theme + '"',
            },
          ],
        },
        {
          name: "build.mjs",
          force: options.build ?? false,
          replace: [],
        },
        {
          name: "composer.json",
          force: options.composer ?? false,
          replace: [],
        },
        {
          name: "docker-compose.yml",
          force: options.docker ?? false,
          replace: [
            {
              search: "- docker.localhost",
              replace: "- " + settings.name + ".localhost",
            },
          ],
        },
      ];

      for (const file of files) {
        const source = join(updatePath, file.source ?? file.name);
        const destination = join(settings.path, file.name);
        // Check if file in project has been modified, and thus automatic update should be skipped.
        const matching =
          (await calculateChecksum(destination)) === checksums.get(file.name);
        if (matching) {
          logger.verbose("Matching Checksum: " + file.name);
        }
        await shouldWrite(destination, matching || file.force)
          .then((destination) => moveFile(destination, source))
          .then(async () => {
            replaceInFile(destination, file.replace);
            // Calculate new checksum for file.
            checksums.set(file.name, await calculateChecksum(destination));
            logger.info("Updated " + file.name);
          })
          .catch(() => {
            // Delete the skipped file to avoid having to exclude it from the copy.
            rmSync(source);
            logger.error("Skipping " + file.name);
          });
      }
      await saveChecksums(settings, checksums);

      // Clean up legacy folders.
      rmSync(join(settings.path, "config"), { recursive: true, force: true });
      rmSync(join(settings.path, "provisioning"), {
        recursive: true,
        force: true,
      });

      // Remove old config folder.
      rmSync(join(settings.path, ".config"), { recursive: true, force: true });

      // Move updated project files to project folder.
      mergeFiles(updatePath, settings.path);

      // Clean up remaining files.
      rmSync(updatePath, { recursive: true, force: true });
    })
    .catch((reason) => Promise.reject("Unable to extract file " + reason));
}

export function finaliseProject() {
  const options = {
    cwd: settings.path,
    stdio: [0, 1, 2],
  };

  // Set nginx proxy pass.
  replaceInFile(join(settings.path, ".config/nginx/site.conf"), [
    {
      search: /proxy_pass.*https.*;$/gm,
      replace: "proxy_pass    https://" + settings.domain + ";",
    },
  ]);

  // Manage php.ini & debug setting.
  const replace = [];
  if (settings.debug) {
    replace.push({
      search: /xdebug.mode=.*$/gm,
      replace: "xdebug.mode=develop,debug",
    });
  }
  replaceInFile(
    join(settings.path, ".config/php.ini"),
    replace,
    join(settings.path, ".jcore/php.ini")
  );

  // Set executable bits on scripts.
  try {
    logger.verbose("Setting executable bits on scripts.");
    execSync("chmod +x .config/scripts/*", options);
    execSync("chmod +x .config/*.sh", options);
  } catch (e) {
    logger.warn("chmod failed, maybe Windows environment.");
  }

  // Install npm packages.
  if (existsSync(join(settings.path, "package-lock.json"))) {
    logger.info("Installing npm packages from lock file.");
    execSync("npm ci --silent --no-fund", options);
  } else {
    logger.info("Installing npm packages.");
    execSync("npm i --silent --no-fund", options);
  }

  // Install Composer packages.
  logger.info("Installing composer packages.");
  try {
    execSync("composer install --quiet", options);
  } catch (e) {
    logger.warn("Composer failed, maybe not installed.");
  }

  logger.info("Update Docker Images.");
  execSync("docker-compose pull", options);
}

function shouldWrite(
  file: string,
  condition = false
): Promise<string> {
  return new Promise((resolve, reject) => {
    if (condition) {
      resolve(file);
    }
    if (existsSync(file)) {
      reject("File exists: " + file);
    } else {
      resolve(file);
    }
  });
}

function moveFile(destination: string, source: string): Promise<string> {
  try {
    return rename(source, destination).then(() => Promise.resolve(destination));
  } catch {
    return Promise.reject(
      "Unable to move file " + source + " to " + destination
    );
  }
}

interface searchReplace {
  search: string|RegExp,
  replace: string,
}

function replaceInFile(
  file: string,
  replace: Array<searchReplace>,
  destination = ""
) {
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
