import { z } from "zod";

export const cmdSchema = z.object({
  cmd: z.string(),
  target: z.array(z.string()),
  flags: z.map(z.string(), z.any()),
});
export type cmdData = z.infer<typeof cmdSchema>;

export const updateSchema = z.object({
  force: z.boolean(),
  target: z.array(z.string()),
});
export type updateOptions = z.infer<typeof updateSchema>;

export const settingsSchema = z.object({
  nodePath: z.string().default(""),
  execPath: z.string().default(""),
  exec: z.string().default(""),
  inProject: z.boolean().default(false),
  path: z.string().default(""),
  mode: z.string().default(""),
  debug: z.boolean().default(false),
  name: z.string().default(""),
  theme: z.string().default(""),
  branch: z.string().default(""),
  pluginInstall: z.string().default("remote"),
  install: z.boolean().default(true),
  logLevel: z.number().default(2),
  domains: z.array(z.string()).default([]),
  domain: z.string().default(""),
  local: z.string().default(""),
  replace: z.array(z.array(z.string())).default([]),
  remoteHost: z.string().default(""),
  remotePath: z.string().default(""),
  dbExclude: z.array(z.string()).default([]),
  pluginExclude: z.array(z.string()).default([]),
  pluginGit: z.array(z.string()).default([]),
  wpImage: z.string().default(""),
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
