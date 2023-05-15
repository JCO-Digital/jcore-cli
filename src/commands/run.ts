import { execSync, spawnSync } from "child_process";
import { join } from "path";
import { cmdData } from "@/types";
import { finalizeProject } from "@/project";
import { jcoreSettingsData } from "@/settings";
import { logger } from "@/logger";

export function start(data: cmdData) {
  if (!isRunning() && finalizeProject(data.flags.includes("install"))) {
    // Run only if finalize is successful.
    const options = {
      cwd: jcoreSettingsData.path,
      stdio: [0, 1, 2],
    };
    try {
      if (jcoreSettingsData.mode === "foreground") {
        spawnSync("docker-compose", ["up"], options);
      } else {
        execSync("docker-compose up -d", options);
      }
    } catch (e) {
      logger.error("Docker failed");
    }
  }
}

export function stop() {
  logger.info("Stopping Docker");

  const options = {
    cwd: jcoreSettingsData.path,
    stdio: [0, 1, 2],
  };

  try {
    execSync("docker-compose down", options);
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

export function isRunning(): boolean {
  let running = false;
  const options = {
    cwd: jcoreSettingsData.path,
  };
  const output = execSync("docker ps", options).toString();
  const search = output.matchAll(/^.*jcodigi\/wordpress:.*?([a-zA-Z0-9]+)-wordpress-[0-9]$/gm);
  for (const line of search) {
    const project = line[1] ?? "";
    logger.warn(`Project ${project} already running!`);
    running = true;
  }

  return running;
}

export function runCommand(command: string, spawn = false) {
  logger.verbose("Executing command on docker");

  const options = {
    cwd: jcoreSettingsData.path,
    stdio: [0, 1, 2],
  };

  try {
    if (spawn) {
      spawnSync("docker-compose", ["exec", "wordpress", command]);
    } else {
      execSync("docker-compose exec wordpress " + command, options);
    }
  } catch (e) {
    logger.warn("Command '" + command + "' failed to run.");
  }
}
