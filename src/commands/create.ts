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
  lohkoTemplatePath,
  templatesLocation,
  tempUnzipFolder,
  lohkoBlockPath,
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
  slugify,
} from "@/utils";
import inquirer from "inquirer";
import process from "process";
import { parse as tomlParse } from "smol-toml";

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

export async function queryBlock(): Promise<void> {
  // Populate templatesKeys with sub-folders in lohkoTemplatePath.
  const templatesKeys: string[] = [];
  try {
    const entries = readdirSync(
      join(jcoreRuntimeData.workDir, lohkoTemplatePath),
    );
    for (const entry of entries) {
      const entryPath = join(
        jcoreRuntimeData.workDir,
        lohkoTemplatePath,
        entry,
      );
      const stats = lstatSync(entryPath);
      if (stats.isDirectory()) {
        templatesKeys.push(entry);
      }
    }
  } catch (error) {
    logger.error(`Error reading lohko templates directory: ${error}`);
  }

  const questions: Array<object> = [
    {
      type: "input",
      name: "name",
      message: "Enter a block name:",
    },
    {
      type: "list",
      name: "template",
      message: "Select a block template:",
      choices: templatesKeys,
    },
    {
      type: "input",
      name: "description",
      message: "Enter a block description:",
    },
  ];
  const answers = await inquirer.prompt(questions);
  if (answers.name && answers.template) {
    createBlock(answers.name, answers.template, answers.description);
  } else {
    logger.error("Invalid data, skipping block creation!");
  }
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

      // Copy additional files.
      if (templateData.files) {
        for (const file of templateData.files) {
          const destination = join(jcoreRuntimeData.workDir, file.path);
          await unzipFile(file.url, destination);
        }
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

/**
 * Creates a child theme by downloading, extracting, and configuring it.
 *
 * This function downloads a theme archive from the given URL, extracts it to a
 * temporary location, renames it to a folder derived from the project name,
 * moves it to the themes directory, and updates its style.css and the project's
 * Makefile to reflect the new theme name and path.
 *
 * @param {string} name - The desired name for the new child theme (e.g., project name).
 * @param {string} url - The URL of the theme archive to download.
 * @returns {Promise<boolean>} A promise that resolves to true if the theme was created successfully, false otherwise.
 */
export async function createTheme(name: string, url: string): Promise<boolean> {
  jcoreSettingsData.theme = slugify(name);
  const themePath = join(
    jcoreRuntimeData.workDir,
    "wp-content/themes",
    jcoreSettingsData.theme,
  );

  try {
    unzipFile(url, themePath);

    if (existsSync(themePath)) {
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

function createBlock(name: string, template: string, description: string) {
  const slug = slugify(name);
  const source = join(jcoreRuntimeData.workDir, lohkoTemplatePath, template);
  const destination = join(jcoreRuntimeData.workDir, lohkoBlockPath, slug);

  copyFiles(source, destination);

  // Change attributes in copied files to match given data.
  const blockFile = join(destination, "block.json");

  try {
    const blockFileContent = readFileSync(blockFile, "utf8");
    const blockData = JSON.parse(blockFileContent);
    blockData.title = name;
    blockData.name = `jcore/${slug}`;
    blockData.description = description;
    writeFileSync(blockFile, JSON.stringify(blockData, null, 2));

    replaceInFile(join(destination, "render.php"), [
      {
        search: /^Timber::render\( ?'[^']*'/gm,
        replace: `Timber::render( '@lohko/${slug}/view.twig'`,
      },
    ]);
  } catch (error) {
    logger.error(`Error reading or parsing block.json: ${error}`);
    return; // Stop block creation if file cannot be read/parsed
  }
}

/**
 * Downloads an archive from a URL, extracts it, and saves it to a destination folder.
 *
 * This function handles the process of fetching an archive file, typically a zip or tarball,
 * extracting its contents, and placing them into the specified destination directory.
 * It logs verbose information on success and a warning on failure.
 *
 * @param {string} url - The URL of the archive file to download.
 * @param {string} destination - The path to the folder where the archive contents should be extracted.
 * @returns {Promise<boolean>} A promise that resolves to `true` if the file was downloaded and extracted successfully, `false` otherwise.
 */
async function unzipFile(url: string, destination: string): Promise<boolean> {
  const tempUnzipPath = join(jcoreRuntimeData.workDir, tempUnzipFolder);
  try {
    const buffer = await getFile(url);
    await extractArchive(buffer, tempUnzipPath);
    logger.verbose(`Unzipped ${url} to ${destination}.`);
    if (!existsSync(destination)) {
      copyFiles(getUnzippedFolder(tempUnzipPath), destination);
    }
    // Remove temporary files.
    rmSync(tempUnzipPath, {
      recursive: true,
      force: true,
    });
    return true;
  } catch (reason) {
    logger.warn(`Unzipping ${url} failed with ${reason}.`);
  }
  return false;
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
