#!/usr/bin/env node
import parser from "@/parser";
import { readSettings, settings } from "@/settings";
import { runCmd } from "@/cmd";
import { help, helpCmd } from "@/help";
import { logger } from "@/logger";
import { version } from "../package.json";
import semver from "semver/preload";

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
  logger.info("JCORE CLI v." + version);
  logger.info("Mode: " + settings.mode);
  logger.info("Debug: " + (settings.debug ? "On" : "Off"));
  if (semver.gt(settings.latest, version)) {
    logger.warn(`New version ${settings.latest} available. Update with command "${settings.exec} update self"`);
  }
  if (settings.inProject) {
    logger.info("Project: " + settings.name);
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
