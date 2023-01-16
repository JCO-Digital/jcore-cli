import {get} from "https";

export async function getFile(url: string): Promise<string> {
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