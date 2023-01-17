import type {cmdData, JcoreSettings} from "@/types";
import {extractArchive, getFile, getFileString} from "@/utils";
import {archiveLocation, scriptLocation, updateFolder} from "@/constants";
import {writeFile} from "fs/promises";
import {version} from '@/package.json';
import {join} from "path";

export default function (data: cmdData, settings: JcoreSettings) {
    console.log("Update Project");
    getFile(archiveLocation).then(buffer => {
        console.log(settings);
        extractArchive(buffer, join(settings.path, updateFolder)).then(() => {
            console.log("test");
        });
    })
}

export function selfUpdate(data: cmdData) {
    versionCheck().then(info => {
        console.log("Upgrading to v." + info);
        getFileString(scriptLocation + 'jcore').then(body => {
            writeFile(data.execPath, body).then(() => {
                console.log("JCORE CLI Updated.");
            }).catch(reason => {
                console.error("Update Error");
                console.error(reason);
            });
        }).catch(reason => {
            console.error("Can't fetch update. Check network connection.")
        })
    }).catch(reason => {
        console.error(reason);
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