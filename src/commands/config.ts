import { cmdData, settingsSchema } from "@/types";
import { jcoreSettingsData, updateSetting } from "@/settings";
import { logger } from "@/logger";
import { getFlagValue } from "@/utils";

export function config(data: cmdData) {
  const _global: boolean = getFlagValue(data, "global");
  switch (data.target[0].toLowerCase()) {
    case "list":
      console.log(jcoreSettingsData);
      break;
    case "set":
      if (data.target.length > 2) {
        set(data.target[1], data.target[2], _global);
      }
      break;
    case "unset":
      break;
  }
}

function set(target: string, value: string, _global: boolean) {
  const model: Record<string, string | number | boolean | Array<string | Array<string>>> =
    settingsSchema.parse({});
  for (const key in model) {
    if (key.toLowerCase() === target.toLowerCase()) {
      switch (typeof model[key]) {
        case "string":
          updateSetting(key, value, _global);
          return;
        case "number":
          if (isNaN(Number(value))) {
            logger.error(`Error: ${value} is not numeric`);
          } else {
            updateSetting(key, Number(value), _global);
          }
          return;
        case "boolean":
          updateSetting(key, parseBoolean(value), _global);
          return;
        default:
          console.log(key);
      }
    }
  }
  logger.info(`Target ${target} not found.`);
}

function parseBoolean(value: string): boolean {
  switch (value.toLowerCase()) {
    case "true":
    case "yes":
    case "on":
    case "y":
    case "t":
    case "1":
      return true;
    default:
      return false;
  }
}
