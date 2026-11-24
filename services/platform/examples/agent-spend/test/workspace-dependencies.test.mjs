import assert from "node:assert/strict";
import { globSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../../../../", import.meta.url));
const example = "services/platform/examples/agent-spend";

function readJson(path) {
  return JSON.parse(readFileSync(join(root, path), "utf8"));
}

function everyWorkspaceDependencyResolvesFromRootLock() {
  const manifest = readJson("package.json");
  const { packages } = readJson("package-lock.json");
  assert.ok(Array.isArray(manifest.workspaces));
  assert.ok(manifest.workspaces.length > 0);
  for (const pattern of manifest.workspaces) {
    const members = globSync(`${pattern}/package.json`, { cwd: root });
    assert.ok(members.length > 0, `workspace pattern ${pattern} must match a manifest`);
    for (const path of members) {
      const member = dirname(path);
      const declared = readJson(path);
      assert.ok(packages[member], `${member} must have a workspace lock entry`);
      for (const field of ["dependencies", "devDependencies"]) {
        assert.deepEqual(packages[member][field] ?? {}, declared[field] ?? {},
          `${member} ${field} must match its lock entry`);
        for (const name of Object.keys(declared[field] ?? {})) {
          let directory = member;
          let entry;
          while (true) {
            entry = packages[join(directory, "node_modules", name)];
            if (entry || directory === ".") break;
            directory = dirname(directory);
          }
          assert.ok(entry, `${member} ${field} ${name} must resolve in the root lock`);
          if (entry.link) {
            assert.ok(packages[entry.resolved], `${name} must link to a locked workspace`);
          } else {
            assert.equal(typeof entry.version, "string", `${name} must have a locked version`);
          }
        }
      }
    }
  }
}

function agentSpendDevDependenciesAreImported() {
  const manifest = readJson(`${example}/package.json`);
  const imported = new Set();
  const files = globSync("**/*.{mjs,cjs,js,mts,cts,ts,jsx,tsx}", {
    cwd: join(root, example),
    exclude: ["node_modules/**", "test/**"],
  });
  assert.ok(files.length > 0, "the example must have source files to inspect");
  for (const file of files) {
    const source = readFileSync(join(root, example, file), "utf8");
    for (const match of source.matchAll(/(?:\bfrom\s*|\bimport\s*(?:\(\s*)?|\brequire\s*\(\s*)["']([^"']+)["']/gu)) {
      const specifier = match[1];
      if (specifier.startsWith(".") || specifier.startsWith("node:") || specifier.startsWith("/")) continue;
      imported.add(specifier.split("/").slice(0, specifier.startsWith("@") ? 2 : 1).join("/"));
    }
  }
  for (const name of Object.keys(manifest.devDependencies ?? {})) {
    assert.ok(imported.has(name), `${name} is a devDependency without an example source import`);
  }
  for (const name of ["viem", "@sidiora/layerx-agent-integrations"]) {
    assert.ok(imported.has(name), `${name} must remain imported by the example`);
    assert.ok(manifest.dependencies[name], `${name} must remain a runtime dependency`);
  }
}

test(everyWorkspaceDependencyResolvesFromRootLock);
test(agentSpendDevDependenciesAreImported);
