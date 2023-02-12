export const scriptLocation = "https://files.jco.fi/jcore-cli-main/";
export const archiveLocation = "https://files.jco.fi/wordpress-container-main.zip";
export const jcoreGit = "git@bitbucket.org:jcodigital/jcore2.git";
export const jcorePath = "wp-content/themes/jcore2";
export const childGit = "git@bitbucket.org:jcodigital/jcore2-child.git";
export const childPath = "wp-content/themes/jcore2-child";
export const updateFolder = ".update";
export const checksumFile = ".file.checksums";

export const projectFolders = [".jcore/wordpress", ".jcore/ssl", ".jcore/sql"];
export const globalFolders = [".config/jcore/ssl", ".config/jcore/ssh"];
export const externalCommands = ["composer", "npm", "docker", "docker-compose"];

export const commands = [
  {
    cmd: "child",
    text: "Makes project child theme by copying the local jcore2-child folder.",
    usage: ["<themename> - Creates a copy the jcore child theme."],
  },
  {
    cmd: "clean",
    text: "Delete image / VM / temp files.",
    usage: [""],
  },
  {
    cmd: "clone",
    text: "Clones a project from bitbucket, and sets everything up.",
    usage: ["<projectname> - Clones project from bitbucket."],
  },
  {
    cmd: "doctor",
    text: "Check status of the environment.",
    usage: [""],
  },
  {
    cmd: "init",
    text: "Creates a new project in the <target> folder.",
    usage: ["<projectname>"],
  },
  {
    cmd: "pull",
    text: "Syncs content from upstream.",
    usage: ["", "db|plugins|media"],
  },
  {
    cmd: "run",
    text: "Runs a command in container.",
    usage: ["<command>"],
  },
  {
    cmd: "set",
    text: "Set options in config file. Currently mode/debug.",
    usage: [
      "mode (fg|bg) - Sets docker to either running in foreground, or in background.",
      "debug (on|off) - Turns XDebug on or off by default.",
    ],
  },
  {
    cmd: "shell",
    text: "Opens a shell in the container / VM.",
    usage: [""],
  },
  {
    cmd: "start",
    text: "Installs composer and npm dependencies, starts container and gulp.",
    usage: ["- Start normally", "debug - Run with temporary debugging."],
  },
  {
    cmd: "stop",
    text: "Shutdown container. Removes docker ",
    usage: [""],
  },
  {
    cmd: "update",
    text: "Updates project. If the target is 'self' this script updates itself.",
    usage: [
      "- Updates the current project.",
      "self - Updates this script.",
      "drone|package|build|composer|docker - Forces update of protected components.",
    ],
  },
];
export const flags = [
  {
    name: "help",
    flag: "h",
    text: "This info.",
  },
  {
    name: "force",
    flag: "f",
    text: "Overwrites existing files.",
  },
  {
    name: "install",
    flag: "i",
    text: "Installs node modules even if they are already installed.",
  },
  {
    name: "nochild",
    flag: "n",
    text: "Doesn't install child theme on init command.",
  },
  {
    name: "quiet",
    flag: "q",
    text: "Print only errors.",
  },
  {
    name: "verbose",
    flag: "v",
    text: "Print more text.",
  },
  {
    name: "debug",
    flag: "d",
    text: "Print everything.",
  },
];
