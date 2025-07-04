import { z } from "zod";
import { execSync, spawnSync } from "child_process";
import { existsSync, renameSync, rmSync, unlinkSync } from "fs";
import { join } from "path";
import { logger } from "@/logger";
import { jcoreCmdData } from "@/parser";
import { finalizeProject } from "@/project";
import { jcoreRuntimeData, jcoreSettingsData } from "@/settings";
import { jcoreProject } from "@/types";
import { createEnv } from "@/env";
import {
  errorHandler,
  getFlag,
  getFlagString,
  getProjectFolder,
  getSetupFolder,
} from "@/utils";

export function start() {
  if (finalizeProject(getFlag("install"))) {
    // Run only if finalize is successful.

    const options = {
      cwd: jcoreRuntimeData.workDir,
      stdio: [0, 1, 2],
    };
    try {
      if (jcoreSettingsData.mode === "foreground") {
        execSync("docker compose up", options);
      } else {
        execSync("docker compose up -d", options);
      }
    } catch (error) {
      errorHandler(error, "Docker failed");
    }
  }
}

export function stop(path = jcoreRuntimeData.workDir) {
  logger.debug("Stopping Docker");

  const options = {
    cwd: path,
    stdio: [0, 1, 2],
  };

  try {
    execSync("docker compose stop", options);
  } catch (error) {
    errorHandler(error, "Docker failed");
  }
}

export function pull() {
  const scriptPath = getSetupFolder("scripts", true);
  const pluginScript = join(scriptPath, "importplugins");
  const dbScript = join(scriptPath, "importdb");
  const mediaScript = join(scriptPath, "importmedia");
  const installScript = join(scriptPath, "installplugins");

  createEnv();

  const dbFile = getFlagString("dbfile");
  const jcoreSqlPath = getProjectFolder("sql");
  let dbPath: undefined | string;
  if (dbFile !== undefined) {
    dbPath = join(jcoreSqlPath, dbFile);
  }
  if (dbPath !== undefined && existsSync(dbPath)) {
    logger.info("Moving selected database file to update.sql");
    const updatePath = join(jcoreSqlPath, "update.sql");
    if (existsSync(updatePath)) {
      unlinkSync(updatePath);
    }
    renameSync(dbPath, updatePath);
  } else if (dbPath !== undefined && !existsSync(dbPath)) {
    logger.error(`Specified database file does not exist: ${dbPath}`);
  }

  // Set initial run flags.
  const runFlags = {
    plugins: jcoreCmdData.target.includes("plugins"),
    db: jcoreCmdData.target.includes("db"),
    media: jcoreCmdData.target.includes("media"),
  };

  if (jcoreCmdData.target.length === 0) {
    // If no target given, default to plugins and db.
    runFlags.plugins = true;
    runFlags.db = true;
  } else if (jcoreCmdData.target.includes("all")) {
    // If "all", set all flags.
    runFlags.plugins = true;
    runFlags.db = true;
    runFlags.media = true;
  }

  // Run commands according to flags.
  if (runFlags.plugins) {
    runCommand(pluginScript);
  }
  if (runFlags.db) {
    runCommand(dbScript);
  }
  if (runFlags.plugins || runFlags.db) {
    runCommand(installScript);
  }
  if (runFlags.media) {
    runCommand(mediaScript);
  }
}

export function cleanProject(project: jcoreProject) {
  const options = {
    cwd: project.path,
    stdio: [0, 1, 2],
  };

  try {
    logger.info(`Cleaning containers for ${project.name}.`);
    execSync("docker compose rm -f", options);
    logger.info(`Cleaning volumes for project ${project.name}.`);
    const output = execSync(
      `docker volume ls -q --filter=label=com.docker.compose.project=${project.name}`,
    ).toString();
    for (const volume of output.split(/[\r\n]+/)) {
      if (volume.length) {
        logger.debug(`Deleting volume ${volume}.`);
        execSync(`docker volume rm ${volume}`, options);
      }
    }
    logger.debug("Deleting .jcore workfiles.");
    rmSync(join(project.path, ".jcore"), {
      recursive: true,
      force: true,
    });
  } catch (error) {
    errorHandler(error, "Docker failed");
  }
}

