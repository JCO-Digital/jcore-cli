import type {cmdData, JcoreSettings, updateOptions} from "@/types";
import {getFileString} from "@/utils";
import {scriptLocation} from "@/constants";
import {writeFile} from "fs/promises";
import {version} from '@/package.json';
import {updateFiles} from "@/project";
import {error, log} from "console";

export default function (data: cmdData) {
    const options = {
        drone: data.flags.includes('force') || data.target.includes('drone'),
        package: data.flags.includes('force') || data.target.includes('package'),
        build: data.flags.includes('force') || data.target.includes('build'),
        composer: data.flags.includes('force') || data.target.includes('composer'),
        docker: data.flags.includes('force') || data.target.includes('docker'),
    } as updateOptions;

        log("Updating Project");
        updateFiles(options).then(() => {
            log('Update Finished');
        }).catch(reason => {
            error(reason);
        });
}

export function selfUpdate(data: cmdData) {
    versionCheck().then(info => {
        log("Upgrading to v." + info);
        getFileString(scriptLocation + 'jcore').then(body => {
            writeFile(data.execPath, body).then(() => {
                log("JCORE CLI Updated.");
            }).catch(reason => {
                error("Update Error");
                error(reason);
            });
        }).catch(reason => {
            error("Can't fetch update. Check network connection.")
        })
    }).catch(reason => {
        error(reason);
    });
}

function versionCheck(): Promise<string> {
    return new Promise((resolv, reject) => {
        getFileString(scriptLocation + 'package.json').then(json => {
            const info = JSON.parse(json);
            const oldVer = calcVersion(version);
            const newVer = calcVersion(info.version);
            if (newVer > oldVer) {
                resolv(info.version);
            } else {
                reject('No update available.');
            }
        }).catch(() => {
            reject('Network error.')
        });
    });
}

function calcVersion(version: string): number {
    let v = 0;
    let multi = 1;
    const parts = version.split('.');
    while (parts.length) {
        v += multi * Number(parts.pop());
        multi *= 1000;
    }
    return v;
}