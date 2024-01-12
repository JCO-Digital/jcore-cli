import { execSync, spawnSync } from "child_process";
import { existsSync } from "fs";
import { join } from "path";
import { logger } from "@/logger";
import { jcoreCmdData } from "@/parser";
import { finalizeProject } from "@/project";
import { jcoreRuntimeData, jcoreSettingsData } from "@/settings";
import { jcoreProject } from "@/types";
import { getFlag, getSetupFolder } from "@/utils";

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
    } catch (e) {
      logger.error("Docker failed");
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
  } catch (e) {
    logger.error("Docker failed");
  }
}

export function pull() {
  const scriptPath = getSetupFolder("scripts", true);
  const pluginScript = join(scriptPath, "importplugins");
  const dbScript = join(scriptPath, "importdb");
  const mediaScript = join(scriptPath, "importmedia");
  const installScript = join(scriptPath, "installplugins");

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
  } catch (e) {
    logger.error("Docker failed");
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
  } catch (e) {
    logger.error("Docker failed");
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
    const json = execSync("docker compose ls -a --format json").toString();
    for (const project of JSON.parse(json)) {
      const name: string = project.Name ?? "";
      const running: boolean = project.Status.includes("running");
      const configPaths: string = project.ConfigFiles ?? "";
      const path = configPaths.split(",")[0].replace("/docker-compose.yml", "");

      if (existsSync(join(path, ".jcore"))) {
        projects.push({
          name,
          path,
          running,
        });
      }
    }
  } catch (error) {
    // Foo
    logger.warn("Error in parsing docker compose setup");
  }

  return projects;
}

export function runCommand(command: string, spawn = false) {
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
  } catch (e) {
    logger.warn(`Command '${command}' failed to run.`);
  }
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
  } catch (e) {
    logger.warn("Failed to attach to logs");
  }
}
