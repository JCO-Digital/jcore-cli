#!/usr/bin/env node
import parser from "@/parser";
import { readSettings, jcoreSettingsData, jcoreDataData, jcoreRuntimeData } from "@/settings";
import { runCmd } from "@/cmd";
import { help, helpCmd } from "@/help";
import { logger } from "@/logger";
import { version } from "../package.json";
import semver from "semver/preload";
import { init } from "@sentry/node";
import { getFlagValue } from "./utils";

// Initialize Sentry.
init({ dsn: "https://f3ab047d1d2f462eb3bb5aca4e684737@glitchtip.jco.fi/14" });

/**
 * Main init function of the application. This like all other functions expects an initialized settings object.
 */
function initCli() {
  const data = parser(process.argv);

  // Set log level from flags.
  if (getFlagValue(data, "quiet")) {
    jcoreSettingsData.logLevel = logger.levels.error;
  } else if (getFlagValue(data, "debug")) {
    jcoreSettingsData.logLevel = logger.levels.debug;
  } else if (getFlagValue(data, "verbose") || getFlagValue(data, "help")) {
    jcoreSettingsData.logLevel = logger.levels.verbose;
  }

  // Intro text.
  logger.verbose("JCORE CLI v" + version);
  logger.debug("Mode: ".padEnd(12) + jcoreSettingsData.mode);
  logger.debug("Debug: ".padEnd(12) + (jcoreSettingsData.debug ? "On" : "Off"));
  logger.debug("Install: ".padEnd(12) + (jcoreSettingsData.install ? "On" : "Off"));
  if (
    jcoreDataData.latest &&
    jcoreDataData.version &&
    semver.gt(jcoreDataData.latest, jcoreDataData.version)
  ) {
    logger.warn(`New version v${jcoreDataData.latest} available.`);
    logger.verbose(`Update with command "${jcoreRuntimeData.exec} update self"`);
  }
  if (jcoreRuntimeData.inProject) {
    logger.verbose("Project: " + jcoreSettingsData.projectName);
  }

  if (data.cmd) {
    if (getFlagValue(data, "help")) {
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
readSettings().then(initCli);
