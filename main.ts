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

    const data = parser(process.argv);

    if (data.cmd) {
        if (data.flags.includes('help')) {
            // Show help text for command.
            helpCmd(data);
        } else {
            // Run the command.
            runCmd(data, settings)
        }
    } else {
        // Show generic help text.
        help(data);
    }
}

main().then(() => {
});

