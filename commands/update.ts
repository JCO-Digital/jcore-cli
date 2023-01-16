import type {cmdData, JcoreSettings} from "@/types";
import {getFile} from "@/utils";
import {scriptLocation} from "@/constants";
import {writeFile} from "fs/promises";
export default function (cmd: cmdData, settings: JcoreSettings) {
    console.log("Update Project");
}

export function selfUpdate (settings: JcoreSettings) {
    console.log("Self Update");
    getFile(scriptLocation).then(body => {
        //writeFile()
        console.log(body);
    })
}