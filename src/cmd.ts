import { listChecksums, setChecksum } from "@/commands/checksum";
import { cloneProject } from "@/commands/clone";
import { config } from "@/commands/config";
import { copyChildTheme, queryProject } from "@/commands/create";
import { doctor } from "@/commands/doctor";
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
import update, { selfUpdate } from "@/commands/update";
import { configScope } from "@/constants";
import { helpCmd } from "@/help";
import { logger } from "@/logger";
import { jcoreCmdData } from "@/parser";
import {
  jcoreRuntimeData,
  jcoreSettingsData,
  setConfigValue,
} from "@/settings";
import type { jcoreProject } from "@/types";
import { getFlag, isProject } from "@/utils";

/**
 * Invokes functions for all the different commands. Sanity checking should be done here,
 * like if the command needs to be in a project to run.
 */
export function runCmd(): void {
  switch (jcoreCmdData.cmd) {
    case "attach":
      if (isProject() && isRunning()) {
        attach();
      }
      break;
    case "checksum":
      if (isProject()) {
        switch (jcoreCmdData.target.shift()) {
          case "list":
            listChecksums();
            break;
          case "set":
            setChecksum(jcoreCmdData.target);
            break;
          default:
            helpCmd(false);
            break;
        }
      }
      break;
    case "child":
      if (isProject()) {
        // Create Child Theme.
        if (jcoreCmdData.target[0]) {
          if (copyChildTheme(jcoreCmdData.target.join(" "))) {
            // Save settings.
            setConfigValue(
              "theme",
              jcoreSettingsData.theme,
              configScope.PROJECT,
            );
            logger.info(`Theme ${jcoreSettingsData.theme} created.`);
          } else {
            logger.error("Theme creation failed!");
          }
        } else {
          helpCmd(false);
        }
      }
      break;
    case "clean":
      if (jcoreCmdData.target.includes("all")) {
        cleanAll();
      } else if (jcoreCmdData.target.includes("docker")) {
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
        if (jcoreCmdData.target[0]) {
          cloneProject();
        } else {
          helpCmd(false);
        }
      }
      break;
    case "config":
      if (jcoreCmdData.target.length > 0) {
        config();
      } else {
        helpCmd(false);
      }
      break;
    case "doctor":
      doctor();
      break;
    case "init":
      if (isProject(false)) {
        // Create new project.
        queryProject().catch((reason) => {
          logger.error(reason.toString());
        });
      }
      break;
    case "pull":
      if (isProject() && isRunning()) {
        // Pull data from upstream.
        pull();
      }
      break;
    case "run":
      if (isProject() && isRunning()) {
        if (jcoreCmdData.target.length > 0) {
          // Run command.
          runCommand(jcoreCmdData.target.join(" "));
        } else {
          helpCmd(false);
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
          start();
        } else if (getFlag("force")) {
          // Stop everything and then start.
          for (const project of getRunning()) {
            // Stop running projects.
            logger.info(`Stopping ${project.name}.`);
            stop(project.path);
          }
          // Start the project.
          start();
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
      if (jcoreCmdData.target.includes("self")) {
        // Update self.
        selfUpdate();
      } else {
        if (isProject()) {
          // Update project.
          update();
        }
      }
      break;
  }
}
