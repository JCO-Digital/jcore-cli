import type {cmdData} from "@/types";
import {commands, flags} from "@/constants";

export default function parser(args: Array<string>): cmdData {
    const data = {
        nodePath: "",
        execPath: "",
        exec: "",
        cmd: "",
        target: [],
        flags: []
    } as cmdData;

    if (args.length > 2) {
        data.nodePath = args.shift() ?? "";
        data.execPath = (args.shift() ?? "");
        data.exec = data.execPath.split('/').pop() ?? "";
    }
    let count = 0;
    for (let part of args) {
        if (part.substring(0, 1) === "-") {
            // Flag
            for (let flag of flags) {
                if (flag.flag === part.substring(1) || flag.name === part.substring(2)) {
                    data.flags.push(flag.name);
                }
            }
        } else if (count === 0) {
            for (let cmd of commands) {
                if (cmd.cmd === part) {
                    data.cmd = part;
                }
            }
            count++;
        } else {
            data.target.push(part);
            count++;
        }
    }

    return data;
}

export function help(cmd: cmdData) {
    let output = "\nUsage: " + cmd.exec + " <command> [options] <target>\n";

    if (cmd.flags.includes('help')) {
        const padding = 16;
        output += "\nPossible commands:\n";
        for (let cmd of commands) {
            output += cmd.cmd.padEnd(padding) + " - " + cmd.text + "\n";
        }

        output += "\nPossible options:\n";
        for (let flag of flags) {
            output += ("--" + flag.name + " / -" + flag.flag).padEnd(padding) + " - " + flag.text + "\n";
        }
    } else {
        output += "Use flag --help for more info.";
    }

    console.info(output);
}

export function helpCmd(cmd: cmdData) {
    let output = "\n";
    for (let command of commands) {
        if (cmd.cmd === command.cmd) {
            output += command.text + "\n\n";
            output += "Usage:\n\n";
            for (let use of command.usage) {
                output += cmd.exec + " " + command.cmd + " " + use +"\n";
            }
        }
    }

    console.info(output);
}