export const scriptLocation = "https://files.jco.fi/jcore-cli-main/";

export const commands = [
    {
        cmd: "init",
        text: "Creates a new project in the <target> folder.",
        usage: [
            "<projectname>"
        ]
    },
    {
        cmd: "update",
        text: "Updates project. If the target is 'self' this script updates itself.",
        usage: [
            "",
            "self",   
        ]
    },
    {
        cmd: "clone",
        text: "Clones a project from bitbucket, and sets everything up.",
        usage: [
            "<projectname>"
        ]
    },
    {
        cmd: "start",
        text: "Installs composer and npm dependencies, starts container and gulp.",
        usage: [
            "",
            "debug"
        ]
    },
    {
        cmd: "stop",
        text: "Shutdown container.",
        usage: [
            "",
        ]
    },
    {
        cmd: "clean",
        text: "Delete image / VM / temp files.",
        usage: [
            "",
        ]
    },
    {
        cmd: "child",
        text: "Makes project child theme by copying the local jcore2-child folder.",
        usage: [
            "",
        ]
    },
    {
        cmd: "pull",
        text: "Syncs content from upstream.",
        usage: [
            "",
            "db|plugins|media"
        ]
    },
    {
        cmd: "run",
        text: "Runs a command in container.",
        usage: [
            "<command>",
        ]
    },
    {
        cmd: "shell",
        text: "Opens a shell in the container / VM.",
        usage: [
            "",
        ]
    },
    {
        cmd: "set",
        text: "Set options in config file. Currently mode/debug.",
        usage: [
            "mode (foreground|background)",
            "debug (on|off)",
        ]
    },
    {
        cmd: "doctor",
        text: "Check status of the environment.",
        usage: [
            "",
        ]
    },
];
export const flags = [
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
