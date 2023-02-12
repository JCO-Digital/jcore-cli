import type { cmdData, updateOptions } from "@/types";
import { join } from "path";
import { getFileString } from "@/utils";
import { scriptLocation } from "@/constants";
import { writeFile } from "fs/promises";
import { updateFiles } from "@/project";
import { settings } from "@/settings";
import { version } from "../../package.json";
import { logger } from "@/logger";
import semver from "semver/preload";

export default function (data: cmdData) {
  const options = {
    drone: data.flags.includes("force") || data.target.includes("drone"),
    package: data.flags.includes("force") || data.target.includes("package"),
    build: data.flags.includes("force") || data.target.includes("build"),
    composer: data.flags.includes("force") || data.target.includes("composer"),
    docker: data.flags.includes("force") || data.target.includes("docker"),
  } as updateOptions;

  logger.info("Updating Project");
  updateFiles(options)
    .then(() => {
      logger.info("Update Finished");
    })
    .catch((reason) => {
      logger.error(reason);
    });
}

export function selfUpdate() {
  fetchVersion()
    .then(versionCheck)
    .then((info) => {
      logger.info("Upgrading to v." + info);
      return getFileString(scriptLocation + "jcore");
    })
    .then((body) => writeFile(settings.execPath, body))
    .then(() => {
      logger.info("JCORE CLI Updated.");
    })
    .catch((reason) => {
      logger.error("Update Error");
      logger.error(reason);
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
          reject("Wrong format");
        } catch (e) {
          reject("JSON error.");
        }
      })
      .catch(() => {
        reject("Network error.");
      });
  });
}

function versionCheck(newVersion: string): Promise<string> {
  return new Promise((resolve, reject) => {
    if (semver.gt(newVersion, version)) {
      resolve(newVersion);
    }
    reject("No update available.");
  });
}
