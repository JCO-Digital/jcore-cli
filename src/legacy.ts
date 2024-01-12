/*
 * Legacy functions.
 */

import { existsSync, readFileSync, unlinkSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";
import { logger } from "@/logger";
import {
  jcoreRuntimeData,
  jcoreSettingsData,
  saveConfigFile,
} from "@/settings";

const globalConfigLegacy = join(homedir(), ".config/jcore/config");
export const projectConfigLegacyFilename = "config.sh";

export function convertGlobalSettings(globalConfig: string) {
  if (!existsSync(globalConfig) && existsSync(globalConfigLegacy)) {
    // Read global settings if they exist.
    try {
      const values = new Map() as Map<string, string | string[]>;
      const data = readFileSync(globalConfigLegacy, "utf8");
      parseSettings(values, data);

      const logLevel = Number(values.get("loglevel"));

      saveConfigFile(globalConfig, {
        mode: values.get("mode"),
        debug: values.get("debug") === "true",
        logLevel: Number.isNaN(logLevel) ? 2 : logLevel,
        install: values.get("install") === "true",
      });
      unlinkSync(globalConfigLegacy);
    } catch (e) {
      logger.error("Global settings conversion failed.");
    }
  }
}

export function convertProjectSettings(projectConfigFilename: string) {
  const localConfig = join(jcoreRuntimeData.workDir, projectConfigFilename);
  const localConfigLegacy = join(
    jcoreRuntimeData.workDir,
    projectConfigLegacyFilename,
  );
  if (!existsSync(localConfig) && existsSync(localConfigLegacy)) {
    try {
      const values = new Map() as Map<string, string | string[]>;

      values.set("name", jcoreSettingsData.projectName);

      // Read and parse config.sh.
      const data = readFileSync(localConfigLegacy, "utf8");
      parseSettings(values, data);

      let newDomain = "";
      let newLocal = "";
      const domains: string[] = [];
      const replace = [];
      const domainsValue = values.get("domains");
      if (Array.isArray(domainsValue)) {
        for (const domain of domainsValue) {
          const parts = domain.split(";");
          const local = `${parts[1]}.localhost`;
          replace.push(`//${parts[0]}|//${local}`);
          if (!domains.includes(local)) {
            domains.push(local);
          }
          if (newDomain === "") {
            newDomain = parts[0];
            newLocal = local;
          }
        }
      }

      const newValues = {
        projectName: values.get("name"),
        template: "jcore2",
        theme: values.get("theme"),
        branch: values.get("branch"),
        remoteHost: values.get("remotehost"),
        remotePath: values.get("remotepath"),
        replace,
        domains,
        remoteDomain: newDomain,
        localDomain: newLocal,
        dbExclude: values.get("db_exclude"),
        pluginExclude: values.get("plugin_exclude"),
        pluginGit: values.get("plugin_git"),
        pluginInstall: values.get("plugin_install"),
        install: values.get("install") === "true",
      };
      saveConfigFile(localConfig, newValues);
      // Delete old file.
      unlinkSync(localConfigLegacy);
    } catch (e) {
      logger.error("Error converting project settings.");
    }
  }
}

function parseSettings(
  values: Map<string, string | string[]>,
  data: string,
): void {
  // Remove all comments to make matching more straight forward.
  let clean = data;
  for (const match of clean.matchAll(/ *#.*$/gm)) {
    clean = clean.replace(match[0], "");
  }

  // Look for all BASH variable assignments.
  for (const match of clean.matchAll(/^([A-Z_]+)= *([^(].*)$/gm)) {
    // Assign value to map.
    values.set(match[1].toLowerCase(), cleanBashVar(values, match[2]));
  }
  // Look for BASH arrays.
  for (const match of clean.matchAll(/^([A-Z_]+)= ?\(\s*([^)]+)\s*\)/gm)) {
    const value = [];
    for (const row of match[2].split("\n")) {
      const text = cleanBashVar(values, row);
      // Don't add empty lines to array.
      if (text) {
        value.push(text);
      }
    }
    // Assign array to map.
    values.set(match[1].toLowerCase(), value);
  }
}

function cleanBashVar(
  values: Map<string, string | string[]>,
  text: string,
): string {
  // Remove wrapping double quotes.
  let value = text.replace(/^["' ]+|["' ]+$/gm, "");

  // Look for all references to BASH variables.
  for (const varMatch of value.matchAll(/\$([A-Z_]+)/gm)) {
    const key = varMatch[1].toLowerCase();
    if (values.has(key)) {
      // If variable exists in map, substitute variable for value.
      const str = values.get(key);
      if (typeof str === "string") {
        value = value.replace(varMatch[0], str);
      }
    }
  }
  return value;
}
