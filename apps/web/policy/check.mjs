import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { appContentSecurityPolicy } from "../src/security/csp.ts";

const approvedLocalPackages = {
  "@layerx/ui": {
    resolved: "packages/layerx-ui",
    version: "0.1.0",
    license: "Apache-2.0",
  },
  "@sidiora/layerx-sdk": {
    resolved: "../../services/agent/sdk/typescript",
    version: "0.1.0",
    license: "Apache-2.0",
  },
  "@paxeer/wallet": {
    resolved: "../../sdk/wallet",
    version: "0.1.0",
    license: "Apache-2.0",
    private: true,
  },
};

const allowedLicenses = new Set([
  "0BSD",
  "Apache-2.0",
  "Apache-2.0 AND LGPL-3.0-or-later",
  "Apache-2.0 AND LGPL-3.0-or-later AND MIT",
  "BSD-2-Clause",
  "BSD-3-Clause",
  "BlueOak-1.0.0",
  "CC-BY-4.0",
  "ISC",
  "LGPL-3.0-or-later",
  "MIT",
  "MPL-2.0",
]);

function isInstalledPackagePath(path) {
  const segments = path.split("/");
  const packageName = /^[a-z0-9_~-][a-z0-9._~-]*$/iu;
  for (let index = 0; index < segments.length;) {
    if (segments[index++] !== "node_modules") return false;
    if (segments[index]?.startsWith("@")) {
      if (!packageName.test(segments[index++].slice(1))) return false;
    }
    if (index >= segments.length || !packageName.test(segments[index++])) return false;
  }
  return true;
}

export function checkDependencyPolicy({ manifest, lockfile, budget, localManifests }) {
  assert.equal(lockfile.lockfileVersion, 3, "the web lockfile must use npm lockfile version 3");
  assert.deepEqual(
    manifest.dependencies,
    budget.approved,
    "a direct runtime dependency changed without dependency-budget approval",
  );
  assert.ok(
    Number.isSafeInteger(budget.maximumDirectRuntimeDependencies) &&
      budget.maximumDirectRuntimeDependencies > 0,
    "the direct runtime dependency budget must be a positive finite integer",
  );
  assert.ok(
    Object.keys(manifest.dependencies).length <= budget.maximumDirectRuntimeDependencies,
    "the direct runtime dependency budget was exceeded",
  );
  assert.deepEqual(
    lockfile.packages[""].dependencies,
    manifest.dependencies,
    "the web lockfile does not pin the declared runtime dependencies",
  );
  assert.deepEqual(
    lockfile.packages[""].devDependencies,
    manifest.devDependencies,
    "the web lockfile does not pin the declared development dependencies",
  );

  const localTargets = new Set();
  for (const [name, approval] of Object.entries(approvedLocalPackages)) {
    assert.equal(manifest.dependencies[name], `file:${approval.resolved}`, `${name} manifest path changed`);
    const link = lockfile.packages[`node_modules/${name}`];
    assert.ok(link?.link === true, `${name} must remain an approved local package link`);
    assert.equal(link.resolved, approval.resolved, `${name} must resolve to its checked-in package`);
    const pinned = lockfile.packages[approval.resolved];
    const actual = localManifests[name];
    for (const metadata of [pinned, actual]) {
      assert.ok(metadata && metadata.link !== true, `${name} local package metadata is missing or linked`);
      assert.equal(metadata.name, name, `${name} local package identity changed`);
      assert.equal(metadata.version, approval.version, `${name} local package version changed`);
      assert.equal(metadata.license, approval.license, `${name} local package license disposition changed`);
    }
    if (approval.private === true) {
      assert.equal(actual.private, true, `${name} requires a private package`);
    }
    assert.deepEqual(pinned.dependencies, actual.dependencies, `${name} lockfile dependencies changed`);
    localTargets.add(approval.resolved);
  }

  for (const [path, metadata] of Object.entries(lockfile.packages)) {
    if (path === "") continue;
    let pinned = metadata;
    if (metadata.link === true) {
      const name = path.startsWith("node_modules/") ? path.slice("node_modules/".length) : "";
      assert.ok(Object.hasOwn(approvedLocalPackages, name), `${path} is an unapproved local package link`);
      const approval = approvedLocalPackages[name];
      assert.equal(metadata.resolved, approval.resolved, `${name} must resolve to its checked-in package`);
      pinned = lockfile.packages[metadata.resolved];
    }
    assert.equal(typeof pinned.version, "string", `${path} is not pinned to a version`);
    assert.ok(
      allowedLicenses.has(pinned.license),
      `${path} uses non-allowlisted license ${pinned.license}`,
    );
    assert.ok(
      path.startsWith("node_modules/") ||
        [...localTargets].some((localTarget) =>
          path === localTarget || path.startsWith(`${localTarget}/node_modules/`),
        ),
      `${path} is an unapproved local package target`,
    );
    if (!localTargets.has(path)) {
      const localTarget = [...localTargets].find((prefix) => path.startsWith(`${prefix}/`));
      const installedPath = localTarget ? path.slice(localTarget.length + 1) : path;
      assert.ok(isInstalledPackagePath(installedPath), `${path} is not a canonical installed package path`);
    }
  }
}

