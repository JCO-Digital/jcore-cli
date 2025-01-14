import { execSync } from "child_process";
import { existsSync, mkdirSync, rmSync, unlinkSync } from "fs";
import { join } from "path";
import {
  configScope,
  projectConfigFilename,
  templatesLocation,
  themeFolder,
} from "@/constants";
import { convertProjectSettings, projectConfigLegacyFilename } from "@/legacy";
import { logger } from "@/logger";
import { jcoreCmdData } from "@/parser";
import { finalizeProject, replaceInFile, updateFiles } from "@/project";
import {
  getScopeConfigFile,
  jcoreRuntimeData,
  jcoreSettingsData,
  readProjectSettings,
  updateConfigValues,
} from "@/settings";
import {
  configValue,
  jcoreSubmodule,
  jcoreTemplate,
  templateSchema,
} from "@/types";
import {
  compareChecksum,
  copyFiles,
  extractArchive,
  getFile,
  getFileString,
  getFlag,
  getFlagString,
  getUnzippedFolder,
  nameToFolder,
} from "@/utils";
import inquirer from "inquirer";
import process from "process";
import { parse as tomlParse } from "smol-toml";

export async function queryProject() {
  const projectData = {
    projectName: jcoreCmdData.target[0] ?? "",
    template: getFlagString("template") ?? "",
    branch: getFlagString("branch") ?? "",
    submodules: [] as jcoreSubmodule[],
  };
  const templates = new Map<string, jcoreTemplate>();

  const toml = await getFileString(templatesLocation);
  const data = tomlParse(toml);
  const templatesKeys = Object.keys(data);
  for (const template of templatesKeys) {
    templates.set(template, templateSchema.parse(data[template]));
  }

  const questions: Array<object> = [];
  if (!projectData.projectName) {
    questions.push({
      type: "input",
      name: "projectName",
      message: "Select a project name",
    });
  }
  if (!projectData.template) {
    questions.push({
      type: "list",
      name: "template",
      message: "Select a project template",
      choices: templatesKeys,
    });
  }

  if (questions.length) {
    const answers = await inquirer.prompt(questions);
    Object.assign(projectData, answers);
    // Empty the questions.
    questions.length = 0;
  }

  if (!templates.has(projectData.template)) {
    return Promise.reject(`Unknown Template: ${projectData.template}`);
  }

  const templateData = templates.get(projectData.template);
  if (!templateData) {
    return Promise.reject("Template Error");
  }

  if (!projectData.branch) {
    if (templateData.branches.length > 1) {
      questions.push({
        type: "list",
        name: "branch",
        message: "Select a branch",
        default: templateData.branch,
        choices: templateData.branches,
      });
    } else if (templateData.branch) {
      projectData.branch = templateData.branch;
    }
  }

  if (questions.length) {
    const answers = await inquirer.prompt(questions);
    Object.assign(projectData, answers);
  }

  jcoreSettingsData.projectName = projectData.projectName;
  jcoreSettingsData.template = projectData.template;
  jcoreSettingsData.branch = projectData.branch;

  // Template replacement.
  templateData.themeUrl = templateData.themeUrl.replace(
    /\{\{ *branch *\}\}/,
    projectData.branch,
  );

  createProject(templateData);
}

