import { execSync } from "child_process";
import {
  existsSync,
  lstatSync,
  mkdirSync,
  readdirSync,
  readFileSync,
  rmSync,
  unlinkSync,
  writeFileSync,
} from "fs";
import { join } from "path";
import {
  configScope,
  projectConfigFilename,
  templatesLocation,
  lohkoBlockPath,
  lohkoTemplateLocation,
  lohkoPath,
  lohkoLocation,
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
  Choice,
  configValue,
  jcoreSubmodule,
  jcoreTemplate,
  templateSchema,
} from "@/types";
import { compareChecksum } from "@/checksums";
import { getFileString, copyFiles, unzipFile } from "@/fileHelpers";
import { getFlag, getFlagString, slugify } from "@/utils";
import process from "process";
import { parse as tomlParse } from "smol-toml";
import { input, select, confirm, checkbox, password } from "@inquirer/prompts";
import Mustache from "mustache";

/**
 * Creates a new project based on user input or provided parameters.
 *
 * This function prompts the user for project details if not provided via command line arguments,
 * validates the selected template, and initiates the project creation process. It handles
 * template selection, branch selection, and prepares the project configuration before calling
 * the createProject function.
 *
 * @returns {Promise<void>} A promise that resolves when the project query and setup are complete
 * @throws {Promise<string>} Rejects with an error message if the template is invalid
 */
export async function queryProject(): Promise<void> {
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

  if (!projectData.projectName) {
    projectData.projectName = await input({
      message: "Select a project name:",
    });
  }

  if (!projectData.projectName) {
    return Promise.reject("No project name given.");
  }

  if (!projectData.template) {
    projectData.template = await select({
      message: "Select a project template",
      choices: templatesKeys,
    });
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
      projectData.branch = await select({
        message: "Select a branch",
        default: templateData.branch,
        choices: templateData.branches,
      });
    } else if (templateData.branch) {
      projectData.branch = templateData.branch;
    }
  }
  if (!projectData.branch) {
    return Promise.reject("No branch selected.");
  }

  jcoreSettingsData.projectName = projectData.projectName;
  jcoreSettingsData.template = projectData.template;
  jcoreSettingsData.branch = projectData.branch;

  // Template replacement.
  templateData.themeUrl = Mustache.render(templateData.themeUrl, projectData);

  createProject(templateData);
}

export async function queryBlock(): Promise<void> {
  // Check if Lohko exists.
  await queryLohko();

  const blockData = {
    name: jcoreCmdData.target[1] ?? "",
    template: getFlagString("template") ?? "",
    description: "",
  };

  if (!blockData.name) {
    blockData.name = await input({
      message: "Enter a block name:",
    });
  }
  if (!blockData.name) {
    return Promise.reject("No block name given.");
  }

  // Populate templatesKeys with sub-folders in lohkoTemplatePath.
  const templatesKeys: string[] = [];
  const lohkoTemplates = await unzipFile(lohkoTemplateLocation);
  try {
    const entries = readdirSync(lohkoTemplates);
    for (const entry of entries) {
      const entryPath = join(lohkoTemplates, entry);
      const stats = lstatSync(entryPath);
      if (stats.isDirectory()) {
        templatesKeys.push(entry);
      }
    }
  } catch (error) {
    logger.error(`Error reading lohko templates directory: ${error}`);
  }

  if (!blockData.template) {
    if (templatesKeys.length === 0) {
      return Promise.reject("No block templates found.");
    }
    blockData.template = await select({
      message: "Select a block template:",
      choices: templatesKeys,
    });
  }
  if (!templatesKeys.includes(blockData.template)) {
    return Promise.reject(`Unknown Template: ${blockData.template}`);
  }

  // If no description is given, ask for it. But description is optional.
  if (!blockData.description) {
    blockData.description = await input({
      message: "Enter a block description:",
    });
  }

  createBlock(
    blockData.name,
    join(lohkoTemplates, blockData.template),
    blockData.description,
  );

  logger.debug("Cleaning up template folder.");
  rmSync(lohkoTemplates, {
    recursive: true,
    force: true,
  });
}

