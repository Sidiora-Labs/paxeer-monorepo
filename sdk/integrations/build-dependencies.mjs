import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../", import.meta.url));
const readManifest = (path) => JSON.parse(readFileSync(new URL(path, new URL("../../", import.meta.url)), "utf8"));
const packages = new Map(readManifest("package.json").workspaces.map((path) => {
  const manifest = readManifest(`${path}/package.json`);
  return [manifest.name, manifest];
}));
const target = process.argv[2];
if (process.argv.length !== 3 || !packages.has(target)) {
  throw new Error(`Expected one workspace name, received ${process.argv.slice(2).join(" ")}`);
}

const visited = new Set();
const visiting = new Set();
const ordered = [];

function visit(name) {
  if (visited.has(name)) return;
  if (visiting.has(name)) throw new Error(`Workspace dependency cycle at ${name}`);
  const manifest = packages.get(name);
  visiting.add(name);
  for (const field of ["dependencies", "devDependencies", "peerDependencies", "optionalDependencies"]) {
    for (const dependency of Object.keys(manifest[field] ?? {})) {
      if (packages.has(dependency)) visit(dependency);
    }
  }
  visiting.delete(name);
  visited.add(name);
  if (name !== target && typeof manifest.scripts?.build === "string") ordered.push(name);
}

visit(target);
for (const name of ordered) {
  // The graph already orders prerequisites; suppress their recursive prebuild hooks.
  const result = spawnSync("npm", ["run", "build", "--workspace", name, "--ignore-scripts"], {
    cwd: root,
    stdio: "inherit",
  });
  if (result.error) throw result.error;
  if (result.signal) throw new Error(`${name} build terminated by ${result.signal}`);
  if (result.status !== 0) process.exit(result.status);
}
