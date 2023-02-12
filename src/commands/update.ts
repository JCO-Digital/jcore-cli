import type { cmdData, updateOptions } from "@/types";
import { fetchVersion, getFileString } from "@/utils";
import { scriptLocation } from "@/constants";
import { writeFile } from "fs/promises";
import { updateFiles } from "@/project";
import { settings } from "@/settings";
import { logger } from "@/logger";
import semver from "semver/preload";
import { join } from "path";

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
    .then((version) => {
      logger.info("Upgrading to v" + version);
      return getFileString(join(scriptLocation, settings.exec));
    })
    .then((body) => writeFile(settings.execPath, body))
    .then(() => {
      logger.info("JCORE CLI Updated.");
    })
    .catch((reason) => {
      logger.error(reason);
    });
}

function versionCheck(newVersion: string): Promise<string> {
  return new Promise((resolve, reject) => {
    if (semver.gt(newVersion, settings.version)) {
      resolve(newVersion);
    }
    reject("No update available.");
  });
}
