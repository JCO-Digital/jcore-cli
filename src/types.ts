export type cmdData = {
  cmd: string;
  target: Array<string>;
  flags: Array<string>;
};

export type updateOptions = {
  drone: boolean;
  package: boolean;
  build: boolean;
  composer: boolean;
  docker: boolean;
};
