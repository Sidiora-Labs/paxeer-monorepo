import { rm } from "node:fs/promises";

await rm(new URL("../.next/dev", import.meta.url), {
  recursive: true,
  force: true,
});
