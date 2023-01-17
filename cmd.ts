import type { JcoreSettings, cmdData } from "@/types";
import update, { selfUpdate } from "@/commands/update";
import start from "@/commands/start";

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
    }
}