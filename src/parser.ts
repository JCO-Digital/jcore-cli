import type { cmdData } from "@/types";
import { commands, flags } from "@/constants";
import { parse } from "path";
import { jcoreSettingsData } from "@/settings";
import { logger } from "@/logger";

export default function parser(args: Array<string>): cmdData {
  const data = {
    cmd: "",
    target: [],
    flags: new Map(),
  } as cmdData;

  if (args.length > 1) {
    jcoreSettingsData.nodePath = args.shift() ?? "";
    jcoreSettingsData.execPath = args.shift() ?? "";
    jcoreSettingsData.exec = parse(jcoreSettingsData.execPath).base;
  }
  let count = 0;
  for (let i = 0; i < args.length; i++) {
    const part = args[i];
    // We hit a flag.
    if (part.substring(0, 1) === "-") {
      // Start checking every flag defined for a match.
      for (const flag of flags) {
        // Short match is for -<flag short>
        const shortMatch = flag.flag === part.substring(1);
        // Long flags --<flag name> requires to split with equals sign, however they might not take a value.
        const equalsMatch = part.lastIndexOf("=");
        // Get the flag name, without the value.
        const nameMatch = part.substring(2, equalsMatch === -1 ? part.length : equalsMatch);
        const longMatch = flag.name === nameMatch;
        if (!shortMatch && !longMatch) {
          continue;
        }
        // We do not have an argument, so we just check for a match.
        if (!flag.args && (shortMatch || longMatch)) {
          data.flags.set(flag.name, true);
          continue;
        }
        // Check for flags that can take arguments.
        let argumentData;
        if (shortMatch) {
          // Short matches has their arguments as the next "part"
          if (!args[i + 1]) {
            logger.error(`Error: Flag ${flag.name} needs an argument.`);
            throw new Error(`Flag ${flag.name} needs an argument.`);
          }
          argumentData = args[i + 1];
          i = i + 1;
        }
        if (longMatch) {
          // Long flags have their value after the equal sign in this part.
          // If we do not have a equal sign or it is the last.
          if (equalsMatch === -1 || equalsMatch === part.length - 1) {
            logger.error(`Error: Flag ${flag.name} needs an argument.`);
            throw new Error(`Flag ${flag.name} needs an argument.`);
          }
          // If we get a match get the actual data.
          const start = equalsMatch + 1;
          argumentData = part.slice(start);
        }
        if (!argumentData) {
          logger.error(`Error: Flag ${flag.name} needs an argument.`);
          throw new Error(`Flag ${flag.name} needs an argument.`);
        }
        // Finally we take the parser from the flag and run that, if we have a parser.
        let parsed = argumentData;
        if (flag.type) {
          parsed = flag.type.parse(argumentData);
        }
        data.flags.set(flag.name, parsed);
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

  console.log(data);

  return data;
}
