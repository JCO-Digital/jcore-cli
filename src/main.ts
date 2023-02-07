#!/usr/bin/env node
import parser from "@/parser";
import { readSettings, settings } from "@/settings";
import { runCmd } from "@/cmd";
import { help, helpCmd } from "@/help";
import { logger } from "@/logger";
import { version } from "../package.json";

function init() {
  // Intro text.
  logger.info("JCORE CLI v." + version);
  logger.info("Mode: " + settings.mode);
  logger.info("Debug: " + (settings.debug ? "On" : "Off"));
  if (settings.inProject) {
    logger.info("Project: " + settings.name);
  }

  const data = parser(process.argv);

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
