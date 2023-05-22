import { logger } from "@/logger";
import { externalCommands, globalFolders, projectFolders } from "@/constants";
import { jcoreSettingsData } from "@/settings";
import { join } from "path";
import { accessSync, existsSync, mkdirSync } from "fs";
import { homedir, userInfo } from "os";
import { W_OK } from "constants";
import { execSync, StdioOptions } from "child_process";

export function doctor() {
  logger.verbose("\nChecking system:");
  if (checkFolders() && checkCommands()) {
    logger.info("\nEverything seems fine.");
  } else {
    logger.error("\nErrors encountered!");
  }
}

export function checkFolders(logLevel: number = jcoreSettingsData.logLevel): boolean {
  let pass = true;
  if (jcoreSettingsData.inProject) {
    for (const folder of projectFolders) {
      if (!processFolder(join(jcoreSettingsData.path, folder), logLevel)) {
        pass = false;
      }
    }
  }
  for (const folder of globalFolders) {
    if (!processFolder(join(homedir(), folder), logLevel)) {
      pass = false;
    }
  }
  return pass;
}

export function checkCommands(logLevel: number = jcoreSettingsData.logLevel): boolean {
  const options = {
    stdio: ["pipe", "pipe", "ignore"] as StdioOptions,
  };

  let pass = true;
  for (const command of externalCommands) {
    try {
      const output = execSync(`${command.name} ${command.version}`, options).toString();
      const version = output.match(/([0-9]+)\.([0-9]+)\.([0-9]+)/);
      if (version !== null) {
        logger.info(`${command.name} version ${version[0]} found.`, logLevel);
      } else {
        logger.error(`${command.name} version error.`);
      }
    } catch (e) {
      logger.error(`Command ${command.name} not found!`);
      pass = false;
    }
  }
  return pass;
}

function processFolder(path: string, logLevel: number): boolean {
  if (existsSync(path)) {
    // Folder exists, check permissions.
    logger.debug(`Folder ${path} exists`, logLevel);
    try {
      // Check if folder is writable.
      accessSync(path, W_OK);
      logger.info(`Folder ${path} OK.`);
      // Everything is fine.
      return true;
    } catch (e) {
      // Folder is not writable.
      const user = userInfo().username;
      logger.error(`Folder ${path} is not writable. It is probably owned by root.`);
      logger.info(`Fix this by running "sudo chown ${user} -R ${path}".`, logLevel);
    }
  } else {
    // Folder doesn't exist, create it.
    logger.verbose(`Folder ${path} doesn't exist, creating it.`, logLevel);
    try {
      mkdirSync(path, { recursive: true });
      // We can return true, because the folder should be created.
      return true;
    } catch (e) {
      // Folder creation failed.
      logger.error(`Failed to create folder ${path}`);
    }
  }
  return false;
}
