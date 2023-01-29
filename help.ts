import {cmdData} from "@/types";
import {echo} from "@/utils";
import {settings} from "@/settings";
import {commands, flags} from "@/constants";

export function help(cmd: cmdData) {
    echo("\nUsage: " + settings.exec + " <command> [options] <target>");

    if (cmd.flags.includes('help')) {
        const padding = 16;
        echo("\nPossible commands:");
        for (let cmd of commands) {
            echo(cmd.cmd.padEnd(padding) + " - " + cmd.text);
        }

        echo("\nPossible options:");
        for (let flag of flags) {
            echo(("--" + flag.name + " / -" + flag.flag).padEnd(padding) + " - " + flag.text);
        }
    } else {
        echo("Use flag --help for more info.");
    }

}

export function helpCmd(cmd: cmdData, text: boolean = true, usage: boolean = true) {
    const padding = 16;
    for (let command of commands) {
        if (cmd.cmd === command.cmd) {
            echo("");
            if (text) {
                echo(command.text);
            }
            if (usage) {
                echo('Usage:');
                for (let use of command.usage) {
                    const part = use.split('-');
                    let useText = part[0].trim().padEnd(padding);
                    if (part.length > 1) {
                        useText += ' - ' + part[1].trim();
                    }
                    echo(usageText(command.cmd, useText));
                }
            }
        }
    }
}

export function usageText(cmd: string, text: string) {
    return settings.exec + ' ' + cmd + ' ' + text;
}