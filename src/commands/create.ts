import { cmdData } from "@/types";
import { settings, writeSettings } from "@/settings";
import { existsSync, mkdirSync } from "fs";
import { execSync } from "child_process";
import { finaliseProject, updateFiles } from "@/project";
import { childGit, childPath, jcoreGit, jcorePath } from "@/constants";
import { logger } from "@/logger";
import { mergeFiles } from "@/utils";
import { join } from "path";
import process from "process";

export function createProject(data: cmdData) {
  settings.name = data.target[0];
  settings.path = join(process.cwd(), settings.name);
  if (existsSync(settings.path)) {
    logger.warn("Project path exists: " + settings.path);
  } else {
    logger.info("Create Project: " + settings.name);
    mkdirSync(settings.path);

    const options = {
      cwd: settings.path,
      stdio: [0, 1, 2],
    };

    // Run project update.
    updateFiles().then(() => {
      // Git init.
      execSync("git init", options);

      // Add git submodules.
      let extra = "";
      if (settings.branch) {
        extra += "-b " + settings.branch;
      }
      execSync(
        "git submodule add -f " + extra + ' "' + jcoreGit + '" ' + jcorePath,
        options
      );
      execSync(
        "git submodule add -f " + extra + ' "' + childGit + '" ' + childPath,
        options
      );

      // Copy child theme.
      if (!data.flags.includes("nochild")) {
        createChildTheme(settings.name);
      }

      // TODO Write config
      writeSettings();

      // GIT commit
      execSync("git add -A", options);
      execSync('git commit -m "Initial Commit"', options);

      // TODO Finalize project.
      finaliseProject();
    });
  }
}

export function createChildTheme(name: string) {
  settings.theme = name;
  const themePath = join(settings.path, "wp-content/themes", settings.theme);
  mergeFiles(join(settings.path, childPath), themePath, true);
}
