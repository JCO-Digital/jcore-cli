import {get} from "https";
import AdmZip from "adm-zip";

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
export async function getFile(url: string): Promise<Buffer> {
    return new Promise<Buffer>((resolve, reject) => {
        get(url).on('response', function (response) {
            if (response.statusCode === 200) {
                const data: Buffer[] = [];

                response.on('data', function(chunk) {
                    data.push(chunk);
                }).on('end', function() {
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

export async function extractArchive(buffer: Buffer, output: string) {
    try {
        const zip = new AdmZip(buffer);
        zip.extractAllTo(output);

        console.log(`Extracted to "${output}" successfully`);
    } catch (e) {
        console.log(`Something went wrong. ${e}`);
    }
}