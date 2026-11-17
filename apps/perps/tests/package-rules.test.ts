import assert from "node:assert/strict";
import { existsSync, globSync, readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { parse } from "yaml";

const workspaceRoot = fileURLToPath(new URL("..", import.meta.url));
const repositoryRoot = path.resolve(workspaceRoot, "../..");

interface Manifest {
  name: string;
  packageManager?: string;
  scripts?: Record<string, string>;
}

interface Workspace {
  packages: string[];
  sharedWorkspaceLockfile: boolean;
}

interface WorkflowStep {
  run?: string;
  if?: string;
  "continue-on-error"?: boolean;
  "working-directory"?: string;
}

interface WorkflowJob {
  if?: string;
  "continue-on-error"?: boolean;
  defaults?: { run?: { "working-directory"?: string } };
  strategy?: { matrix?: { package?: string[] } };
  steps?: WorkflowStep[];
}

function read(relative: string): string {
  return readFileSync(path.resolve(workspaceRoot, relative), "utf8");
}

function memberDirectories(): string[] {
  const workspace = parse(read("pnpm-workspace.yaml")) as Workspace;
  const members = new Set<string>();
  const excluded = new Set<string>();
  for (const pattern of workspace.packages) {
    const negative = pattern.startsWith("!");
    for (const directory of globSync(negative ? pattern.slice(1) : pattern, {
      cwd: workspaceRoot,
    })) {
      if (existsSync(path.resolve(workspaceRoot, directory, "package.json"))) {
        (negative ? excluded : members).add(directory);
      }
    }
  }
  return [...members].filter((member) => !excluded.has(member)).sort();
}

function workspaceUsesOneLockfile(): void {
  const manifest = JSON.parse(read("package.json")) as Manifest;
  assert.match(
    manifest.packageManager ?? "",
    /^pnpm@\d+\.\d+\.\d+(?:\+sha512\.[a-f0-9]+)?$/,
  );
  const workspace = parse(read("pnpm-workspace.yaml")) as Workspace;
  assert.equal(workspace.sharedWorkspaceLockfile, true);
  const directories = [".", ...memberDirectories()];
  const lockNames = new Set([
    "pnpm-lock.yaml",
    "package-lock.json",
    "npm-shrinkwrap.json",
    "yarn.lock",
    "bun.lock",
    "bun.lockb",
  ]);
  const lockfiles = directories.flatMap((directory) =>
    readdirSync(path.resolve(workspaceRoot, directory))
      .filter((name) => lockNames.has(name))
      .map((name) => path.posix.join(directory, name)),
  );
  assert.deepEqual(lockfiles, ["pnpm-lock.yaml"]);
  const lock = parse(read("pnpm-lock.yaml")) as {
    importers: Record<string, unknown>;
  };
  assert.deepEqual(Object.keys(lock.importers).sort(), directories.sort());
}
test(workspaceUsesOneLockfile);

function membersRunRealSuites(): void {
  for (const directory of memberDirectories()) {
    const manifest = JSON.parse(read(`${directory}/package.json`)) as Manifest;
    for (const [name, command] of Object.entries(manifest.scripts ?? {})) {
      if (name !== "test" && !name.startsWith("test:")) continue;
      assert.ok(command.trim(), `${directory}: ${name} must execute a suite`);
      assert.doesNotMatch(
        command,
        /\becho\b.*(?:no tests?|test.*(?:specified|given))|--passWithNoTests|--if-present|\|\|\s*(?:true|:)/i,
        `${directory}: ${name} must execute a suite`,
      );
      assert.match(
        command,
        /\b(?:node|jest|vitest|playwright|pnpm|npm|yarn)\b/,
      );
    }
  }
}
test(membersRunRealSuites);

function memberSuitesHaveRunners(): void {
  const workflow = parse(
    readFileSync(
      path.join(repositoryRoot, ".github/workflows/perps-test.yml"),
      "utf8",
    ),
  ) as { jobs: Record<string, WorkflowJob> };
  const covered = new Set<string>();
  for (const job of Object.values(workflow.jobs)) {
    if (job.if !== undefined || job["continue-on-error"]) continue;
    for (const step of job.steps ?? []) {
      if (step.if !== undefined || step["continue-on-error"]) continue;
      const directory =
        step["working-directory"] ?? job.defaults?.run?.["working-directory"];
      if (
        directory === "apps/perps" &&
        step.run === 'pnpm --dir "${{ matrix.package }}" test'
      ) {
        for (const member of job.strategy?.matrix?.package ?? []) {
          covered.add(path.posix.join("apps/perps", member, "package.json"));
        }
      }
    }
  }
  assert.ok(covered.size > 0, "The workflow must execute member test scripts");
  const excluded = new Set<string>();
  const exclusions = readFileSync(
    path.join(repositoryRoot, "tests/ci/data/js_suite_exclusions.txt"),
    "utf8",
  );
  for (const line of exclusions.split("\n")) {
    if (!line.trim() || line.trimStart().startsWith("#")) continue;
    const [manifest, reason] = line.split("|").map((part) => part.trim());
    assert.ok(reason && reason.split(/\s+/).length >= 8, manifest);
    excluded.add(manifest);
  }
  for (const directory of memberDirectories()) {
    const manifest = JSON.parse(read(`${directory}/package.json`)) as Manifest;
    if (
      !Object.keys(manifest.scripts ?? {}).some(
        (name) => name === "test" || name.startsWith("test:"),
      )
    )
      continue;
    const filename = path.posix.join("apps/perps", directory, "package.json");
    assert.ok(
      covered.has(filename) || excluded.has(filename),
      `${filename} needs a workflow runner or service exclusion`,
    );
  }
  for (const filename of covered) {
    const manifest = JSON.parse(
      readFileSync(path.join(repositoryRoot, filename), "utf8"),
    ) as Manifest;
    assert.ok(
      manifest.scripts?.test,
      `${filename} must declare the test script the workflow runs`,
    );
  }
}
test(memberSuitesHaveRunners);