async function queryLohko(defaultInstall = false): Promise<void> {
  if (!existsSync(lohkoPath)) {
    if (!defaultInstall) {
      const install = await confirm({
        message: "Lohko is not installed, do you want to install it?",
        default: true,
      });
      if (!install) {
        return Promise.reject("Lohko will not be installed, aborting!");
      }
    }

    const destination = join(jcoreRuntimeData.workDir, lohkoPath);
    await unzipFile(lohkoLocation, destination).catch((reason) => {
      return Promise.reject(reason);
    });

    const blockChoices: Array<Choice<string>> = [];
    const lohkoFiles = readdirSync(
      join(jcoreRuntimeData.workDir, lohkoBlockPath),
    );
    lohkoFiles.forEach((file) => {
      const blockFile = join(
        jcoreRuntimeData.workDir,
        lohkoBlockPath,
        file,
        "block.json",
      );
      if (existsSync(blockFile)) {
        const blockData = JSON.parse(readFileSync(blockFile, "utf8"));
        if (blockData.title) {
          blockChoices.push({
            name: blockData.title,
            description: blockData.description,
            value: file,
            checked: defaultInstall,
          });
        }
      }
    });
    const keepBlocks = await checkbox({
      message: "Select Lohko blocks to install:",
      choices: blockChoices,
    });
    lohkoFiles.forEach((file) => {
      if (!keepBlocks.includes(file)) {
        const blockPath = join(jcoreRuntimeData.workDir, lohkoBlockPath, file);
        if (existsSync(blockPath)) {
          rmSync(blockPath, {
            recursive: true,
            force: true,
          });
          logger.debug(`Removed Lohko block: ${file}`);
        }
      }
    });

    logger.info(`Lohko installed to ${destination}`);
  }
}

export async function queryUser(): Promise<void> {
  const userData = {
    name: jcoreCmdData.target[1] ?? "",
    email: jcoreCmdData.target[2] ?? "",
    password: jcoreCmdData.target[3] ?? "",
    role: "",
  };

  if (!userData.name) {
    userData.name = await input({
      message: "Enter username:",
      validate: (value) => value.match(/^[a-z0-9]+$/) !== null,
    });
  }
  if (!userData.email) {
    userData.email = await input({
      message: "Enter email:",
      default: userData.name + "@example.com",
      validate: (value) =>
        value.match(/^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/) !==
        null,
    });
  }
  if (!userData.password) {
    userData.password = await password({
      message: "Enter password:",
      mask: true,
    });
  }
  const roleChoices: Array<Choice<string>> = [
    {
      name: "Administrator",
      description:
        "The highest level of permission. Admins have the power to access almost everything.",
      value: "administrator",
    },
    {
      name: "Editor",
      description:
        "Has access to all posts, pages, comments, categories, and tags, and can upload media.",
      value: "editor",
    },
    {
      name: "Author",
      description:
        "Can write, upload media, edit, and publish their own posts.",
      value: "author",
    },
    {
      name: "Contributor",
      description:
        "Has no publishing or uploading capability but can write and edit their own posts until they are published.",
      value: "contributor",
    },
    {
      name: "Viewer",
      description:
        "Viewers can read and comment on posts and pages on private sites.",
      value: "viewer",
    },
    {
      name: "Subscriber",
      description: "People who subscribe to your site’s updates.",
      value: "subscriber",
    },
  ];
  userData.role = await select({
    message: "Select user role",
    choices: roleChoices,
  });
  console.debug(userData);
}

/**
 * Creates a new project using the specified template data.
 *
 * This function sets up a new project directory, initializes git, adds required submodules,
 * creates a child theme if applicable, and configures the project settings. It handles the
 * actual project creation after queryProject() has gathered all necessary information.
 *
 * @param {jcoreTemplate} templateData - The template configuration for the project
 * @returns {void}
 */
export function createProject(templateData: jcoreTemplate): void {
  jcoreRuntimeData.workDir = join(
    process.cwd(),
    slugify(jcoreSettingsData.projectName),
  );
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
        const themeName = await createTheme(
          jcoreSettingsData.projectName,
          templateData.themeUrl,
        );
        settings.theme = themeName;
      }

      // Copy additional files.
      if (templateData.files) {
        for (const file of templateData.files) {
          const destination = join(jcoreRuntimeData.workDir, file.path);
          await unzipFile(file.url, destination);
        }
      }

      await queryLohko(templateData.lohko);

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

