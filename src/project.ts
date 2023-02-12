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
import { updateOptions } from "@/types";
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "fs";
import { settings } from "@/settings";
import { logger } from "@/logger";
import { execSync } from "child_process";
import { checkFolders } from "@/commands/doctor";

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

      const checksums = await loadChecksums();

      const files = [
        {
          name: "config.sh",
          force: false,
          replace: [
            {
              search: /#?NAME="[^"]*"/gm,
              replace: 'NAME="' + settings.name + '"',
            },
            {
              search: /#?THEME="[^"]*"/gm,
              replace: 'THEME="' + settings.theme + '"',
            },
            {
              search: /#?BRANCH="[^"]*"/gm,
              replace: 'BRANCH="' + settings.branch + '"',
            },
          ],
        },
        {
          name: "readme.md",
          force: false,
          replace: [
            {
              search: "# WordPress Container",
              replace: "# " + settings.name.charAt(0).toUpperCase() + settings.name.slice(1),
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
              search: /"name" *: *"[^"]*"/gm,
              replace: `"name": "${settings.name}"`,
            },
            {
              search: /"theme" *: *"[^"]*"/gm,
              replace: `"theme": "${settings.theme}"`,
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
        const matching = (await calculateChecksum(destination)) === checksums.get(file.name);
        if (matching) {
          logger.verbose("Matching Checksum: " + file.name);
        }

        if (matching || file.force || !existsSync(destination)) {
          replaceInFile(source, file.replace, destination);
          // Calculate new checksum for file.
          checksums.set(file.name, await calculateChecksum(destination));
          logger.info("Updated " + file.name);
        } else {
          logger.error("Skipping " + file.name);
        }
        // Delete the file to avoid having to exclude it from the copy.
        rmSync(source);
      }
      await saveChecksums(checksums);

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

export function finaliseProject(): boolean {
  const options = {
    cwd: settings.path,
    stdio: [0, 1, 2],
  };

  if (!checkFolders()) {
    return false;
  }

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
    return false;
  }

  // Install npm packages.
  try {
    if (existsSync(join(settings.path, "package-lock.json"))) {
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

  logger.info("Update Docker Images.");
  execSync("docker-compose pull", options);

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
  }
}