export function createProject(templateData: jcoreTemplate) {
  jcoreRuntimeData.workDir = join(process.cwd(), jcoreSettingsData.projectName);
  if (existsSync(jcoreRuntimeData.workDir)) {
    logger.error(`Project path exists: ${jcoreRuntimeData.workDir}`);
    return;
  }

  logger.info(`Create Project: ${jcoreSettingsData.projectName}`);
  mkdirSync(jcoreRuntimeData.workDir);

  const settings: Record<string, configValue> = {
    projectName: jcoreSettingsData.projectName,
    template: jcoreSettingsData.template,
    localDomain: `${jcoreSettingsData.projectName}.localhost`,
    domains: [`${jcoreSettingsData.projectName}.localhost`],
  };

  const options = {
    cwd: jcoreRuntimeData.workDir,
    stdio: [0, 1, 2],
  };

  if (jcoreSettingsData.branch) {
    settings.branch = jcoreSettingsData.branch;
  }

  // Run project update.
  updateFiles()
    .then(async () => {
      // Git init.
      execSync("git init", options);

      // Add git submodules.
      for (const submodule of templateData.submodules) {
        const branch =
          jcoreSettingsData.branch && submodule.useBranch
            ? `-b ${jcoreSettingsData.branch}`
            : "";
        execSync(
          `git submodule add -f ${branch} "${submodule.repo}" ${submodule.path}`,
          options,
        );
      }

      // Copy child theme.
      if (templateData.themeUrl && !getFlag("notheme")) {
        await createTheme(jcoreSettingsData.projectName, templateData.themeUrl);
        jcoreSettingsData.theme = jcoreSettingsData.projectName;
        settings.theme = jcoreSettingsData.projectName;
      }

      // Write config
      const configFile = getScopeConfigFile(configScope.PROJECT);
      updateConfigValues(settings, configFile);

      // GIT commit
      execSync("git add -A", options);
      execSync('git commit -m "Initial Commit"', options);

      // Finalize project.
      finalizeProject();
    })
    .catch((reason) => {
      logger.error(reason);
    });
}

export async function createTheme(name: string, url: string) {
  const tempThemePath = join(jcoreRuntimeData.workDir, themeFolder);

  try {
    const buffer = await getFile(url);
    await extractArchive(buffer, tempThemePath);
    logger.verbose("Unzipped");

    jcoreSettingsData.theme = nameToFolder(name);
    const themePath = join(
      jcoreRuntimeData.workDir,
      "wp-content/themes",
      jcoreSettingsData.theme,
    );

    if (!existsSync(themePath)) {
      copyFiles(getUnzippedFolder(tempThemePath), themePath);
      // Remove temporary files.
      rmSync(tempThemePath, {
        recursive: true,
        force: true,
      });
      replaceInFile(join(themePath, "style.css"), [
        { search: /^Theme Name:.*$/gm, replace: `Theme Name: ${name}` },
      ]);

      replaceInFile(join(jcoreRuntimeData.workDir, "Makefile"), [
        {
          search: /^theme :=.*$/gm,
          replace: `theme := ${join("wp-content/themes", jcoreSettingsData.theme)}`,
        },
      ]);

      return true;
    }
  } catch (reason) {
    logger.warn("Theme extraction failed.");
  }
  return false;
}

export async function migrateProject() {
  try {
    const options = {
      docker: false,
      package: false,
    };
    if (!compareChecksum("docker-compose.yml", true)) {
      options.docker = true;
      logger.warn("Checksum mismatch for docker-compose.yml.");
      const answer = await inquirer.prompt([
        {
          type: "confirm",
          name: "docker",
          message: "Overwrite docker-compose.yml",
        },
      ]);
      if (!answer.docker) {
        logger.warn("Aborting migaration.");
        return;
      }
    }
    if (!compareChecksum("package.json", true)) {
      options.package = true;
      logger.warn("Checksum mismatch for package.json.");
    }

    logger.info("Converting config file.");
    convertProjectSettings(projectConfigFilename);
    readProjectSettings();

    if (options.docker) {
      logger.info("Updating docker-compose.yml");
      updateFiles(["docker-compose.yml"]);
    }
    // Update project.
    updateFiles().then(() => {
      if (options.package) {
        // Add smol-toml to project.
        logger.info("Adding smol-toml to project.");
        const options = {
          cwd: jcoreRuntimeData.workDir,
          stdio: [0, 1, 2],
        };
        execSync("npm i -D smol-toml", options);
      }
    });

    // Delete old file.
    const localConfigLegacy = join(
      jcoreRuntimeData.workDir,
      projectConfigLegacyFilename,
    );
    unlinkSync(localConfigLegacy);
  } catch (e) {
    logger.error("Migration failed.");
  }
}
