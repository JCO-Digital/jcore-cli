import { settings } from "@/settings";

class Logger {
  levels = {
    error: 0,
    warn: 1,
    info: 2,
    http: 3,
    verbose: 4,
    debug: 5,
    silly: 6,
  };
  public error(message: string) {
    console.error(message);
  }
  public warn(message: string, level: number = settings.logLevel) {
    if (level >= this.levels.warn) {
      console.warn(message);
    }
  }

  public info(message: string, level: number = settings.logLevel) {
    if (level >= this.levels.info) {
      console.info(message);
    }
  }

  public http(message: string, level: number = settings.logLevel) {
    if (level >= this.levels.http) {
      console.log(message);
    }
  }

  public verbose(message: string, level: number = settings.logLevel) {
    if (level >= this.levels.verbose) {
      console.log(message);
    }
  }

  public debug(message: string, level: number = settings.logLevel) {
    if (level >= this.levels.debug) {
      console.debug(message);
    }
  }

  public silly(message: string, level: number = settings.logLevel) {
    if (level >= this.levels.silly) {
      console.debug(message);
    }
  }
}

export const logger = new Logger();
