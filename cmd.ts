import type { JcoreSettings, cmdData } from "@/types";
import update, { selfUpdate } from "@/commands/update";
import start, {stop} from "@/commands/docker";

export function runCmd(data: cmdData, settings: JcoreSettings) {
    switch (data.cmd) {
        case "update":
            if (data.target.includes('self')) {
                selfUpdate(data);
                // Update self.
            } else {
                // Update project.
                update(data, settings);
            }
            break;
        case "start":
            // Start the project.
            start(data, settings);
            break;
        case "stop":
            // Start the project.
            stop(data, settings);
            break;
    }
}