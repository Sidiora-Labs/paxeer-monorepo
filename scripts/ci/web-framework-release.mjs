import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../", import.meta.url));
const minimum = [16, 4, 6];
const tracked = new Set(
  execFileSync(
    "git",
    [
      "ls-files",
      "-z",
      "--",
      ":(glob)**/package.json",
      ":(glob)**/pnpm-workspace.yaml",
      ":(glob)**/pnpm-lock.yaml",
      ":(glob)**/package-lock.json",
      ":(glob)**/npm-shrinkwrap.json",
    ],
    { cwd: root, encoding: "utf8" },
  )
    .split("\0")
    .filter(Boolean),
);
const files = new Map();
const failures = new Set();

function read(file) {
  if (!files.has(file))
    files.set(file, readFileSync(path.join(root, file), "utf8"));
  return files.get(file);
}

function check(file, release) {
  const match = /^(16)(?:\.(\d+|[xX*]))?(?:\.(\d+|[xX*]))?(-[\w.-]+)?/.exec(
    release,
  );
  if (!match) return;
  const version = [
    16,
    Number.parseInt(match[2], 10) || 0,
    Number.parseInt(match[3], 10) || 0,
  ];
  const comparison = version[1] - minimum[1] || version[2] - minimum[2];
  if (comparison < 0 || (comparison === 0 && match[4])) {
    failures.add(`${file}: next ${release} is below ${minimum.join(".")}`);
  }
}

function declarations(value, releases = []) {
  if (!value || typeof value !== "object") return releases;
  for (const [name, entry] of Object.entries(value)) {
    if (
      (name === "next" || name.startsWith("next@")) &&
      typeof entry === "string"
    ) {
      releases.push(
        ...entry.matchAll(
          /\b16(?:\.(?:\d+|[xX*]))?(?:\.(?:\d+|[xX*]))?(?:-[\w.-]+)?\b/g,
        ),
      );
    } else if (entry && typeof entry === "object") {
      declarations(entry, releases);
    }
  }
  return releases;
}

function checkNpm(file) {
  const lock = JSON.parse(read(file));
  for (const [name, entry] of Object.entries(lock.packages ?? {})) {
    if (/(^|\/)node_modules\/next$/.test(name))
      check(file, entry.version ?? "");
  }
  function dependencies(entries) {
    for (const [name, entry] of Object.entries(entries ?? {})) {
      if (name === "next") check(file, entry.version ?? "");
      dependencies(entry.dependencies);
    }
  }
  dependencies(lock.dependencies);
}

function checkPnpm(file) {
  const text = read(file);
  for (const match of text.matchAll(
    /^ {2}['"]?\/?next@?(16\.\d+\.\d+(?:-[\w.-]+)?)/gm,
  )) {
    check(file, match[1]);
  }
  for (const match of text.matchAll(
    /^ +next:\s*\n +specifier:[^\n]*\n +version: ['"]?(16\.\d+\.\d+(?:-[\w.-]+)?)/gm,
  )) {
    check(file, match[1]);
  }
}

function workspaceIncludes(directory, patterns, targets) {
  const positive = patterns.filter((pattern) => !pattern.startsWith("!"));
  const negative = patterns
    .filter((pattern) => pattern.startsWith("!"))
    .map((pattern) => pattern.slice(1));
  const matches = (target, pattern) =>
    path.matchesGlob(
      target,
      path.posix.normalize(path.posix.join(directory, pattern)),
    );
  return targets.some(
    (target) =>
      positive.some((pattern) => matches(target, pattern)) &&
      !negative.some((pattern) => matches(target, pattern)),
  );
}

try {
  const locks = new Set();
  const targets = [];
  const workspaces = [];
  for (const file of tracked) {
    if (path.basename(file) !== "package.json") continue;
    const manifest = JSON.parse(read(file));
    const patterns = Array.isArray(manifest.workspaces)
      ? manifest.workspaces
      : manifest.workspaces?.packages;
    if (patterns) workspaces.push([path.posix.dirname(file), patterns]);
    const releases = [];
    for (const section of [
      "dependencies",
      "devDependencies",
      "optionalDependencies",
      "peerDependencies",
      "overrides",
      "resolutions",
      "pnpm",
    ])
      declarations(manifest[section], releases);
    if (releases.length === 0) continue;
    for (const [release] of releases) check(file, release);
    targets.push(path.posix.dirname(file));

    let directory = path.dirname(file);
    while (true) {
      for (const name of [
        "package-lock.json",
        "npm-shrinkwrap.json",
        "pnpm-lock.yaml",
      ]) {
        const lock = path.posix.join(directory, name);
        if (tracked.has(lock)) locks.add(lock);
      }
      if (directory === ".") break;
      directory = path.posix.dirname(directory);
    }
  }
  for (const file of tracked) {
    if (path.basename(file) !== "pnpm-workspace.yaml") continue;
    const patterns = [];
    let packages = false;
    for (const line of read(file).split("\n")) {
      if (/^packages:\s*(?:#.*)?$/.test(line)) {
        packages = true;
        continue;
      }
      const item = /^\s*-\s+["']?([^"'#]+?)["']?\s*(?:#.*)?$/.exec(line);
      if (packages && item) patterns.push(item[1].trim());
      else if (/^\S/.test(line) && !line.startsWith("#")) packages = false;
    }
    workspaces.push([path.posix.dirname(file), patterns]);
  }
  for (const [directory, patterns] of workspaces) {
    if (!workspaceIncludes(directory, patterns, targets)) continue;
    for (const name of [
      "package-lock.json",
      "npm-shrinkwrap.json",
      "pnpm-lock.yaml",
    ]) {
      const lock = path.posix.join(directory, name);
      if (tracked.has(lock)) locks.add(lock);
    }
  }
  for (const file of locks) {
    if (file.endsWith(".yaml")) checkPnpm(file);
    else checkNpm(file);
  }
  for (const failure of failures) console.error(failure);
  if (failures.size > 0) process.exitCode = 1;
} catch (error) {
  console.error(error.message);
  process.exitCode = 1;
} finally {
  console.log(
    `Read ${files.size} files for the Next.js ${minimum.join(".")} release check.`,
  );
}
