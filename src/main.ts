#!/usr/bin/env node
import { runCmd } from "@/cmd";
import { help, helpCmd } from "@/help";
import { logger } from "@/logger";
import { jcoreCmdData } from "@/parser";
import {
  jcoreDataData,
  jcoreRuntimeData,
  jcoreSettingsData,
  readSettings,
} from "@/settings";
import semver from "semver/preload";
import { version } from "../package.json";
import { getFlag } from "./utils";
import { exit } from "node:process";

/**
 * Main init function of the application. This like all other functions expects an initialized settings object.
 */
function initCli() {
  // Debug info text.
  logger.debug("Version: ".padEnd(12) + version);
  logger.debug("Mode: ".padEnd(12) + jcoreSettingsData.mode);
  logger.debug("Debug: ".padEnd(12) + (jcoreSettingsData.debug ? "On" : "Off"));
  logger.debug(
    "Install: ".padEnd(12) + (jcoreSettingsData.install ? "On" : "Off"),
  );
  if (
    jcoreDataData.latest &&
    jcoreDataData.version &&
    semver.gt(jcoreDataData.latest, jcoreDataData.version)
  ) {
    logger.warn(`New version v${jcoreDataData.latest} available.`);
    logger.verbose(
      `Update with command "${jcoreRuntimeData.exec} update self"`,
    );
  }
  if (jcoreRuntimeData.inProject) {
    logger.verbose(`Project: ${jcoreSettingsData.projectName}`);
  }

  if (
    jcoreSettingsData.pluginInstall === "composer" &&
    !getFlag("letmebreakthings")
  ) {
    logger.error(
      "Composer plugin mode is deprecated, and will break mainWP / WP-cli workflow.",
    );
    logger.warn(
      'Please change the mode to "remote", and clean up the composer.json file, and remove plugin push from deploy file.',
    );
    exit(1);
  }

  if (jcoreCmdData.cmd) {
    if (getFlag("help")) {
      // Show help text for command.
      helpCmd();
    } else {
      // Run the command.
      runCmd();
    }
  } else {
    logger.info(`JCORE CLI v${version}`);
    // Show generic help text.
    help();
  }
}

// Run the code.
readSettings().then(initCli);
