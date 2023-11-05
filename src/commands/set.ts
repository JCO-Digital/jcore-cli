import { cmdData } from "@/types";
import { jcoreSettingsData, writeSettings } from "@/settings";
import { logger } from "@/logger";
import { getFlagValue } from "@/utils";

export function set(data: cmdData) {
  const settings: Record<string, any> = {};
  switch (data.target[0].toLowerCase()) {
    case "mode":
      if (data.target[1].toLowerCase() === "fg" || data.target[1].toLowerCase() === "foreground") {
        jcoreSettingsData.mode = "foreground";
      } else {
        jcoreSettingsData.mode = "background";
      }
      settings.mode = jcoreSettingsData.mode;
      logger.info("Mode set to " + jcoreSettingsData.mode);
      break;
    case "debug":
      jcoreSettingsData.debug = parseSetting(data.target[1]);
      settings.debug = jcoreSettingsData.debug;
      logger.info("Debug set to " + (jcoreSettingsData.debug ? "On" : "Off"));
      break;
    case "install":
      jcoreSettingsData.install = parseSetting(data.target[1]);
      settings.install = jcoreSettingsData.install;
      logger.info("Install set to " + (jcoreSettingsData.install ? "On" : "Off"));
      break;
    case "loglevel":
      if (data.target[1].match(/^[0-9]$/)) {
        jcoreSettingsData.logLevel = Number(data.target[1]);
        settings.logLevel = jcoreSettingsData.logLevel;
        logger.info(`LogLevel set to ${jcoreSettingsData.logLevel}`);
      }
      break;
  }
  writeSettings(settings, getFlagValue(data,"global"));
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
