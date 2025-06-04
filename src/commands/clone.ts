import { execSync } from "child_process";
import { existsSync } from "fs";
import { join, parse } from "path";
import { jcorePath } from "@/constants";
import { logger } from "@/logger";
import { jcoreCmdData } from "@/parser";
import { finalizeProject } from "@/project";
import { jcoreRuntimeData, jcoreSettingsData, readSettings } from "@/settings";
import process from "process";

export function cloneProject() {
  let source = jcoreCmdData.target[0];
  if (jcoreCmdData.target[1]) {
    jcoreSettingsData.projectName = jcoreCmdData.target[1];
  }
  if (jcoreCmdData.target[0].search(/^[a-zA-Z0-9_-]+$/) !== -1) {
    // Target is just name of the project. Needs to be extended.
    source = jcoreSettingsData.projectDefault.replace("{name}", source);
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
      if (reason instanceof Error) {
        logger.error("Clone Failed: ${reason.message}");
      }
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
    if (reason instanceof Error) {
      logger.error("Submodules Failed: ${reason.message}");
    }
    return;
  }

  if (jcoreSettingsData.branch) {
    try {
      // Switch jcore submodule to branch.
      options.cwd = join(jcoreRuntimeData.workDir, jcorePath);
      if (existsSync(options.cwd)) {
        execSync(`git switch ${jcoreSettingsData.branch}`, options);
      }
    } catch (reason) {
      if (reason instanceof Error) {
        logger.error("Branch Switch Failed: ${reason.message}");
      }
      return;
    }
  }

  logger.info("Clone complete.");
  logger.info(`Enter project folder with "cd ${jcoreRuntimeData.workDir}"`);
}
