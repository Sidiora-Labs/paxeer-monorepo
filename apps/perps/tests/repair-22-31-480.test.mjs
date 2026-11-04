import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const repositoryRoot = fileURLToPath(new URL("../../../", import.meta.url));

function repair_22_31_480_installed_binaries_are_untracked() {
  const result = spawnSync("git", ["ls-files", "apps/perps/node_modules"], {
    cwd: repositoryRoot,
    encoding: "utf8",
  });
  assert.ifError(result.error);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout, "", "Installed dependencies must leave the index");
}

function repair_22_31_480_installed_webpack_is_ignored() {
  const result = spawnSync(
    "git",
    ["check-ignore", "-q", "apps/perps/node_modules/.bin/webpack"],
    { cwd: repositoryRoot, encoding: "utf8" },
  );
  assert.ifError(result.error);
  assert.equal(result.status, 0, result.stderr || "webpack must be ignored");
}

function repair_22_31_480_generated_dependency_files_are_untracked_and_ignored() {
  const paths = ["apps/perps/node_modules", "apps/perps/tsconfig.tsbuildinfo"];
  const tracked = spawnSync("git", ["ls-files", "--", ...paths], {
    cwd: repositoryRoot,
    encoding: "utf8",
  });
  assert.ifError(tracked.error);
  assert.equal(tracked.status, 0, tracked.stderr);
  assert.equal(
    tracked.stdout,
    "",
    "Installed dependencies and TypeScript build metadata must leave the index",
  );

  for (const path of [
    "apps/perps/node_modules/.bin/webpack",
    "apps/perps/tsconfig.tsbuildinfo",
  ]) {
    const ignored = spawnSync("git", ["check-ignore", "-v", "--", path], {
      cwd: repositoryRoot,
      encoding: "utf8",
    });
    assert.ifError(ignored.error);
    assert.equal(ignored.status, 0, ignored.stderr || `${path} must be ignored`);
    assert.match(
      ignored.stdout,
      /^apps\/perps\/\.gitignore:\d+:[^\t]+\t/,
      `${path} must be ignored by apps/perps/.gitignore`,
    );
    assert.equal(ignored.stdout.split("\t")[1].trim(), path);
  }
}

test(repair_22_31_480_installed_binaries_are_untracked);
test(repair_22_31_480_installed_webpack_is_ignored);
test(repair_22_31_480_generated_dependency_files_are_untracked_and_ignored);
