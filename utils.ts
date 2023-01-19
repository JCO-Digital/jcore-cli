import {get} from "https";
import AdmZip from "adm-zip";
import {access} from "fs/promises";

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

/*
 * Simple wrapper that returns boolean promise whether a file exists of not.
 */
export async function fileExists(file: string): Promise<boolean> {
    try {
        await access(file);
        return true;
    } catch {
        return false;
    }
}
