import { configValue, jcoreSubmodule, jcoreTemplate, templateSchema } from "@/types";
import {
  getScopeConfigFile,
  jcoreRuntimeData,
  jcoreSettingsData,
  updateConfigValues,
} from "@/settings";
import { existsSync, mkdirSync } from "fs";
import { execSync } from "child_process";
import { finalizeProject, replaceInFile, updateFiles } from "@/project";
import { childPath, configScope, templatesLocation } from "@/constants";
import { logger } from "@/logger";
import { getFlag, copyFiles, nameToFolder, getFlagString, getFileString } from "@/utils";
import { join } from "path";
import process from "process";
import { jcoreCmdData } from "@/parser";
import inquirer from "inquirer";
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
    return Promise.reject(`Template Error`);
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

  createProject(templateData);
}

export function createProject(templateData: jcoreTemplate) {
  jcoreRuntimeData.workDir = join(process.cwd(), jcoreSettingsData.projectName);
  if (existsSync(jcoreRuntimeData.workDir)) {
    logger.error("Project path exists: " + jcoreRuntimeData.workDir);
    return;
  }

  logger.info("Create Project: " + jcoreSettingsData.projectName);
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
    .then(() => {
      // Git init.
      execSync("git init", options);

      // Add git submodules.
      templateData.submodules.forEach((submodule) => {
        const branch =
          jcoreSettingsData.branch && submodule.useBranch ? `-b ${jcoreSettingsData.branch}` : "";
        execSync(`git submodule add -f ${branch} "${submodule.repo}" ${submodule.path}`, options);
      });

      // Copy child theme.
      if (templateData.child && !getFlag("nochild")) {
        copyChildTheme(jcoreSettingsData.projectName);
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
