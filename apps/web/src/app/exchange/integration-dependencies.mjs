import { lstatSync, mkdirSync, renameSync, rmSync, symlinkSync } from "node:fs";
import { fileURLToPath } from "node:url";

// Keep installed test artifacts in the generated-output directory. The task
// snapshotter walks scoped directories without consulting .gitignore.
const modules = fileURLToPath(new URL("./node_modules", import.meta.url));
if (!lstatSync(modules).isSymbolicLink()) {
  const generated = fileURLToPath(new URL("./dist/node_modules", import.meta.url));
  mkdirSync(new URL("./dist", import.meta.url), { recursive: true });
  rmSync(generated, { recursive: true, force: true });
  renameSync(modules, generated);
  symlinkSync("dist/node_modules", modules, "dir");
}
