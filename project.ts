import {extractArchive, getFile} from "@/utils";
import {archiveLocation, updateFolder} from "@/constants";
import {join, parse} from "path";
import {access, readFile, rename, rm, writeFile} from "fs/promises";
import {JcoreSettings} from "@/types";

export function updateFiles(settings: JcoreSettings) {
    const updatePath = join(settings.path, updateFolder);
    console.log(updatePath);
    return getFile(archiveLocation)
        .then(buffer => extractArchive(buffer, updatePath))
        .then(async () => {
            console.log('Unzipped');

            // Config
            await shouldWrite(join(settings.path, 'config.sh'))
                .then(destination => moveFile(destination, updatePath))
                .then(file => replaceInFile(file, /#NAME="[^"]*"/, 'NAME="project"'))
                .catch(() => {
                    console.error('Skipping config.sh');
                });

            console.log(1);

            await shouldWrite(join(settings.path, '.drone.yml'))
                .then(destination => moveFile(destination, updatePath, 'project.drone.yml'))
                .then(file => replaceInFile(file, 'wp-content/themes/projectname', 'wp-content/themes/' + settings.theme))
                .then(() => {
                    console.log('Update .drone.yml')
                })
                .catch(reason => {
                    console.error('Skipping .drone.yml ' + reason);
                });

            console.log(2);


            /*
            // Drone
            await overWriteIf(settings, 'project.drone.yml', false, '.drone.yml').then(async file => {
                await replaceInFile(file, 'wp-content/themes/projectname', 'wp-content/themes/' + settings.theme);
                console.log('Drone updated');
            });
            console.log(2);

            // Package.json
            await overWriteIf(settings, 'package.json').then(async file => {
                await replaceInFile(file, /"name":.*$/, '"name": "' + settings.name + '"');
                await replaceInFile(file, /"theme":.*$/, '"theme": "' + settings.theme + '"');
                console.log('Package.json updated');
            });
            console.log(3);

            // Gulpfile
            await overWriteIf(settings, 'gulpfile.babel.js').then(() => {
                console.log('Gulpfile updated');
            });
            console.log(4);

            // Composer.json
            await overWriteIf(settings, 'composer.json').then(() => {
                console.log('Composer updated');
            });
            console.log(5);

            // Docker Compose
            await overWriteIf(settings, 'docker-compose.yml').then(file => {
                console.log("Docker Compose updated");
            });
            console.log(6);

            rm(join(settings.path, '.config'), {force: true}).then(() => {
                console.log('Removing config folder.')
                rename(join(updatePath, '.config'), join(settings.path, '.config')).then(() => {
                    console.log('Moving config folder.')
                })
            });

             */
        }).catch(reason => Promise.reject('Unable to extract file ' + reason));

}

function shouldWrite(file: string, condition: boolean = false): Promise<string> {
    return new Promise(async (resolve, reject) => {
        if (condition) {
            resolve(file)
        }
        try {
            await access(file);
            reject('File exists: ' + file);
        } catch {
            resolve(file);
        }
    });
}

function moveFile(destination: string, updatePath: string, sourceFileName: string | null = null): Promise<string> {
    const sourceName = sourceFileName ?? parse(destination).base;
    const sourceFile = join(updatePath, sourceName);
    try {
        return rename(sourceFile, destination).then(() => Promise.resolve(destination));
    } catch {
        return Promise.reject('Unable to move file ' + sourceFile + ' to ' + destination);
    }
}

function replaceInFile(file: string, search: RegExp | string, replace: string): Promise<void> {
    return readFile(file, 'utf8')
        .then(data => writeFile(file, data.replace(search, replace), 'utf8'));
}
