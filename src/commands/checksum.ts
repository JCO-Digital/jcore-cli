import { existsSync } from "fs";
import { join } from "path";
import { logger } from "@/logger";
import { jcoreRuntimeData } from "@/settings";
import { calculateChecksum, loadChecksums, saveChecksums } from "@/checksums";

/**
 * Lists the saved checksums, and if they match
 */
export function listChecksums() {
  for (const [key, value] of loadChecksums()) {
    const fullPath = join(jcoreRuntimeData.workDir, key);
    if (existsSync(fullPath)) {
      const match = calculateChecksum(fullPath) === value ? "OK" : "Changed";
      logger.info(`Checksum for ${key}:`.padEnd(35) + match);
    } else {
      logger.warn(`${`File ${key}:`.padEnd(35)}Missing`);
    }
  }
}

export function setChecksum(files: string[]) {
  const checksums = loadChecksums();
  for (const file of files) {
    const filePath = join(jcoreRuntimeData.workDir, file);
    if (existsSync(filePath)) {
      const checksum = calculateChecksum(filePath);
      if (checksums.has(file) && checksum === checksums.get(file)) {
        logger.verbose(`File ${file} already correct.`);
      } else {
        checksums.set(file, checksum);
        logger.info(`File ${file} checksum set.`);
      }
    } else {
      logger.warn(`File ${file} doesn't exist.`);
    }
  }
  saveChecksums(checksums);
}
