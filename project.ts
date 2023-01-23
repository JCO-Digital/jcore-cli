import {extractArchive, loadChecksums, getFile, saveChecksums, calculateChecksum} from "@/utils";
import {archiveLocation, updateFolder} from "@/constants";
import {join} from "path";
import {access, readFile, rename, writeFile} from "fs/promises";
import {JcoreSettings, updateOptions} from "@/types";
import {renameSync, rmSync} from "fs";
import {error, log} from "console";

const defaultOptions = {drone: false, package: false, gulp: false, composer: false, docker: false} as updateOptions;

export function updateFiles(settings: JcoreSettings, options: updateOptions = defaultOptions) {
    const updatePath = join(settings.path, updateFolder);

    if (!settings.name || settings.path === '/') {
        return Promise.reject('Not a project.');
    }


    return getFile(archiveLocation)
        .then(buffer => extractArchive(buffer, updatePath))
        .then(async () => {
            log('Unzipped');

            const checksums = await loadChecksums(settings);

            const files = [
                {
                    name: 'config.sh',
                    force: false,
                    replace: [
                        {search: /#?NAME="[^"]*"/, replace: 'NAME="' + settings.name + '"'},
                    ]
                },
                {
                    name: '.drone.yml',
                    force: options.drone ?? false,
                    source: 'project.drone.yml',
                    replace: [
                        {search: 'wp-content/themes/projectname', replace: 'wp-content/themes/' + settings.theme},
                    ]
                },
                {
                    name: 'package.json',
                    force: options.package ?? false,
                    replace: [
                        {search: /"name":.*$/, replace: '"name": "' + settings.name + '"'},
                        {search: /"theme":.*$/, replace: '"theme": "' + settings.theme + '"'},
                    ]
                },
                {
                    name: 'gulpfile.babel.js',
                    force: options.gulp ?? false,
                    replace: []
                },
                {
                    name: 'composer.json',
                    force: options.composer ?? false,
                    replace: []
                },
                {
                    name: 'docker-compose.yml',
                    force: options.docker ?? false,
                    replace: [
                        {search: '- docker.localhost', replace: '- ' + settings.name + '.localhost'},
                    ]
                },
            ];

            for (let file of files) {
                const source = join(updatePath, file.source ?? file.name);
                const destination = join(settings.path, file.name);
                const matching = (await calculateChecksum(destination) === checksums.get(file.name));
                if (matching) {
                    log('Matching Checksum: ' + file.name);
                }
                await shouldWrite(destination, matching || file.force)
                    .then(destination => moveFile(destination, source))
                    .then(destination => replaceInFile(destination, file.replace))
                    .then(async () => {
                        const checksum = await calculateChecksum(destination);
                        checksums.set(file.name, checksum);
                        log('Updated ' + file.name);
                    })
                    .catch(reason => {
                        rmSync(source);
                        error('Skipping ' + file.name);
                    });
            }
            await saveChecksums(settings, checksums);

            // Clean up legacy folders.
            rmSync(join(settings.path, 'config'), {recursive: true, force: true});
            rmSync(join(settings.path, 'provisioning'), {recursive: true, force: true});

            // Remove old config folder.
            rmSync(join(settings.path, '.config'), {recursive: true, force: true});

            // Move config folder into place.
            renameSync(join(updatePath, '.config'), join(settings.path, '.config'));

            // Clean up remaining files.
            rmSync(updatePath, {recursive: true, force: true});
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

