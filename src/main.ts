#!/usr/bin/env node
import parser from "@/parser";
import { readSettings, settings } from "@/settings";
import { runCmd } from "@/cmd";
import { help, helpCmd } from "@/help";
import { logger } from "@/logger";
import { version } from "../package.json";
import semver from "semver/preload";

/**
 * Main init function of the application. This like all other functions expects an initialized settings object.
 */
function init() {
  const data = parser(process.argv);

  // Set log level from flags.
  if (data.flags.includes("quiet")) {
    settings.logLevel = logger.levels.error;
  } else if (data.flags.includes("debug")) {
    settings.logLevel = logger.levels.debug;
  } else if (data.flags.includes("verbose")) {
    settings.logLevel = logger.levels.verbose;
  }

  // Intro text.
  logger.verbose("JCORE CLI v" + version);
  logger.debug("Mode: " + settings.mode);
  logger.debug("Debug: " + (settings.debug ? "On" : "Off"));
  if (semver.gt(settings.latest, version)) {
    logger.warn(`New version v${settings.latest} available.`);
    logger.verbose(`Update with command "${settings.exec} update self"`);
  }
  if (settings.inProject) {
    logger.verbose("Project: " + settings.name);
  }

  if (data.cmd) {
    if (data.flags.includes("help")) {
      // Show help text for command.
      helpCmd(data);
    } else {
      // Run the command.
      runCmd(data);
    }
  } else {
    // Show generic help text.
    help(data);
  }
}

// Run the code.
readSettings().then(init);
