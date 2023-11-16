import { cmdSchema } from "@/types";
import { commands, configScope, flagReturnType, logLevels, optionDefinition } from "@/constants";
import { parse } from "path";
import { jcoreRuntimeData } from "@/settings";

export const jcoreCmdData = cmdSchema.parse({});

export default function parser(args: Array<string>) {
  if (args.length > 1) {
    jcoreRuntimeData.nodePath = args.shift() ?? "";
    jcoreRuntimeData.execPath = args.shift() ?? "";
    jcoreRuntimeData.exec = parse(jcoreRuntimeData.execPath).base;
  }
  let count = 0;
  for (let i = 0; i < args.length; i++) {
    const part = args[i];
    // We hit a flag.
    if (part.substring(0, 2) === "--") {
      const find = part.match(/^--([^=]+)(=(.*))?$/);
      if (find) {
        matchFlag(find[1].toLowerCase(), find[3]);
      }
    } else if (part.substring(0, 1) === "-") {
      if (matchFlag(part.substring(1).toLowerCase(), args[i + 1]) === flagReturnType.HASVALUE) {
        i++;
      }
    } else if (count === 0) {
      for (const cmd of commands) {
        if (cmd.cmd === part) {
          jcoreCmdData.cmd = part;
        }
      }
      count++;
    } else {
      jcoreCmdData.target.push(part);
      count++;
    }
  }
}

function matchFlag(flag: string, value: string | undefined): flagReturnType {
  for (const option of optionDefinition) {
    if (flag === option.name || flag === option.alias) {
      if (option.argument !== undefined) {
        const parsed = option.argument(value);
        setFlagValue(option.name, parsed);
        return flagReturnType.HASVALUE;
      }
      setFlagValue(option.name, true);
      return flagReturnType.FOUND;
    }
  }
  return flagReturnType.NOTFOUND;
}

function setFlagValue(flag: string, value: boolean | string | number) {
  switch (flag) {
    case "global":
      jcoreCmdData.scope = configScope.GLOBAL;
      return;
    case "local":
      jcoreCmdData.scope = configScope.LOCAL;
      return;
    case "debug":
      jcoreCmdData.logLevel = logLevels.DEBUG;
      return;
    case "verbose":
      jcoreCmdData.logLevel = logLevels.VERBOSE;
      return;
    case "quiet":
      jcoreCmdData.logLevel = logLevels.ERROR;
      return;
    case "loglevel":
      if (typeof value === "number") {
        jcoreCmdData.logLevel = value;
      }
      return;
    default:
      jcoreCmdData.flags.set(flag, value);
      return;
  }
}
