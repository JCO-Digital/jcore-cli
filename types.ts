export type JcoreSettings = {
    path: string,
    mode: string,
    debug: number,
    theme: string,
    branch: string,
    plugins: string,
}

export type cmdData = {
    node: string,
    exec: string,
    cmd: string,
    target: Array<string>,
    flags: Array<string>
}
