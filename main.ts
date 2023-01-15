#!/usr/bin/env node
import parser, {help, helpCmd} from '@/parser';
import {version} from '@/package.json';
import {readSettings} from "@/settings";
import {runCmd} from "@/cmd";

async function main() {
    const settings = await readSettings();

    // Intro text.
    console.log("JCORE CLI v." + version);
    console.log("Mode: " + settings.mode);
    console.log("Debug: " + (settings.debug ? 'On' : 'Off'));

    const cmd = parser(process.argv);

    if (cmd.cmd) {
        if (cmd.flags.includes('help')) {
            // Show help text for command.
            helpCmd(cmd);
        } else {
            // Run the command.
            runCmd(cmd, settings)
        }
    } else {
        // Show generic help text.
        help(cmd);
    }
}

main().then(() => {
    console.log("Done");
});

