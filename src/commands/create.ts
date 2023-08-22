import { cmdData } from "@/types";
import { jcoreSettingsData, writeSettings } from "@/settings";
import { existsSync, mkdirSync } from "fs";
import { execSync } from "child_process";
import { finalizeProject, replaceInFile, updateFiles } from "@/project";
import { childGit, childPath, jcoreGit, jcorePath } from "@/constants";
import { logger } from "@/logger";
import { getFlagValue, mergeFiles, nameToFolder } from "@/utils";
import { join } from "path";
import process from "process";

export function createProject(data: cmdData) {
  jcoreSettingsData.name = data.target[0];
  jcoreSettingsData.path = join(process.cwd(), jcoreSettingsData.name);
  if (existsSync(jcoreSettingsData.path)) {
    logger.warn("Project path exists: " + jcoreSettingsData.path);
  } else {
    logger.info("Create Project: " + jcoreSettingsData.name);
    mkdirSync(jcoreSettingsData.path);

    const options = {
      cwd: jcoreSettingsData.path,
      stdio: [0, 1, 2],
    };

    // Get the requested branch
    const branch = getFlagValue(data, "branch");

    if (branch) {
      jcoreSettingsData.branch = branch;
    }

    // Run project update.
    updateFiles().then(() => {
      // Git init.
      execSync("git init", options);

      // Add git submodules.
      let extra = "";
      if (jcoreSettingsData.branch) {
        extra += "-b " + jcoreSettingsData.branch;
      }
      execSync("git submodule add -f " + extra + ' "' + jcoreGit + '" ' + jcorePath, options);
      execSync("git submodule add -f " + extra + ' "' + childGit + '" ' + childPath, options);

      // Copy child theme.
      if (!getFlagValue(data, "nochild")) {
        copyChildTheme(jcoreSettingsData.name);
      }

      // Write config
      writeSettings();

      // GIT commit
      execSync("git add -A", options);
      execSync('git commit -m "Initial Commit"', options);

      // Finalize project.
      finalizeProject();
    });
  }
}

export function copyChildTheme(name: string): boolean {
  jcoreSettingsData.theme = nameToFolder(name);
  const themePath = join(jcoreSettingsData.path, "wp-content/themes", jcoreSettingsData.theme);
  if (!existsSync(themePath)) {
    mergeFiles(join(jcoreSettingsData.path, childPath), themePath, true);
    replaceInFile(join(themePath, "style.css"), [
      { search: /^Theme Name:.*$/gm, replace: `Theme Name: ${name}` },
    ]);
    return true;
  }
  return false;
}
