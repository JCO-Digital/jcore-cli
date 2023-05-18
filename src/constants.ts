export const scriptLocation = "https://files.jco.fi/jcore-cli-main/";
export const archiveLocation = "https://files.jco.fi/wordpress-container-main.zip";
export const jcoreGit = "git@bitbucket.org:jcodigital/jcore2.git";
export const jcorePath = "wp-content/themes/jcore2";
export const childGit = "git@bitbucket.org:jcodigital/jcore2-child.git";
export const childPath = "wp-content/themes/jcore2-child";
export const updateFolder = ".update";
export const checksumFile = ".file.checksums.json";

export const projectFolders = [".jcore/wordpress", ".jcore/ssl", ".jcore/sql"];
export const globalFolders = [".config/jcore/ssl", ".config/jcore/ssh"];
export const externalCommands = ["composer", "npm", "docker", "docker-compose"];

export const commands = [
  {
    cmd: "checksum",
    text: "Manages file checksums.",
    description:
      "This is used to check which files have been changed manually, and should not be overwritten automatically. You can list all existing checksums, and reset checksums for specific files.",
    usage: [
      "list - Lists all checksums, and weather they match.",
      "set [...filename] - Calculate and set checksums for the given files.",
    ],
  },
  {
    cmd: "child",
    text: "Makes project child theme by copying the local jcore2-child folder.",
    description:
      "It uses a file copy operation, rather than checking out the git repository, so uncommitted files in the folder will be included. The default name will be the same as the project name, but a different name can be specified as the target.",
    usage: ["<themename> - Creates a copy the jcore child theme."],
  },
  {
    cmd: "clean",
    text: "Delete image / container / temp files.",
    description: "",
    usage: [
      "",
      "docker - Clean dangling images, containers and volumes.",
      "all - Cleans up after all projects. (Might delete non JCORE docker data)",
    ],
  },
  {
    cmd: "clone",
    text: "Clones a project from bitbucket, and sets everything up.",
    description: "",
    usage: ["<projectname> - Clones project from bitbucket."],
  },
  {
    cmd: "doctor",
    text: "Checks the status of the environment.",
    description: "",
    usage: [""],
  },
  {
    cmd: "init",
    text: "Creates a new project in the <target> folder.",
    description: "",
    usage: ["<projectname>"],
  },
  {
    cmd: "pull",
    text: "Syncs content from upstream.",
    description: "",
    usage: ["", "db|plugins|media"],
  },
  {
    cmd: "run",
    text: "Runs a command in container.",
    description: "",
    usage: ["<command>"],
  },
  {
    cmd: "set",
    text: "Set options in config file. Currently mode/debug.",
    description: "",
    usage: [
      "mode (fg|bg) - Sets docker to either running in foreground, or in background.",
      "debug (on|off) - Turns XDebug on or off by default.",
      "install (on|off) - Should npm and composer be run every start.",
    ],
  },
  {
    cmd: "shell",
    text: "Opens a shell in the container / VM.",
    description: "",
    usage: [""],
  },
  {
    cmd: "start",
    text: "Installs composer and npm dependencies, and starts container.",
    description: "",
    usage: ["- Start normally", "debug - Run with temporary debugging."],
  },
  {
    cmd: "stop",
    text: "Shutdown container. Removes docker ",
    description: "",
    usage: [""],
  },
  {
    cmd: "update",
    text: "Updates project. If the target is 'self' this script updates itself.",
    description: "",
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
