import { cmdData } from "@/types";
import { jcoreSettingsData, writeSettings } from "@/settings";
import { logger } from "@/logger";
import { getFlagValue } from "@/utils";

export function config(data: cmdData) {
  const _global: boolean = getFlagValue(data, "global");
  switch (data.target[0].toLowerCase()) {
    case "list":
      break;
    case "set":
      set(data.target[1].toLowerCase(), data.target[2].toLowerCase(), _global);
      break;
    case "unset":
      break;
  }
}

function set(target: string, value: string, _global: boolean) {
  const settings: Record<string, any> = {};
  switch (target) {
    case "mode":
      if (value === "fg" || value === "foreground") {
        jcoreSettingsData.mode = "foreground";
      } else {
        jcoreSettingsData.mode = "background";
      }
      settings.mode = jcoreSettingsData.mode;
      logger.info("Mode set to " + jcoreSettingsData.mode);
      break;
    case "debug":
      jcoreSettingsData.debug = parseSetting(value);
      settings.debug = jcoreSettingsData.debug;
      logger.info("Debug set to " + (jcoreSettingsData.debug ? "On" : "Off"));
      break;
    case "install":
      jcoreSettingsData.install = parseSetting(value);
      settings.install = jcoreSettingsData.install;
      logger.info("Install set to " + (jcoreSettingsData.install ? "On" : "Off"));
      break;
    case "loglevel":
      if (value.match(/^[0-9]$/)) {
        jcoreSettingsData.logLevel = Number(value);
        settings.logLevel = jcoreSettingsData.logLevel;
        logger.info(`LogLevel set to ${jcoreSettingsData.logLevel}`);
      }
      break;
  }
  writeSettings(settings, _global);
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
