import { createHash } from "crypto";
import { readFileSync, writeFileSync } from "fs";
import { join } from "path";
import { checksumFile } from "./constants";
import { jcoreRuntimeData } from "./settings";
import { loadJsonFile } from "./utils";

/**
 * Loads checksums from the checksum file.
 *
 * @returns {Map<string, string>} The loaded checksums.
 */
export function loadChecksums(): Map<string, string> {
  const data = loadJsonFile(join(jcoreRuntimeData.workDir, checksumFile));
  const checksums = new Map<string, string>();
  for (const [key, value] of Object.entries(data)) {
    if (typeof value === "string") {
      checksums.set(key, value);
    }
  }
  return checksums;
}

/**
 * Saves checksums from a Map to a JSON file.
 *
 * @param {Map<string, string>} checksums - The map of checksums to save.
 * @returns {boolean} True if saving was successful, false otherwise.
 */
export function saveChecksums(checksums: Map<string, string>): boolean {
  try {
    const object = Object.fromEntries(checksums);
    const json = JSON.stringify(object, null, 2);
    writeFileSync(join(jcoreRuntimeData.workDir, checksumFile), json, "utf8");
    return true;
  } catch {
    return false;
  }
}

/**
 * Compares the checksum of a file with the stored checksum.
 *
 * @param {string} file - The file to compare the checksum for.
 * @param {boolean} strict - Whether to perform a strict comparison.
 * @returns {boolean} True if the checksums match, false otherwise.
 */
export function compareChecksum(file: string, strict: boolean = true): boolean {
  const checksums = loadChecksums();
  if (!strict && checksums.get(file) === undefined) {
    // Return true for missing checksum in non-strict mode.
    return true;
  }
  return (
    checksums.get(file) ===
    calculateChecksum(join(jcoreRuntimeData.workDir, file))
  );
}

/**
 * Updates the checksum of a file.
 *
 * @param {string} file - The file to update the checksum for.
 */
export function updateChecksum(file: string) {
  const checksums = loadChecksums();
  checksums.set(file, calculateChecksum(file));
  saveChecksums(checksums);
}

/**
 * Calculates the checksum of a file.
 *
 * @param {string} file - The file to calculate the checksum for.
 * @returns {string} The calculated checksum.
 */
export function calculateChecksum(file: string): string {
  try {
    const data = readFileSync(file, "utf8");
    return createHash("sha256").update(data).digest("hex");
  } catch (e) {
    return "";
  }
}
