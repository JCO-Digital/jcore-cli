import { z } from "zod";

export enum configScope {
  INVALID = 0,
  DEFAULT = 1,
  GLOBAL = 2,
  PROJECT = 3,
  LOCAL = 4,
}
export enum flagReturnType {
  NOTFOUND = 0,
  FOUND = 1,
  HASVALUE = 2,
}

export enum logLevels {
  ERROR,
  WARN,
  INFO,
  HTTP,
  VERBOSE,
  DEBUG,
  SILLY,
};

export const scriptLocation = "https://github.com/JCO-Digital/jcore-cli/releases/latest/download/";
export const scriptName = "jcore";
export const archiveLocation =
  "https://github.com/JCO-Digital/wordpress-container/releases/latest/download/release.zip";
export const jcoreGit = "git@bitbucket.org:jcodigital/jcore2.git";
export const jcorePath = "wp-content/themes/jcore2";
export const childGit = "git@bitbucket.org:jcodigital/jcore2-child.git";
export const childPath = "wp-content/themes/jcore2-child";
export const updateFolder = ".update";
export const checksumFile = ".file.checksums.json";

export const projectFolders = [".jcore/wordpress", ".jcore/ssl", ".jcore/sql"];
export const globalFolders = [".config/jcore/ssl", ".config/jcore/ssh"];
export const externalCommands = [
  { name: "node", version: "-v", min: 14 },
  { name: "npm", version: "-v", min: 1 },
  { name: "composer", version: "-V", min: 2 },
  { name: "docker", version: "-v", min: 20 },
  { name: "docker-compose", version: "-v", min: 2 }
];

export const projectSettings = [
  "projectName",
  "theme",
  "branch",
  "remoteDomain",
  "localDomain",
  "domains",
  "replace",
  "remoteHost",
  "remotePath",
  "dbExclude",
  "pluginInstall",
  "pluginExclude",
  "pluginGit",
  "wpImage",
  "wpDebug",
  "wpDebugLog",
  "wpDebugDisplay"
];

export const optionDefinition = [
  {
    name: "force",
    alias: "f",
    description: "Overwrites existing files."
  },
  {
    name: "global",
    alias: "g",
    description: "Write settings globally."
  },
  {
    name: "help",
    alias: "h",
    description: "This info."
  },
  {
    name: "install",
    alias: "i",
    description: "Installs node modules even if they are already installed."
  },
  {
    name: "local",
    alias: "l",
    description: "Write settings locally."
  },
  {
    name: "nochild",
    alias: "n",
    description: "Doesn't install child theme on init command."
  },
  {
    name: "quiet",
    alias: "q",
    description: "Print only errors."
  },
  {
    name: "verbose",
    alias: "v",
    description: "Print more text."
  },
  {
    name: "debug",
    alias: "d",
    description: "Print everything."
  },
  {
    name: "loglevel",
    alias: "p",
    argument: Number,
    description: "Print everything."
  },
  {
    name: "branch",
    alias: "b",
    argument: String,
    description: "Set the JCORE branch to use, used in the init command."
  }
];


export const commands = [
  {
    cmd: "attach",
    text: "Attach to the logs of all containers",
    description: "",
    usage: ["<container> - A specific container to attach to, leave empty for all."]
  },
  {
    cmd: "checksum",
    text: "Manages file checksums.",
    description:
      "This is used to check which files have been changed manually, and should not be overwritten automatically. You can list all existing checksums, and reset checksums for specific files.",
    usage: [
      "list - Lists all checksums, and weather they match.",
      "set [...filename] - Calculate and set checksums for the given files."
    ]
  },
  {
    cmd: "child",
    text: "Makes project child theme by copying the local jcore2-child folder.",
    description:
      "It uses a file copy operation, rather than checking out the git repository, so uncommitted files in the folder will be included. The default name will be the same as the project name, but a different name can be specified as the target.",
    usage: ["<themename> - Creates a copy the jcore child theme."]
  },
  {
    cmd: "clean",
    text: "Delete image / container / temp files.",
    description: "",
    usage: [
      "",
      "docker - Clean dangling images, containers and volumes.",
      "all - Cleans up after all projects. (Might delete non JCORE docker data)"
    ]
  },
  {
    cmd: "clone",
    text: "Clones a project from bitbucket, and sets everything up.",
    description: "",
    usage: ["<projectname> - Clones project from bitbucket."]
  },
  {
    cmd: "doctor",
    text: "Checks the status of the environment.",
    description: "",
    usage: [""]
  },
  {
    cmd: "init",
    text: "Creates a new project in the <target> folder.",
    description: "",
    usage: ["<projectname>"]
  },
  {
    cmd: "pull",
    text: "Syncs content from upstream.",
    description: "",
    usage: ["", "db|plugins|media"]
  },
  {
    cmd: "run",
    text: "Runs a command in container.",
    description: "",
    usage: ["<command>"]
  },
  {
    cmd: "config",
    text: "Set options in config file. Currently mode/debug.",
    description: "",
    usage: [
      "list - List all settings.",
      "set - Sets config value.",
      "unset - Removes a setting, probably returning it to defaults."
    ]
  },
  {
    cmd: "shell",
    text: "Opens a shell in the container / VM.",
    description: "",
    usage: [""]
  },
  {
    cmd: "status",
    text: "Shows information about running projects.",
    description: "",
    usage: [""]
  },
  {
    cmd: "start",
    text: "Installs composer and npm dependencies, and starts container.",
    description: "",
    usage: ["- Start normally", "debug - Run with temporary debugging."]
  },
  {
    cmd: "stop",
    text: "Shutdown container. Removes docker ",
    description: "",
    usage: [""]
  },
  {
    cmd: "update",
    text: "Updates project. If the target is 'self' this script updates itself.",
    description: "",
    usage: [
      "- Updates the current project.",
      "self - Updates this script.",
      "<...filename> - Updates only the selected files."
    ]
  }
];