export function cleanAll() {
  for (const project of getProjects()) {
    if (!project.running) {
      cleanProject(project);
    }
  }
  cleanDocker(true);
}

export function cleanDocker(all = false) {
  const options = {
    stdio: [0, 1, 2],
  };

  try {
    logger.info("Cleaning Containers");
    execSync("docker container prune -f", options);
    logger.info("Cleaning Images");
    execSync(`docker image prune -f ${all ? " -a" : ""}`, options);
    logger.info("Cleaning Volumes");
    execSync(`docker volume prune -f ${all ? " -a" : ""}`, options);
    logger.info("Cleaning Networks");
    execSync("docker network prune -f", options);
  } catch (error) {
    errorHandler(error, "Docker failed");
  }
}

export function isRunning(messageIfStopped = true): boolean {
  const runningProjects = getRunning();

  if (messageIfStopped) {
    if (runningProjects.length === 0) {
      logger.warn("Project is not running!");
    }
  } else {
    for (const project of runningProjects) {
      logger.warn(`Project ${project.name} is running!`);
    }
  }

  return runningProjects.length > 0;
}

export function getRunning(): jcoreProject[] {
  return getProjects().filter((project) => project.running);
}

function getProjects(): jcoreProject[] {
  const projects: jcoreProject[] = [];
  try {
    const dockerProjects = runJson("docker compose ls -a --format json");
    if (Array.isArray(dockerProjects)) {
      for (const project of dockerProjects) {
        const name: string = project.Name ?? "";
        const running: boolean = project.Status.includes("running");
        const configPaths: string = project.ConfigFiles ?? "";
        const path = configPaths
          .split(",")[0]
          .replace("/docker-compose.yml", "");

        if (existsSync(join(path, ".jcore"))) {
          projects.push({
            name,
            path,
            running,
          });
        }
      }
    }
  } catch (error) {
    errorHandler(error, "Docker failed");
  }

  return projects;
}

export function runCommand(command: string, spawn = false): Promise<void> {
  logger.verbose("Executing command on docker");

  const options = {
    cwd: jcoreRuntimeData.workDir,
    stdio: [0, 1, 2],
  };

  try {
    if (spawn) {
      spawnSync("docker compose", ["exec", "wordpress", command]);
    } else {
      execSync(`docker compose exec wordpress ${command}`, options);
    }
  } catch (error) {
    if (error instanceof Error) {
      logger.debug(error.message);
    }
    return Promise.reject(`Command '${command}' failed to run.`);
  }
  return Promise.resolve();
}

export function attach() {
  const options = {
    cwd: jcoreRuntimeData.workDir,
    stdio: [0, 1, 2],
  };
  logger.info("Attaching to logs");
  try {
    const command = `docker compose logs -f --since 5m ${jcoreCmdData.target}`;
    execSync(command, options);
  } catch (error) {
    errorHandler(error, "Failed to attach to logs");
  }
}

function runJson(command: string): object {
  try {
    const output = execSync(command);
    const json = output.toString();
    const data = JSON.parse(json);
    return data;
  } catch (error) {
    errorHandler(error);
  }
  return [];
}

const pnpmListSchema = z.array(
  z.object({
    dependencies: z.record(
      z.object({
        from: z.string(),
        version: z.string(),
      }),
    ),
  }),
);

export function isPnpm(): boolean {
  const data = runJson("pnpm list -g --json");

  try {
    const list = pnpmListSchema.parse(data);
    return list[0].dependencies["@jcodigital/jcore-cli"] !== undefined;
  } catch (error) {
    errorHandler(error);
  }
  return false;
}

export function updatePnpm() {
  const options = {
    cwd: jcoreRuntimeData.workDir,
    stdio: [0, 1, 2],
  };

  execSync("pnpm update -g @jcodigital/jcore-cli", options);
}
