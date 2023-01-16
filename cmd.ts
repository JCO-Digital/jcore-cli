import type { JcoreSettings, cmdData } from "@/types";
import update, { selfUpdate } from "@/commands/update";
import start from "@/commands/start";

export function runCmd(cmd: cmdData, settings: JcoreSettings) {
    switch (cmd.cmd) {
        case "update":
            if (cmd.target.includes('self')) {
                selfUpdate(cmd, settings);
                // Update self.
            } else {
                // Update project.
                update(cmd, settings);
            }
            break;
        case "start":
            // Start the project.
            start(cmd, settings);
            break;
    }
}