import {cmdData} from "@/types";
import {echo} from "@/utils";
import {settings} from "@/settings";
import {existsSync, mkdirSync} from "fs";

export function createProject (data: cmdData) {
    if (!existsSync(settings.path)) {
        mkdirSync(settings.path);
    }
    echo("Create Project: " + settings.name);
    console.log(settings);
}