#!/usr/bin/env node
import parser, {help} from './parser';
import pack from './package.json';

console.log("JCORE CLI v." + pack.version + "\nMode: foreground\nDebug: 0\n");

const cmd = parser(process.argv);

if (!cmd.cmd) {
    help(cmd);
}

console.log(cmd);