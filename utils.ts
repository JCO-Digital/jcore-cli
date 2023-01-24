import {get} from "https";
import AdmZip from "adm-zip";
import {access, readFile, writeFile} from "fs/promises";
import {JcoreSettings} from "@/types";
import {join} from "path";
import {createHash} from "crypto";
import {checksumFile} from "@/constants";
import {existsSync, lstatSync, mkdirSync, readdirSync, renameSync} from "fs";

export async function getFileString(url: string): Promise<string> {
    return new Promise<string>((resolve, reject) => {
        get(url).on('response', function (response) {
            if (response.statusCode === 200) {
                let body = '';
                response.on('data', function (chunk) {
                    body += chunk;
                });
                response.on('end', function () {
                    resolve(body);
                });
            } else {
                reject(response.statusCode);
            }
        });
    });
}

export function getFile(url: string): Promise<Buffer> {
    return new Promise<Buffer>((resolve, reject) => {
        get(url).on('response', function (response) {
            if (response.statusCode === 200) {
                const data: Buffer[] = [];

                response.on('data', function (chunk) {
                    data.push(chunk);
                }).on('end', function () {
                    //at this point data is an array of Buffers
                    //so Buffer.concat() can make us a new Buffer
                    //of all of them together
                    resolve(Buffer.concat(data));
                });
            } else {
                reject(response.statusCode);
            }
        });
    });
}

export function extractArchive(buffer: Buffer, output: string): Promise<void> {
    return new Promise((resolve, reject) => {
        try {
            const zip = new AdmZip(buffer);
            zip.extractAllTo(output);

            resolve();
        } catch (e) {
            reject(e);
        }
    });
}

export async function loadChecksums(settings: JcoreSettings): Promise<Map<string, string>> {
    try {
        const json = await readFile(join(settings.path, checksumFile), 'utf8');
        const data = JSON.parse(json);
        return new Map(Object.entries(data));
        ;
    } catch {
        return new Map<string, string>();
    }
}

export async function saveChecksums(settings: JcoreSettings, checksums: Map<string, string>): Promise<boolean> {
    try {
        const object = Object.fromEntries(checksums)
        const json = JSON.stringify(object);
        await writeFile(join(settings.path, checksumFile), json, 'utf8');
        return true;
    } catch {
        return false
    }
}

export async function calculateChecksum(file: string): Promise<string> {
    return access(file)
        .then(() => readFile(file, 'utf8'))
        .then(data => createHash('sha256').update(data).digest('hex'))
        .catch(reason => '');
}

export function mergeFiles(sourceDir: string, destinationDir: string) {
    for (let file of readdirSync(sourceDir)) {
        if (lstatSync(join(sourceDir, file)).isDirectory()) {
            if (!existsSync(join(destinationDir, file))) {
                mkdirSync(join(destinationDir, file));
            }
            mergeFiles(join(sourceDir, file), join(destinationDir, file))
        } else {
            renameSync(join(sourceDir, file), join(destinationDir, file));
        }
    }

}