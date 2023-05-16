import { z } from "zod";

export const cmdSchema = z.object({
  cmd: z.string(),
  target: z.array(z.string()),
  flags: z.array(z.string()),
});
export type cmdData = z.infer<typeof cmdSchema>;

export const updateSchema = z.object({
  drone: z.boolean(),
  package: z.boolean(),
  build: z.boolean(),
  composer: z.boolean(),
  docker: z.boolean(),
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
  domain: z.string(),
  local: z.string(),
});
export type jcoreSettings = z.infer<typeof settingsSchema>;

export const projectSchema = z.object({
  name: z.string(),
  path: z.string(),
  running: z.boolean(),
  started: z.number().default(Date.now()),
});
export type projectData = z.infer<typeof projectSchema>;

export const dataSchema = z.object({
  projects: z.array(projectSchema).default([]),
  version: z.string().default(""),
  latest: z.string().default(""),
  lastCheck: z.number().default(0),
});
export type jcoreData = z.infer<typeof dataSchema>;
