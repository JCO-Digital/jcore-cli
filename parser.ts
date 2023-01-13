const commands = [
    {
        cmd: "init",
        text: "Creates a new project in the <target> folder."
    },
    {
        cmd: "update",
        text: "Updates project. If the target is 'self' this script updates itself."
    },
    {
        cmd: "clone",
        text: "Clones a project from bitbucket, and sets everything up."
    },
    {
        cmd: "start",
        text: "Installs composer and npm dependencies, starts container and gulp."
    },
    {
        cmd: "stop",
        text: "Shutdown container."
    },
    {
        cmd: "clean",
        text: "Delete image / VM / temp files."
    },
    {
        cmd: "child",
        text: "Makes project child theme by copying the local jcore2-child folder."
    },
    {
        cmd: "pull",
        text: "Syncs content from upstream."
    },
    {
        cmd: "run",
        text: "Runs a command in container."
    },
    {
        cmd: "shell",
        text: "Opens a shell in the container / VM."
    },
    {
        cmd: "set",
        text: "Set options in config file. Currently mode/debug."
    },
    {
        cmd: "doctor",
        text: "Check status of the environment."
    },
];
const flags = [
    {
        name: "help",
        flag: "h",
        text: "This info."
    },
    {
        name: "force",
        flag: "f",
        text: "Overwrites existing files."
    },
    {
        name: "start",
        flag: "s",
        text: "Starts container and gulp on commands that supports it."
    },
    {
        name: "install",
        flag: "i",
        text: "Installs node modules even if they are already installed."
    },
    {
        name: "nochild",
        flag: "n",
        text: "Doesn't install child theme on init command."
    },
];

export default function parser(args: Array<string>) {
    const data = {
        node: "",
        exec: "",
        cmd: "",
        target: "",
        flags: [] as Array<string>
    }

    data.node = args.shift();
    data.exec = args.shift().split("/").pop();
    let count = 0;
    for (let part of args) {
        if (part.substring(0, 1) === "-") {
            // Flag
            for (let flag of flags) {
                if (flag.flag === part.substring(1) || flag.name === part.substring(2)) {
                    data.flags.push(flag.name);
                }
            }
        } else if (data.cmd === "") {
            for (let cmd of commands) {
                if (cmd.cmd === part) {
                    data.cmd = part;
                }
            }
        } else if (data.target === "") {
            data.target = part;
        }
    }

    return data;
}

export function help(cmd) {
    let output = "Usage: " + cmd.exec + " <command> [options] <target>\n";

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
        output += "\nUse --help for more info.";
    }

    console.info(output);
}