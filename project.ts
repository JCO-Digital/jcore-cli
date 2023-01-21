import {extractArchive, getFile} from "@/utils";
import {archiveLocation, updateFolder} from "@/constants";
import {join} from "path";
import {access, readFile, rename, writeFile} from "fs/promises";
import {JcoreSettings} from "@/types";
import {renameSync, rmSync} from "fs";

export function updateFiles(settings: JcoreSettings) {
    const updatePath = join(settings.path, updateFolder);

    if (!settings.name || settings.path === '/') {
        return Promise.reject('Not a project.');
    }

    const files = [
        {
            name: 'config.sh',
            replace: [
                {search: /#?NAME="[^"]*"/, replace: 'NAME="' + settings.name + '"'},
            ]
        },
        {
            name: '.drone.yml',
            source: 'project.drone.yml',
            replace: [
                {search: 'wp-content/themes/projectname', replace: 'wp-content/themes/' + settings.theme},
            ]
        },
        {
            name: 'package.json',
            replace: [
                {search: /"name":.*$/, replace: '"name": "' + settings.name + '"'},
                {search: /"theme":.*$/, replace: '"theme": "' + settings.theme + '"'},
            ]
        },
        {
            name: 'gulpfile.babel.js',
            replace: []
        },
        {
            name: 'composer.json',
            replace: []
        },
        {
            name: 'docker-compose.yml',
            replace: [
                {search: '- docker.localhost', replace: '- ' + settings.name + '.localhost'},
            ]
        },
    ];

    return getFile(archiveLocation)
        .then(buffer => extractArchive(buffer, updatePath))
        .then(async () => {
            console.log('Unzipped');

            for (let file of files) {
                const source = join(updatePath, file.source ?? file.name);
                const destination = join(settings.path, file.name);
                await shouldWrite(destination)
                    .then(destination => moveFile(destination, source))
                    .then(destination => replaceInFile(destination, file.replace))
                    .then(() => {
                        console.log('Updated ' + file.name);
                    })
                    .catch(reason => {
                        rmSync(source);
                        console.error('Skipping ' + file.name);
                    });


            }

            // Clean up legacy folders.
            rmSync(join(settings.path, 'config'), {recursive: true, force: true});
            rmSync(join(settings.path, 'provisioning'), {recursive: true, force: true});

            // Remove old config folder.
            rmSync(join(settings.path, '.config'), {recursive: true, force: true});

            // Move config folder into place.
            renameSync(join(updatePath, '.config'), join(settings.path, '.config'));

            // Clean up remaining files.
            //rmSync(updatePath, {recursive: true,force: true});
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

function moveFile(destination: string, source: string): Promise<string> {
    try {
        return rename(source, destination).then(() => Promise.resolve(destination));
    } catch {
        return Promise.reject('Unable to move file ' + source + ' to ' + destination);
    }
}

function replaceInFile(file: string, replace: Array<any>): Promise<void> {
    return readFile(file, 'utf8')
        .then(data => {
            for (let row of replace) {
                data = data.replace(row.search, row.replace);
            }
            return writeFile(file, data, 'utf8')
        });
}
