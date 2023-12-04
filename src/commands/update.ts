import { fetchVersion, getFileString, getFlag } from "@/utils";
import { scriptLocation, scriptName } from "@/constants";
import { writeFile } from "fs/promises";
import { updateFiles } from "@/project";
import { jcoreDataData, jcoreRuntimeData } from "@/settings";
import { logger } from "@/logger";
import semver from "semver/preload";
import { join } from "path";
import { jcoreCmdData } from "@/parser";

export default function () {
  logger.info("Updating Project");
  updateFiles(jcoreCmdData.target)
    .then(() => {
      logger.info("Update Finished");
    })
    .catch((reason) => {
      logger.error(reason);
    });
}

export function selfUpdate() {
  const force = getFlag("force");
  fetchVersion()
    .then((version) => versionCheck(version, force))
    .then((version) => {
      logger.info("Upgrading to v" + version);
      return getFileString(join(scriptLocation, scriptName));
    })
    .then((body) => writeFile(jcoreRuntimeData.execPath, body))
    .then(() => {
      logger.info("JCORE CLI Updated.");
    })
    .catch((reason) => {
      logger.error(reason);
    });
}

function versionCheck(newVersion: string, force = false): Promise<string> {
  return new Promise((resolve, reject) => {
    if (force || semver.gt(newVersion, jcoreDataData.version)) {
      resolve(newVersion);
    }
    reject("No update available.");
  });
}
