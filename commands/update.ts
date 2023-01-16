import type {cmdData, JcoreSettings} from "@/types";
import {getFile} from "@/utils";
import {scriptLocation} from "@/constants";
import {writeFile} from "fs/promises";
import {version} from '@/package.json';

export default function (cmd: cmdData, settings: JcoreSettings) {
    console.log("Update Project");
}

export function selfUpdate(cmd: cmdData, settings: JcoreSettings) {
    getFile(scriptLocation + 'package.json').then(json => {
        const info = JSON.parse(json);
        const oldVer = calcVersion(version);
        const newVer = calcVersion(info.version);
        if (newVer > oldVer) {
            console.log("Upgrading to v." + info.version);
            getFile(scriptLocation + 'jcore').then(body => {
                writeFile(cmd.execPath, body).then(() => {
                    console.log("JCORE CLI Updated.");
                }).catch(reason => {
                    console.error("Update Error");
                    console.error(reason);
                });
            }).catch(reason => {
                console.error("Can't fetch update. Check network connection.")
            })
        } else {
            console.error("No new version found.");
        }
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