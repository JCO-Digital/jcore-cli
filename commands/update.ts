import type {cmdData, JcoreSettings} from "@/types";
import { spawn } from "child_process";

export default function (cmd: cmdData, settings: JcoreSettings) {
    console.log("Update Project");
}

export function selfUpdate (settings: JcoreSettings) {
    console.log("Self Update");
    const ls = spawn("docker-compose", ["up"]);

    ls.stdout.on("data", data => {
        console.log(`stdout: ${data}`);
    });
    
    ls.stderr.on("data", data => {
        console.log(`stderr: ${data}`);
    });
    
    ls.on('error', (error) => {
        console.log(`error: ${error.message}`);
    });
    
    ls.on("close", code => {
        console.log(`child process exited with code ${code}`);
    });
}