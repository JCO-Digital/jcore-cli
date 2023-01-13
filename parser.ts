export default function parser(args: Array<string>) {
    const data = {
        node: "",
        exec: "",
        cmd: "",
        target: "",
        flags: [] as Array<string>
    }

    for (let i = 0; i < args.length; ++i) {
        const part = args[i];
        if (i === 0) {
            data.node = part;
        } else if (i === 1) {
            data.exec = part.split("/").pop();
        } else if (part.substring(0, 1) === "-") {
            // Flag
            data.flags.push(part.substring(1));
        } else if (data.cmd === "") {
            data.cmd = part;
        } else if (data.target === "") {
            data.target = part;
        }
    }

    return data;
}