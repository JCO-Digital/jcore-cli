export type JcoreSettings = {
    inProject: boolean,
    path: string,
    mode: string,
    debug: number,
    name: string,
    theme: string,
    branch: string,
    plugins: string,
}

export type cmdData = {
    nodePath: string,
    execPath: string,
    exec: string,
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