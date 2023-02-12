import type { cmdData } from "@/types";
import update, { selfUpdate } from "@/commands/update";
import { start, stop, pull } from "@/commands/run";
import { isProject } from "@/utils";
import { helpCmd } from "@/help";
import { copyChildTheme, createProject } from "@/commands/create";
import { cloneProject } from "@/commands/clone";
import { set } from "@/commands/set";
import { doctor } from "@/commands/doctor";
import { settings, writeSettings } from "@/settings";
import { logger } from "@/logger";
import { listChecksums, setChecksum } from "@/commands/checksum";

export function runCmd(data: cmdData) {
  switch (data.cmd) {
    case "checksum":
      if (isProject()) {
        switch (data.target.shift()) {
          case "list":
            listChecksums();
            break;
          case "set":
            setChecksum(data.target);
            break;
          default:
            helpCmd(data, false);
            break;
        }
      }
      break;
    case "child":
      if (isProject()) {
        // Create Child Theme.
        if (data.target[0]) {
          if (copyChildTheme(data.target.join(" "))) {
            // Save settings.
            writeSettings();
            logger.info(`Theme ${settings.theme} created.`);
          } else {
            logger.error("Theme creation failed!");
          }
        } else {
          helpCmd(data, false);
        }
      }
      break;
    case "clean":
      // TODO Clean
      break;
    case "clone":
      if (isProject(false)) {
        // Clone project.
        if (data.target[0]) {
          cloneProject(data);
        } else {
          helpCmd(data, false);
        }
      }
      break;
    case "doctor":
      doctor();
      break;
    case "init":
      if (isProject(false)) {
        // Create new project.
        if (data.target[0]) {
          createProject(data);
        } else {
          helpCmd(data, false);
        }
      }
      break;
    case "pull":
      if (isProject()) {
        // Pull data from upstream.
        pull(data);
      }
      break;
    case "run":
      // TODO run
      break;
    case "set":
      if (data.target.length > 1) {
        set(data);
      } else {
        helpCmd(data, false);
      }
      break;
    case "shell":
      // TODO shell
      break;
    case "start":
      if (isProject()) {
        // Start the project.
        start();
      }
      break;
    case "stop":
      if (isProject()) {
        // Start the project.
        stop();
      }
      break;
    case "update":
      if (data.target.includes("self")) {
        // Update self.
        selfUpdate();
      } else {
        if (isProject()) {
          // Update project.
          update(data);
        }
      }
      break;
  }
}
