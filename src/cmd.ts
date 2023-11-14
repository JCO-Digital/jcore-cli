import type { cmdData, jcoreProject } from "@/types";
import update, { selfUpdate } from "@/commands/update";
import {
  attach,
  cleanAll,
  cleanDocker,
  cleanProject,
  getRunning,
  isRunning,
  pull,
  runCommand,
  start,
  stop,
} from "@/commands/run";
import { getFlagValue, isProject } from "@/utils";
import { helpCmd } from "@/help";
import { copyChildTheme, createProject } from "@/commands/create";
import { cloneProject } from "@/commands/clone";
import { config } from "@/commands/config";
import { doctor } from "@/commands/doctor";
import { jcoreRuntimeData, jcoreSettingsData, setConfigValue } from "@/settings";
import { logger } from "@/logger";
import { listChecksums, setChecksum } from "@/commands/checksum";
import { configScope } from "@/constants";

/**
 * Invokes functions for all the different commands. Sanity checking should be done here,
 * like if the command needs to be in a project to run.
 * @param data
 */
export function runCmd(data: cmdData): void {
  switch (data.cmd) {
    case "attach":
      if (isProject() && isRunning()) {
        attach(data);
      }
      break;
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
            setConfigValue("theme", jcoreSettingsData.theme, configScope.PROJECT);
            logger.info(`Theme ${jcoreSettingsData.theme} created.`);
          } else {
            logger.error("Theme creation failed!");
          }
        } else {
          helpCmd(data, false);
        }
      }
      break;
    case "clean":
      if (data.target.includes("all")) {
        cleanAll();
      } else if (data.target.includes("docker")) {
        cleanDocker();
      } else if (isProject()) {
        // Clean
        // TODO Use actual projects instead of jcoreSettings.
        const project = {
          name: jcoreSettingsData.projectName,
          path: jcoreRuntimeData.workDir,
          running: false,
        } as jcoreProject;

        cleanProject(project);
      }
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
    case "config":
      if (data.target.length > 0) {
        config(data);
      } else {
        helpCmd(data, false);
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
      if (isProject() && isRunning()) {
        // Pull data from upstream.
        pull(data);
      }
      break;
    case "run":
      if (isProject() && isRunning()) {
        if (data.target.length > 0) {
          // Run command.
          runCommand(data.target.join(" "));
        } else {
          helpCmd(data, false);
        }
      }
      break;
    case "shell":
      if (isProject() && isRunning()) {
        // Open a shell.
        runCommand("/bin/bash");
      }
      break;
    case "status":
      if (
        getRunning().map((project) => {
          // Stop running projects.
          logger.info(`Project ${project.name} running.`);
        }).length === 0
      ) {
        logger.info("No running projects.");
      }
      break;
    case "start":
      if (isProject()) {
        if (!isRunning(false)) {
          // Start the project.
          start(data);
        } else if (getFlagValue(data, "force")) {
          // Stop everything and then start.
          getRunning().forEach((project) => {
            // Stop running projects.
            logger.info(`Stopping ${project.name}.`);
            stop(project.path);
          });
          // Start the project.
          start(data);
        }
      }
      break;
    case "stop":
      if (
        getRunning().map((project) => {
          // Stop running projects.
          logger.info(`Stopping ${project.name}.`);
          stop(project.path);
        }).length === 0
      ) {
        logger.info("No running projects.");
      }
      break;
    case "update":
      if (data.target.includes("self")) {
        // Update self.
        selfUpdate(data);
      } else {
        if (isProject()) {
          // Update project.
          update(data);
        }
      }
      break;
  }
}
