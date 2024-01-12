import { readSettings, jcoreSettingsData, jcoreRuntimeData } from "@/settings";
import { existsSync } from "fs";
import { logger } from "@/logger";
import { execSync } from "child_process";
import { join, parse } from "path";
import process from "process";
import { childPath, jcorePath } from "@/constants";
import { finalizeProject } from "@/project";
import { jcoreCmdData } from "@/parser";

export function cloneProject() {
  let source = jcoreCmdData.target[0];
  if (jcoreCmdData.target[1]) {
    jcoreSettingsData.projectName = jcoreCmdData.target[1];
  }
  if (jcoreCmdData.target[0].search(/^[a-zA-Z0-9_-]+$/) !== -1) {
    // Target is just name of the project. Needs to be extended.
    source = `git@bitbucket.org:jcodigital/${source}.git`;
    if (!jcoreSettingsData.projectName) {
      // Set argument as project name if second argument not given.
      jcoreSettingsData.projectName = jcoreCmdData.target[0];
    }
  } else if (!jcoreSettingsData.projectName) {
    // If full git path and not second argument given, read project name from git path.
    const path = parse(jcoreCmdData.target[0]);
    jcoreSettingsData.projectName = path.name;
  }

  // Set project path.
  jcoreRuntimeData.workDir = join(process.cwd(), jcoreSettingsData.projectName);
  if (existsSync(jcoreRuntimeData.workDir)) {
    logger.warn(`Project path exists: ${jcoreRuntimeData.workDir}`);
  } else {
    logger.info(`Clone Project: ${jcoreSettingsData.projectName}`);

    const options = {
      cwd: process.cwd(),
      stdio: [0, 1, 2],
    };

    try {
      execSync(`git clone ${source} ${jcoreRuntimeData.workDir}`, options);
    } catch (reason) {
      logger.error("Clone Failed");
      return;
    }

    // After clone settings need to be read again. Run finalizeProject in anonymous function because of optional argument.
    readSettings()
      .then(setupProject)
      .then(() => finalizeProject());
  }
}

function setupProject() {
  const options = {
    cwd: jcoreRuntimeData.workDir,
    stdio: [0, 1, 2],
  };

  // Initialize submodules.
  try {
    execSync("git submodule update --init", options);
  } catch (reason) {
    logger.error("Submodules Failed");
    return;
  }

  if (jcoreSettingsData.branch) {
    try {
      // Switch jcore submodule to branch.
      options.cwd = join(jcoreRuntimeData.workDir, jcorePath);
      if (existsSync(options.cwd)) {
        execSync(`git switch ${jcoreSettingsData.branch}`, options);
      }

      // Switch jcore child submodule to branch.
      options.cwd = join(jcoreRuntimeData.workDir, childPath);
      if (existsSync(options.cwd)) {
        execSync(`git switch ${jcoreSettingsData.branch}`, options);
      }
    } catch (reason) {
      logger.error("Branch Switch Failed");
      return;
    }
  }

  logger.info("Clone complete.");
  logger.info(`Enter project folder with "cd ${jcoreRuntimeData.workDir}"`);
}
