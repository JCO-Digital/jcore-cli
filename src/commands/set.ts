import { cmdData } from "@/types";
import { jcoreSettingsData, writeSettings } from "@/settings";
import { logger } from "@/logger";

export function set(data: cmdData) {
  switch (data.target[0].toLowerCase()) {
    case "mode":
      if (data.target[1].toLowerCase() === "fg" || data.target[1].toLowerCase() === "foreground") {
        jcoreSettingsData.mode = "foreground";
      } else {
        jcoreSettingsData.mode = "background";
      }
      logger.info("Mode set to " + jcoreSettingsData.mode);
      break;
    case "debug":
      jcoreSettingsData.debug = parseSetting(data.target[1]);
      logger.info("Debug set to " + (jcoreSettingsData.debug ? "On" : "Off"));
      break;
    case "install":
      jcoreSettingsData.install = parseSetting(data.target[1]);
      logger.info("Install set to " + (jcoreSettingsData.install ? "On" : "Off"));
      break;
    case "loglevel":
      if (data.target[1].match(/^[0-9]$/)) {
        jcoreSettingsData.logLevel = Number(data.target[1]);
        logger.info(`LogLevel set to ${jcoreSettingsData.logLevel}`);
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
