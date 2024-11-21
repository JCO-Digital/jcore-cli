import { configScope } from "@/constants";
import { z } from "zod";

export type configValue = string | number | boolean | Array<string>;
export type jsonValue =
  | Array<string | number | boolean>
  | object
  | string
  | number
  | boolean
  | null;

export const flagsSchema = z.object({});
export type flagData = z.infer<typeof flagsSchema>;

export const cmdSchema = z.object({
  cmd: z.string().default(""),
  target: z.array(z.string()).default([]),
  logLevel: z.number().default(2),
  scope: z.number().default(configScope.PROJECT),
  flags: z
    .map(z.string(), z.union([z.boolean(), z.string(), z.number()]))
    .default(new Map()),
});
export type jcoreCmd = z.infer<typeof cmdSchema>;

export const updateSchema = z.object({
  force: z.boolean(),
  target: z.array(z.string()),
});
export type updateOptions = z.infer<typeof updateSchema>;

export const runtimeSchema = z.object({
  exec: z.string().default(""),
  execPath: z.string().default(""),
  inProject: z.boolean().default(false),
  nodePath: z.string().default(""),
  workDir: z.string().default(""),
  branch: z.string().default(""),
});

export type jcoreRuntime = z.infer<typeof runtimeSchema>;

export const settingsSchema = z.object({
  branch: z.string().default(""),
  template: z.string().default("jcore2"),
  dbExclude: z.array(z.string()).default([]),
  dbPrefix: z.string().default("wp_"),
  debug: z.boolean().default(false),
  domains: z.array(z.string()).default([]),
  install: z.boolean().default(true),
  localDomain: z.string().default(""),
  logLevel: z.number().default(2),
  mode: z.string().default(""),
  pluginExclude: z.array(z.string()).default([]),
  pluginGit: z.array(z.string()).default([]),
  pluginInstall: z.string().default("remote"),
  pluginLocal: z.array(z.string()).default([]),
  projectName: z.string().default(""),
  remoteDomain: z.string().default(""),
  remoteHost: z.string().default(""),
  remotePath: z.string().default(""),
  replace: z.array(z.string()).default([]),
  theme: z.string().default(""),
  wpDebug: z.boolean().default(true),
  wpDebugDisplay: z.boolean().default(true),
  wpDebugLog: z.boolean().default(false),
  wpImage: z.string().default(""),
  wpVersion: z.string().default(""),
});
export type jcoreSettings = z.infer<typeof settingsSchema>;

export const projectSchema = z.object({
  name: z.string(),
  path: z.string(),
  running: z.boolean(),
});
export type jcoreProject = z.infer<typeof projectSchema>;

export const dataSchema = z.object({
  version: z.string().default(""),
  latest: z.string().default(""),
  lastCheck: z.number().default(0),
});
export type jcoreData = z.infer<typeof dataSchema>;

export const submoduleSchema = z.object({
  path: z.string(),
  repo: z.string(),
  useBranch: z.boolean().default(false),
});
export type jcoreSubmodule = z.infer<typeof submoduleSchema>;
export const templateSchema = z.object({
  branch: z.string(),
  branches: z.array(z.string()),
  child: z.boolean().default(false),
  submodules: z.array(submoduleSchema).default([]),
});
export type jcoreTemplate = z.infer<typeof templateSchema>;
