import { jcoreSettingsData } from "@/settings";
import chalk from "chalk";
import { logLevels } from "@/constants";

class Logger {
  public error(message: string) {
    console.error(chalk.redBright.bold(message));
  }
  public warn(message: string, level: number = jcoreSettingsData.logLevel) {
    if (level >= logLevels.WARN) {
      console.warn(chalk.yellowBright(message));
    }
  }

  public info(message: string, level: number = jcoreSettingsData.logLevel) {
    if (level >= logLevels.INFO) {
      console.info(message);
    }
  }

  public http(message: string, level: number = jcoreSettingsData.logLevel) {
    if (level >= logLevels.HTTP) {
      console.log(message);
    }
  }

  public verbose(message: string, level: number = jcoreSettingsData.logLevel) {
    if (level >= logLevels.VERBOSE) {
      console.log(message);
    }
  }

  public debug(message: string, level: number = jcoreSettingsData.logLevel) {
    if (level >= logLevels.DEBUG) {
      console.debug(message);
    }
  }

  public silly(message: string, level: number = jcoreSettingsData.logLevel) {
    if (level >= logLevels.SILLY) {
      console.debug(message);
    }
  }
}

export const logger = new Logger();
