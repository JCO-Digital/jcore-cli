export type JcoreSettings = {
    nodePath: string,
    execPath: string,
    exec: string,
    inProject: boolean,
    path: string,
    mode: string,
    debug: number,
    name: string,
    theme: string,
    branch: string,
    plugins: string,
    logLevel: number,
}

export type cmdData = {
    cmd: string,
    target: Array<string>,
    flags: Array<string>
}

export type updateOptions = {
    drone: boolean,
    package: boolean,
    build: boolean,
    composer: boolean,
    docker: boolean,
}