/**
 * Creates a child theme by downloading, extracting, and configuring it.
 *
 * This function downloads a theme archive from the given URL, extracts it to a
 * temporary location, renames it to a folder derived from the project name,
 * moves it to the themes directory, and updates its style.css and the project's
 * Makefile to reflect the new theme name and path.
 *
 * @param {string} themeName - The desired name for the new child theme (e.g., project name).
 * @param {string} url - The URL of the theme archive to download.
 * @returns {Promise<string>} The name of the theme folder of successful.
 */
export async function createTheme(
  themeName: string,
  url: string,
): Promise<string> {
  logger.info(`Creating theme ${themeName}.`);
  jcoreSettingsData.theme = slugify(themeName);
  const themePath = join(
    jcoreRuntimeData.workDir,
    "wp-content/themes",
    jcoreSettingsData.theme,
  );

  try {
    const location = await unzipFile(url, themePath, {
      ...jcoreSettingsData,
      themeName,
    });
    logger.debug(`Files unzipped to ${location}`);
    if (existsSync(themePath)) {
      logger.debug(`Replacing theme name in style.css with ${themeName}.`);
      replaceInFile(join(themePath, "style.css"), [
        { search: /^Theme Name:.*$/gm, replace: `Theme Name: ${themeName}` },
      ]);

      logger.debug(
        `Replacing theme in Makefile with ${jcoreSettingsData.theme}.`,
      );
      replaceInFile(join(jcoreRuntimeData.workDir, "Makefile"), [
        {
          search: /^theme := .*$/gm,
          replace: `theme := ${join("wp-content/themes", jcoreSettingsData.theme)}`,
        },
      ]);

      logger.debug(
        `Replacing theme in pnpm-workspace.yaml with ${jcoreSettingsData.theme}.`,
      );
      replaceInFile(join(jcoreRuntimeData.workDir, "pnpm-workspace.yaml"), [
        {
          search: /wp-content\/themes\/[^"]+/gm,
          replace: join("wp-content/themes", jcoreSettingsData.theme),
        },
      ]);

      return jcoreSettingsData.theme;
    } else {
      logger.error(`Creation of theme in folder ${themePath} failed!`);
    }
  } catch (reason) {
    if (reason instanceof Error) {
      logger.error("Theme extraction failed: ${reason.message}");
    }
  }
  return "";
}

function createBlock(name: string, template: string, description: string) {
  const slug = slugify(name);
  const destination = join(jcoreRuntimeData.workDir, lohkoBlockPath, slug);

  if (existsSync(destination)) {
    logger.error(`Block path exists: ${destination}`);
    return;
  }

  copyFiles(template, destination, {
    name,
    slug,
    description,
  });

  // Change attributes in copied files to match given data.
  const blockFile = join(destination, "block.json");

  try {
    const blockFileContent = readFileSync(blockFile, "utf8");
    const blockData = JSON.parse(blockFileContent);
    blockData.title = name;
    blockData.name = `jcore/${slug}`;
    blockData.description = description;
    writeFileSync(blockFile, JSON.stringify(blockData, null, "\t"), "utf8");
  } catch (error) {
    logger.error(`Error reading or parsing block.json: ${error}`);
    return; // Stop block creation if file cannot be read/parsed
  }
}

/**
 * Migrates an existing project to the latest configuration format and updates necessary files.
 *
 * This function handles the conversion of the old project configuration file format,
 * potentially updates `docker-compose.yml` and `package.json` based on checksum checks
 * and user confirmation, and installs necessary dependencies.
 * It also removes the old configuration file after successful migration.
 *
 * @returns {Promise<void>} A promise that resolves when the migration is complete or rejects on failure.
 */
export async function migrateProject(): Promise<void> {
  try {
    const options = {
      docker: false,
      package: false,
    };
    if (!compareChecksum("docker-compose.yml", true)) {
      options.docker = true;
      logger.warn("Checksum mismatch for docker-compose.yml.");
      const docker = await confirm({
        message: "Overwrite docker-compose.yml",
      });
      if (!docker) {
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
  } catch (error) {
    if (error instanceof Error) {
      logger.error(`Migration failed: ${error.message}`);
    }
  }
}
