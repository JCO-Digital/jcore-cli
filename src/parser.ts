import type { cmdData } from "@/types";
import { commands, flags } from "@/constants";
import { parse } from "path";
import { jcoreSettingsData } from "@/settings";

export default function parser(args: Array<string>): cmdData {
  const data = {
    cmd: "",
    target: [],
    flags: [],
  } as cmdData;

  if (args.length > 1) {
    jcoreSettingsData.nodePath = args.shift() ?? "";
    jcoreSettingsData.execPath = args.shift() ?? "";
    jcoreSettingsData.exec = parse(jcoreSettingsData.execPath).base;
  }
  let count = 0;
  for (const part of args) {
    if (part.substring(0, 1) === "-") {
      // Flag
      for (const flag of flags) {
        if (flag.flag === part.substring(1) || flag.name === part.substring(2)) {
          data.flags.push(flag.name);
        }
      }
    } else if (count === 0) {
      for (const cmd of commands) {
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
