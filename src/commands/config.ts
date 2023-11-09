import { cmdData, settingsSchema } from "@/types";
import { getConfig, jcoreSettingsData, updateSetting } from "@/settings";
import { logger } from "@/logger";
import { getFlagValue } from "@/utils";
import { configScope } from "@/constants";
import chalk from "chalk";

export function config(data: cmdData) {
  const scope = getFlagValue(data, "global")
    ? configScope.GLOBAL
    : getFlagValue(data, "local")
    ? configScope.LOCAL
    : configScope.PROJECT;
  switch (data.target[0].toLowerCase()) {
    case "list":
      list((data.target[1] ?? "all").toLowerCase());
      break;
    case "set":
      if (data.target.length > 2) {
        set(data.target[1], data.target[2], scope);
      }
      break;
    case "unset":
      if (data.target.length > 1) {
        unset(data.target[1], scope);
      }
      break;
  }
}

function set(target: string, value: string, scope: configScope) {
  // Pseudo setters.
  switch (target.toLowerCase()) {
    case "wpe":
      updateSetting("remoteHost", `${value}@${value}.ssh.wpengine.net`, scope);
      updateSetting("remotePath", `/sites/${value}`, scope);
      return;
    case "php":
      updateSetting("wpImage", `jcodigital/wordpress:${value}`, scope);
      return;
  }

  const model: Record<string, string | number | boolean | Array<string | Array<string>>> =
    settingsSchema.parse({});
  for (const key in model) {
    if (key.toLowerCase() === target.toLowerCase()) {
      switch (typeof model[key]) {
        case "string":
          updateSetting(key, value, scope);
          return;
        case "number":
          if (isNaN(Number(value))) {
            logger.error(`Error: ${value} is not numeric`);
          } else {
            updateSetting(key, Number(value), scope);
          }
          return;
        case "boolean":
          updateSetting(key, parseBoolean(value), scope);
          return;
        default:
          console.log(key);
      }
    }
  }
  logger.error(`Target ${target} not found.`);
}

function unset(target: string, scope: configScope) {
  const model: Record<string, string | number | boolean | Array<string | Array<string>>> =
    settingsSchema.parse({});
  for (const key in model) {
    if (key.toLowerCase() === target.toLowerCase()) {
      updateSetting(key, null, scope);
      return;
    }
  }
  logger.error(`Target ${target} not found.`);
}

function list(option = "") {
  logger.info("");
  if (option === "active") {
    logger.info(chalk.bold("Active settings:"));
    listConfig(jcoreSettingsData);
  } else {
    if (option === "global" || option === "all") {
      logger.info(chalk.bold("Global settings:"));
      listConfig(getConfig(configScope.GLOBAL));
    }

    if (option === "project" || option === "all") {
      logger.info(chalk.bold("Project settings:"));
      listConfig(getConfig(configScope.PROJECT));
    }

    if (option === "local" || option === "all") {
      logger.info(chalk.bold("Local settings:"));
      listConfig(getConfig(configScope.LOCAL));
    }
  }
}

function listConfig(values: Record<string, any>) {
  for (const key in values) {
    const value = values[key];
    logger.info(`${chalk.green(`${key}:`.padEnd(14))} ${formatValue(value)}`);
  }
  logger.info("");
}

export function formatValue(
  value: string | number | boolean | Array<string | Array<string>>
): string {
  if (Array.isArray(value)) {
    return `[\n${value.reduce((a, v) => {
      return `${a}   ${chalk.italic.blueBright(
        Array.isArray(v) ? v.join(`" ${chalk.whiteBright("=>")} "`) : v
      )}\n`;
    }, "")}]`;
  } else {
    switch (typeof value) {
      case "string":
        return chalk.cyan(`"${value}"`);
      case "number":
        return chalk.yellow(value);
      case "boolean":
        return chalk.magenta(value);
    }
  }
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
