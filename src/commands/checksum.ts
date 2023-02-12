import { calculateChecksum, loadChecksums, saveChecksums } from "@/utils";
import { logger } from "@/logger";
import { join } from "path";
import { settings } from "@/settings";
import { existsSync } from "fs";

export function listChecksums() {
  for (const [key, value] of loadChecksums()) {
    const match = calculateChecksum(join(settings.path, key)) === value ? "OK" : "Changed";
    logger.info(`Checksum for ${key}:`.padEnd(35) + match);
  }
}

export function setChecksum(files: string[]) {
  const checksums = loadChecksums();
  for (const file of files) {
    const filePath = join(settings.path, file);
    if (existsSync(filePath)) {
      const checksum = calculateChecksum(filePath);
      if (checksums.has(file) && checksum === checksums.get(file)) {
        logger.verbose(`File ${file} already correct.`);
      } else {
        checksums.set(file,checksum);
        logger.info(`File ${file} checksum set.`);
      }
    } else {
      logger.warn(`File ${file} doesn't exist.`);
    }
  }
  saveChecksums(checksums);
}
