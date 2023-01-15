import type {cmdData, JcoreSettings} from "@/types";
import {get} from "https";
export default function (cmd: cmdData, settings: JcoreSettings) {
    console.log("Update Project");
}

export function selfUpdate (settings: JcoreSettings) {
    const url = "https://files.jco.fi/jcore-cli-main/jcoree";
    console.log("Self Update");
    get(url).on('response', function (response) {
        console.log(response.statusCode);
        let body = '';
        response.on('data', function (chunk) {
            body += chunk;
        });
        response.on('end', function () {
            console.log(body);
            console.log('Finished');
        });
    });
}