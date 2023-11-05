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
  nodePath: z.string(),
  execPath: z.string(),
  exec: z.string(),
  inProject: z.boolean(),
  path: z.string(),
  mode: z.string(),
  debug: z.boolean(),
  name: z.string(),
  theme: z.string(),
  branch: z.string(),
  plugins: z.string(),
  install: z.boolean(),
  logLevel: z.number(),
  domains: z.array(z.string()),
  domain: z.string(),
  local: z.string(),
  replace: z.array(z.array(z.string())),
  remoteHost: z.string(),
  remotePath: z.string(),
  dbExclude: z.array(z.string()),
  pluginExclude: z.array(z.string()),
  pluginGit: z.array(z.string()),
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
