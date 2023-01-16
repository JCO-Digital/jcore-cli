import type {cmdData, JcoreSettings} from "@/types";
import {getFile} from "@/utils";
import {scriptLocation} from "@/constants";
import {writeFile} from "fs/promises";
export default function (cmd: cmdData, settings: JcoreSettings) {
    console.log("Update Project");
}

export function selfUpdate (cmd: cmdData, settings: JcoreSettings) {
    getFile(scriptLocation).then(body => {
        writeFile(cmd.execPath,body).then(() => {
            console.log("JCORE CLI Updated.");
        }).catch(reason => {
            console.error("Update Error");
            console.error(reason);
        });
    }).catch(reason => {
        console.error("Can't fetch update file.")
    })
}