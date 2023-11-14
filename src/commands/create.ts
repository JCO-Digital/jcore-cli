import { cmdData, configValue } from "@/types";
import { getScopeConfigFile, jcoreRuntimeData, jcoreSettingsData, updateConfigValues } from "@/settings";
import { existsSync, mkdirSync } from "fs";
import { execSync } from "child_process";
import { finalizeProject, replaceInFile, updateFiles } from "@/project";
import { childGit, childPath, configScope, jcoreGit, jcorePath } from "@/constants";
import { logger } from "@/logger";
import { getFlagValue, copyFiles, nameToFolder } from "@/utils";
import { join } from "path";
import process from "process";

export function createProject(data: cmdData) {
  jcoreSettingsData.projectName = data.target[0];
  jcoreRuntimeData.workDir = join(process.cwd(), jcoreSettingsData.projectName);
  if (existsSync(jcoreRuntimeData.workDir)) {
    logger.warn("Project path exists: " + jcoreRuntimeData.workDir);
  } else {
    logger.info("Create Project: " + jcoreSettingsData.projectName);
    mkdirSync(jcoreRuntimeData.workDir);

    const settings: Record<string, configValue> = {
      projectName: jcoreSettingsData.projectName,
      localDomain: `${jcoreSettingsData.projectName}.localhost`,
      domains: [`${jcoreSettingsData.projectName}.localhost`],
    }

    const options = {
      cwd: jcoreRuntimeData.workDir,
      stdio: [0, 1, 2],
    };

    // Get the requested branch
    const branch = getFlagValue(data, "branch");

    if (branch) {
      jcoreSettingsData.branch = branch;
      settings.branch = branch;
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
        copyChildTheme(jcoreSettingsData.projectName);
        jcoreSettingsData.theme = jcoreSettingsData.projectName;
        settings.theme = jcoreSettingsData.projectName;
      }

      // Write config
      const configFile = getScopeConfigFile(configScope.PROJECT);
      updateConfigValues(settings, configFile)

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
  const themePath = join(jcoreRuntimeData.workDir, "wp-content/themes", jcoreSettingsData.theme);
  if (!existsSync(themePath)) {
    copyFiles(join(jcoreRuntimeData.workDir, childPath), themePath);
    replaceInFile(join(themePath, "style.css"), [
      { search: /^Theme Name:.*$/gm, replace: `Theme Name: ${name}` },
    ]);
    return true;
  }
  return false;
}
