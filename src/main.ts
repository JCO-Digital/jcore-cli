#!/usr/bin/env node
import parser from "@/parser";
import { readSettings, jcoreSettingsData, jcoreDataData } from "@/settings";
import { runCmd } from "@/cmd";
import { help, helpCmd } from "@/help";
import { logger } from "@/logger";
import { version } from "../package.json";
import semver from "semver/preload";
import { init } from "@sentry/node";

// Initialize Sentry.
init({ dsn: "https://f3ab047d1d2f462eb3bb5aca4e684737@glitchtip.jco.fi/14" });

/**
 * Main init function of the application. This like all other functions expects an initialized settings object.
 */
function initCli() {
  const data = parser(process.argv);

  // Set log level from flags.
  if (data.flags.includes("quiet")) {
    jcoreSettingsData.logLevel = logger.levels.error;
  } else if (data.flags.includes("debug")) {
    jcoreSettingsData.logLevel = logger.levels.debug;
  } else if (data.flags.includes("verbose") || data.flags.includes("help")) {
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
    logger.verbose(`Update with command "${jcoreSettingsData.exec} update self"`);
  }
  if (jcoreSettingsData.inProject) {
    logger.verbose("Project: " + jcoreSettingsData.name);
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
readSettings().then(initCli);
