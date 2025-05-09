import { existsSync, readFileSync, writeFileSync } from "fs";
import { join } from "path";
import { jcoreRuntimeData, jcoreSettingsData } from "./settings";
import { configValue } from "./types";
import { parseErrorHandler } from "./utils";
import { parse as tomlParse } from "smol-toml";

/**
 * Creates the .env file.
 */
export function createEnv() {
  const file = join(jcoreRuntimeData.workDir, "env-values.toml");
  if (existsSync(file)) {
    try {
      const toml = readFileSync(file, "utf8");
      const values = tomlParse(toml) as Record<string, configValue>;

      let env = "";
      for (const key in Object.assign(values, jcoreSettingsData)) {
        const value = values[key];
        if (key === "replace") {
          const defaultRow = `//${jcoreSettingsData.remoteDomain}|//${jcoreSettingsData.localDomain}`;
          if (Array.isArray(value) && !value.includes(defaultRow)) {
            value.push(defaultRow);
          }
        }
        env += `${createEnvName(key)}="${createEnvVariable(value)}"\n`;
      }

      writeFileSync(join(jcoreRuntimeData.workDir, ".env"), env);
    } catch (error) {
      parseErrorHandler(error, file);
      process.exit();
    }
  }
}

/**
 * Creates the env name.
 *
 * @param {string} name - The name of the env.
 * @returns {string} The created env name.
 */
function createEnvName(name: string): string {
  // Convert camelCase to UPPERCASE_SNAKE.
  return name.replace(/([A-Z])/g, "_$1").toUpperCase();
}

/**
 * Creates the env variable.
 *
 * @param {configValue} value - The value of the config.
 * @returns {string} The created env variable.
 */
function createEnvVariable(value: configValue): string {
  if (Array.isArray(value)) {
    const output: Array<string> = [];
    for (const row of value) {
      output.push(row);
    }
    return output.join(" ");
  }
  switch (typeof value) {
    case "string":
      return value;
    case "number":
      return value.toString();
    case "boolean":
      return value ? "true" : "false";
  }
}
