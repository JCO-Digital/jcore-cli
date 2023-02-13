import { cmdData } from "@/types";
import { settings, writeSettings } from "@/settings";
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
      settings.debug = parseSetting(data.target[1]);
      logger.info("Debug set to " + (settings.debug ? "On" : "Off"));
      break;
    case "install":
      settings.install = parseSetting(data.target[1]);
      logger.info("Install set to " + (settings.install ? "On" : "Off"));
      break;
    case "loglevel":
      if (data.target[1].match(/^[0-9]$/)) {
        settings.logLevel = Number(data.target[1]);
        logger.info(`LogLevel set to ${settings.logLevel}`);
      }
      break;
  }
  writeSettings();
}

function parseSetting(value: string): boolean {
  switch (value.toLowerCase()) {
    case "true":
    case "on":
    case "1":
      return true;
    default:
      return false;
  }
}