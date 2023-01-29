#!/usr/bin/env node
import parser from '@/parser';
import {version} from '@/package.json';
import {readSettings, settings} from "@/settings";
import {runCmd} from "@/cmd";
import {echo} from "@/utils";
import {help, helpCmd} from "@/help";

async function main() {
    await readSettings();

    // Intro text.
    echo("JCORE CLI v." + version, 1);
    echo("Mode: " + settings.mode);
    echo("Debug: " + (settings.debug ? 'On' : 'Off'));
    if (settings.inProject) {
        echo("Project: " + settings.name);
    }

    const data = parser(process.argv);

    if (data.cmd) {
        if (data.flags.includes('help')) {
            // Show help text for command.
            helpCmd(data);
        } else {
            // Run the command.
            runCmd(data);
        }
    } else {
        // Show generic help text.
        help(data);
    }
}

main().then(() => {
    echo("Finished", 3);
});

