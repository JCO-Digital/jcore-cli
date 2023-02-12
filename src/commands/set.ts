import { cmdData } from "@/types";
import { settings, writeGlobalSettings } from "@/settings";
import { logger } from "@/logger";

export function set(data: cmdData) {
  switch (data.target[0].toLowerCase()) {
    case "mode":
      if (data.target[1].toLowerCase() === "fg" || data.target[1].toLowerCase() === "foreground") {
        settings.mode = "foreground";
      } else {
        settings.mode = "background";
      }
      logger.info("Mode set to " + settings.mode);
      break;
    case "debug":
      if (data.target[1].toLowerCase() === "on" || data.target[1] === "1") {
        settings.debug = 1;
      } else {
        settings.debug = 0;
      }
      logger.info("Debug set to " + (settings.debug ? "On" : "Off"));
      break;
    case "loglevel":
      if (data.target[1].match(/^[0-9]$/)) {
        settings.logLevel = Number(data.target[1]);
        logger.info(`LogLevel set to ${settings.logLevel}`);
      }
      break;
  }
  writeGlobalSettings();
}
