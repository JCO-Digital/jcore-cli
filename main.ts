#!/usr/bin/env node
import parser from "./parser";

const cmd = parser(process.argv);

console.log(cmd);