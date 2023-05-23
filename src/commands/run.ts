import { execSync, spawnSync } from "child_process";
import { join } from "path";
import { cmdData, jcoreProject } from "@/types";
import { finalizeProject } from "@/project";
import { jcoreSettingsData } from "@/settings";
import { logger } from "@/logger";
import { existsSync } from "fs";

export function start(data: cmdData) {
  if (finalizeProject(data.flags.includes("install"))) {
    // Run only if finalize is successful.

    const options = {
      cwd: jcoreSettingsData.path,
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

export function stop(path = jcoreSettingsData.path) {
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

export function pull(data: cmdData) {
  const scriptPath = "/project/.config/scripts";
  const pluginScript = join(scriptPath, "importplugins");
  const dbScript = join(scriptPath, "importdb");
  const mediaScript = join(scriptPath, "importmedia");
  const installScript = join(scriptPath, "installplugins");

  // Set initial run flags.
  const runFlags = {
    plugins: data.target.includes("plugins"),
    db: data.target.includes("db"),
    media: data.target.includes("media"),
  };

  if (data.target.length === 0) {
    // If no target given, default to plugins and db.
    runFlags.plugins = true;
    runFlags.db = true;
  } else if (data.target.includes("all")) {
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
      `docker volume ls -q --filter=label=com.docker.compose.project=${project.name}`
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
    execSync("docker image prune -f" + (all ? " -a" : ""), options);
    logger.info("Cleaning Volumes");
    execSync("docker volume prune -f" + (all ? " -a" : ""), options);
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
    runningProjects.forEach((project) => {
      logger.warn(`Project ${project.name} is running!`);
    });
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
    cwd: jcoreSettingsData.path,
    stdio: [0, 1, 2],
  };

  try {
    if (spawn) {
      spawnSync("docker compose", ["exec", "wordpress", command]);
    } else {
      execSync("docker compose exec wordpress " + command, options);
    }
  } catch (e) {
    logger.warn("Command '" + command + "' failed to run.");
  }
}
