import type { cmdData, updateOptions } from "@/types";
import { fetchVersion, getFileString, getFlagValue } from "@/utils";
import { scriptLocation } from "@/constants";
import { writeFile } from "fs/promises";
import { updateFiles } from "@/project";
import { jcoreDataData, jcoreSettingsData } from "@/settings";
import { logger } from "@/logger";
import semver from "semver/preload";
import { join } from "path";

export default function (data: cmdData) {
  const options = {
    force: getFlagValue(data, "force"),
    target: data.target,
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
      return getFileString(join(scriptLocation, jcoreSettingsData.exec));
    })
    .then((body) => writeFile(jcoreSettingsData.execPath, body))
    .then(() => {
      logger.info("JCORE CLI Updated.");
    })
    .catch((reason) => {
      logger.error(reason);
    });
}

function versionCheck(newVersion: string): Promise<string> {
  return new Promise((resolve, reject) => {
    if (semver.gt(newVersion, jcoreDataData.version)) {
      resolve(newVersion);
    }
    reject("No update available.");
  });
}
