#!/usr/bin/env node
import parser, {help} from '@/parser';
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
        runCmd(cmd, settings)
    } else {
        help(cmd);
    }
}

main().then(() => {
    console.log("Done");
});

