import { execSync, spawnSync } from "child_process";
import { join } from "path";
import { cmdData } from "@/types";
import { finaliseProject } from "@/project";
import { settings } from "@/settings";
import { logger } from "@/logger";

export function start(data: cmdData) {
  finaliseProject();

  const options = {
    cwd: settings.path,
    stdio: [0, 1, 2],
  };
  if (settings.mode === "foreground") {
    spawnSync("docker-compose", ["up"], options);
  } else {
    execSync("docker-compose up -d", options);
  }
}

export function stop(data: cmdData) {
  logger.info("Stopping Docker");

  const options = {
    cwd: settings.path,
    stdio: [0, 1, 2],
  };

  execSync("docker-compose down", options);
}
export function pull(data: cmdData) {
  const options = {
    cwd: settings.path,
    stdio: [0, 1, 2],
  };

  console.debug(data);

  const pluginScript = join("/project/.config/scripts", "importplugins");
  const dbScript = join("/project/.config/scripts", "importdb");
  const mediaScript = join("/project/.config/scripts", "importmedia");
  const installScript = join("/project/.config/scripts", "installplugins");

  if (data.target.includes("plugins") || data.target.includes("all")) {
    execSync("docker-compose exec wordpress " + pluginScript, options);
  }
  if (data.target.includes("db") || data.target.includes("all")) {
    execSync("docker-compose exec wordpress " + dbScript, options);
  }
  if (
    data.target.includes("plugins") ||
    data.target.includes("db") ||
    data.target.includes("all")
  ) {
    execSync("docker-compose exec wordpress " + installScript, options);
  }
  if (data.target.includes("media") || data.target.includes("all")) {
    execSync("docker-compose exec wordpress " + mediaScript, options);
  }
}

export function executeCommand(data: cmdData) {
  logger.verbose("Executing command on docker");

  const options = {
    cwd: settings.path,
    stdio: [0, 1, 2],
  };

  spawnSync("docker-compose", ["exec", "wordpress", "/bin/bash"], options);
}