export async function runWebPolicy(root = new URL("../", import.meta.url)) {
  const [manifest, lockfile, budget] = await Promise.all(
    ["package.json", "package-lock.json", "policy/runtime-dependencies.json"].map(async (path) =>
      JSON.parse(await readFile(new URL(path, root), "utf8")),
    ),
  );
  const localManifests = Object.fromEntries(await Promise.all(
    Object.entries(approvedLocalPackages).map(async ([name, approval]) => [
      name,
      JSON.parse(await readFile(new URL(`${approval.resolved}/package.json`, root), "utf8")),
    ]),
  ));
  checkDependencyPolicy({ manifest, lockfile, budget, localManifests });
  const productionPolicy = appContentSecurityPolicy("abcdefghijklmnop123456", false);
  assert.match(productionPolicy, /default-src 'self'/u, "the app CSP must default to first-party content");
  assert.match(productionPolicy, /script-src 'self' 'nonce-[^']+' 'strict-dynamic'/u, "the app CSP must nonce scripts");
  assert.match(productionPolicy, /style-src 'self' 'nonce-[^']+'/u, "the app CSP must nonce styles");
  assert.match(productionPolicy, /connect-src 'self'/u, "authenticated connections must remain first-party");
  assert.match(productionPolicy, /font-src 'self'/u, "font origins must be pinned");
  assert.doesNotMatch(productionPolicy, /https?:/u, "the app CSP must not admit third-party origins");
  assert.doesNotMatch(productionPolicy, /unsafe-inline|unsafe-eval/u, "the production app CSP must remain strict");

  const authenticatedRoots = [new URL("src/", root)];
  for (const authenticatedRoot of authenticatedRoots) {
    for (const file of await sourceFiles(authenticatedRoot)) {
      const source = await readFile(file, "utf8");
      assert.doesNotMatch(source, /from\s+["']next\/script["']/u, `${file.pathname} imports next/script`);
      assert.doesNotMatch(source, /<script\b/u, `${file.pathname} contains a raw script element`);
      assert.doesNotMatch(source, /src\s*=\s*["']https?:\/\//u, `${file.pathname} loads a third-party script`);
    }
  }

  return "web dependency, lockfile, license, runtime-budget, and authenticated-plane policies passed";
}

async function sourceFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const nested = await Promise.all(
    entries.map(async (entry) => {
      const location = new URL(entry.name, directory);
      if (entry.isDirectory()) {
        return sourceFiles(new URL(`${entry.name}/`, directory));
      }
      return /\.[cm]?[jt]sx?$/u.test(entry.name) ? [location] : [];
    }),
  );
  return nested.flat();
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  console.log(await runWebPolicy());
}
