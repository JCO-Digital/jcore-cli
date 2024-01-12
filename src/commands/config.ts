import { configScope } from "@/constants";
import { logger } from "@/logger";
import { jcoreCmdData } from "@/parser";
import {
  deleteSetting,
  getConfig,
  jcoreRuntimeData,
  jcoreSettingsData,
  setConfigValue,
} from "@/settings";
import { configValue, settingsSchema } from "@/types";
import chalk from "chalk";

export function config() {
  const scope = jcoreCmdData.scope;
  switch (jcoreCmdData.target[0].toLowerCase()) {
    case "list":
      list((jcoreCmdData.target[1] ?? "all").toLowerCase());
      break;
    case "set":
      if (jcoreCmdData.target.length > 2) {
        set(jcoreCmdData.target[1], jcoreCmdData.target.slice(2), scope);
      }
      break;
    case "unset":
      if (jcoreCmdData.target.length > 1) {
        unset(jcoreCmdData.target[1], scope);
      }
      break;
  }
}

function set(target: string, values: string[], scope: configScope) {
  const value = values[0];

  // Pseudo setters.
  switch (target.toLowerCase()) {
    case "wpe":
      setConfigValue("remoteHost", `${value}@${value}.ssh.wpengine.net`, scope);
      setConfigValue("remotePath", `/sites/${value}`, scope);
      if (jcoreSettingsData.remoteDomain === "localhost") {
        setConfigValue("remoteDomain", `${value}.wpengine.com`, scope);
      }
      return;
    case "php":
      setConfigValue("wpImage", `jcodigi/wordpress:${value}`, scope);
      return;
  }

  const model: Record<string, configValue> = settingsSchema.parse({});
  for (const key in model) {
    if (key.toLowerCase() === target.toLowerCase()) {
      if (Array.isArray(model[key])) {
        if (value === "add" || value === "rm" || value === "remove") {
          const newValues = values.slice(1);
          const current = (jcoreSettingsData as Record<string, configValue>)[
            key
          ];
          if (isArrayOfStrings(current)) {
            for (let newValue of newValues) {
              newValue = newValue.replace(/ *=> */, "|");
              const index = current.indexOf(newValue);
              if (value === "add") {
                if (index === -1) {
                  current.push(newValue);
                  logger.info(
                    `Value ${formatValue(newValue)} added to ${chalk.green(
                      key,
                    )}`,
                  );
                } else {
                  logger.warn(
                    `Value ${formatValue(
                      newValue,
                    )} already exists in ${chalk.green(key)}`,
                  );
                }
              } else {
                if (index === -1) {
                  logger.warn(
                    `Value ${formatValue(
                      newValue,
                    )} doesn't exists in ${chalk.green(key)}`,
                  );
                } else {
                  current.splice(index, 1);
                  logger.info(
                    `Value ${formatValue(newValue)} removed from ${chalk.green(
                      key,
                    )}`,
                  );
                }
              }
            }
            setConfigValue(key, current, scope);
          }
        } else {
          setConfigValue(key, values, scope);
        }
        return;
      }
      switch (typeof model[key]) {
        case "string":
          setConfigValue(key, value, scope);
          return;
        case "number":
          if (Number.isNaN(Number(value))) {
            logger.error(`Error: ${value} is not numeric`);
          } else {
            setConfigValue(key, Number(value), scope);
          }
          return;
        case "boolean":
          setConfigValue(key, parseBoolean(value), scope);
          return;
      }
    }
  }
  logger.error(`Target ${target} not found.`);
}

function unset(target: string, scope: configScope) {
  const model: Record<string, configValue> = settingsSchema.parse({});
  for (const key in model) {
    if (key.toLowerCase() === target.toLowerCase()) {
      deleteSetting(key, scope);
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
    if (jcoreRuntimeData.inProject && option === "all") {
      logger.info(chalk.bold("Default settings:"));
      listConfig(getConfig(configScope.DEFAULT));
    }

    if (option === "global" || option === "all") {
      logger.info(chalk.bold("Global settings:"));
      listConfig(getConfig(configScope.GLOBAL));
    }

    if (
      option === "project" ||
      (jcoreRuntimeData.inProject && option === "all")
    ) {
      logger.info(chalk.bold("Project settings:"));
      listConfig(getConfig(configScope.PROJECT));
    }

    if (
      option === "local" ||
      (jcoreRuntimeData.inProject && option === "all")
    ) {
      logger.info(chalk.bold("Local settings:"));
      listConfig(getConfig(configScope.LOCAL));
    }
  }
}

function listConfig(values: Record<string, configValue>) {
  if (Object.keys(values).length) {
    let max = 0;
    for (const key in values) {
      if (key.length > max) {
        max = key.length;
      }
    }
    for (const key in values) {
      const value = values[key];
      logger.info(
        `${chalk.green(`${key}:`.padEnd(max + 1))} ${formatValue(value)}`,
      );
    }
  } else {
    logger.warn("No settings defined.");
  }
  logger.info("");
}

export function formatValue(value: configValue): string {
  if (Array.isArray(value)) {
    return `[\n${value.reduce((a, v) => {
      return `${a}   ${chalk.italic.blueBright(
        v.replace("|", chalk.whiteBright(" => ")),
      )}\n`;
    }, "")}]`;
  }
  switch (typeof value) {
    case "string":
      return chalk.cyan(value);
    case "number":
      return chalk.yellow(value);
    case "boolean":
      return chalk.magenta(value);
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

function isArrayOfStrings(value: unknown): value is string[] {
  return (
    Array.isArray(value) && value.every((item) => typeof item === "string")
  );
}
