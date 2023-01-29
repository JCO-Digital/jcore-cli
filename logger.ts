class Logger {
    level = 2;
    levels = {
        error: 0,
        warn: 1,
        info: 2,
        http: 3,
        verbose: 4,
        debug: 5,
        silly: 6
    };
    public constructor(private defaultLevel: number) {
        this.level = defaultLevel;
    }

    public error (message: string) {
        console.error(message);
    }
    public warn (message: string) {
        if (this.level >= this.levels.warn) {
            console.warn(message);
        }
    }

    public info (message: string) {
        if (this.level >= this.levels.info) {
            console.info(message);
        }
    }

    public http (message: string) {
        if (this.level >= this.levels.http) {
            console.log(message);
        }
    }

    public verbose (message: string) {
        if (this.level >= this.levels.verbose) {
            console.log(message);
        }
    }

    public debug (message: string) {
        if (this.level >= this.levels.debug) {
            console.debug(message);
        }
    }

    public silly (message: string) {
        if (this.level >= this.levels.silly) {
            console.debug(message);
        }
    }
}

export const logger = new Logger(2);