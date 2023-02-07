import type { cmdData, updateOptions } from "@/types";
import { getFileString } from "@/utils";
import { scriptLocation } from "@/constants";
import { writeFile } from "fs/promises";
import { updateFiles } from "@/project";
import { settings } from "@/settings";
import { version } from "../../package.json";
import { logger } from "@/logger";

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
  versionCheck()
    .then((info) => {
      logger.info("Upgrading to v." + info);
      getFileString(scriptLocation + "jcore")
        .then((body) => {
          writeFile(settings.execPath, body)
            .then(() => {
              logger.info("JCORE CLI Updated.");
            })
            .catch((reason) => {
              logger.error("Update Error");
              logger.error(reason);
            });
        })
        .catch(() => {
          logger.error("Can't fetch update. Check network connection.");
        });
    })
    .catch((reason) => {
      logger.error(reason);
    });
}

function versionCheck(): Promise<string> {
  return new Promise((resolv, reject) => {
    getFileString(scriptLocation + "package.json")
      .then((json) => {
        const info = JSON.parse(json);
        const oldVer = calcVersion(version);
        const newVer = calcVersion(info.version);
        if (newVer > oldVer) {
          resolv(info.version);
        } else {
          reject("No update available.");
        }
      })
      .catch(() => {
        reject("Network error.");
      });
  });
}

function calcVersion(version: string): number {
  let v = 0;
  let multi = 1;
  const parts = version.split(".");
  while (parts.length) {
    v += multi * Number(parts.pop());
    multi *= 1000;
  }
  return v;
}
