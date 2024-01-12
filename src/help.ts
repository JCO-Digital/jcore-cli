import { jcoreRuntimeData } from "@/settings";
import { commands, optionDefinition } from "@/constants";
import { logger } from "@/logger";
import { getFlag } from "@/utils";
import { jcoreCmdData } from "@/parser";

export function help() {
  logger.info(`Usage: ${jcoreRuntimeData.exec} <command> [options] <target>`);

  if (getFlag("help")) {
    const padding = 16;
    logger.info("\nPossible commands:");
    for (const cmd of commands) {
      logger.info(`${cmd.cmd.padEnd(padding)} - ${cmd.text}`);
    }

    logger.info("\nPossible options:");
    for (const option of optionDefinition) {
      logger.info(
        `${`--${option.name} / -${option.alias}`.padEnd(padding)} - ${
          option.description
        }`,
      );
    }
  } else {
    logger.info("Use flag --help for more info.");
  }
}

export function helpCmd(text = true, usage = true) {
  const padding = 16;
  for (const command of commands) {
    if (jcoreCmdData.cmd === command.cmd) {
      logger.info("");
      if (text) {
        logger.info(command.text);
      }
      if (usage) {
        logger.info("Usage:");
        for (const use of command.usage) {
          const part = use.split("-");
          let useText = part[0].trim().padEnd(padding);
          if (part.length > 1) {
            useText += ` - ${part[1].trim()}`;
          }
          logger.info(usageText(command.cmd, useText));
        }
      }
    }
  }
}

export function usageText(cmd: string, text: string) {
  return `${jcoreRuntimeData.exec} ${cmd} ${text}`;
}
