import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import { cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";

function installedLintUsesBundledManifestForSourceAndWasm(t) {
  const sdk = fileURLToPath(new URL("../", import.meta.url));
  const workspace = mkdtempSync(join(tmpdir(), "layerx-sdk-package-"));
  t.after(() => rmSync(workspace, { recursive: true, force: true }));
  const producer = join(workspace, "producer");
  const consumer = join(workspace, "consumer");
  mkdirSync(producer);
  mkdirSync(consumer);
  const manifest = JSON.parse(readFileSync(join(sdk, "package.json"), "utf8"));
  for (const entry of ["package.json", ...manifest.files]) {
    cpSync(join(sdk, entry), join(producer, entry), { recursive: true });
  }
  const [packed] = JSON.parse(execFileSync("npm", [
    "pack", "--offline", "--ignore-scripts", "--json", "--pack-destination", workspace
  ], { cwd: producer, encoding: "utf8" }));
  assert.ok(packed.files.some((entry) => entry.path === "abi-manifest.tsv"));
  writeFileSync(join(consumer, "package.json"), JSON.stringify({ private: true }));
  execFileSync("npm", [
    "install", "--offline", "--ignore-scripts", "--no-audit", "--no-fund",
    "--package-lock=false", "--omit=dev", join(workspace, packed.filename)
  ], { cwd: consumer, encoding: "utf8" });
  rmSync(producer, { recursive: true });
  const installed = join(consumer, "node_modules", "@sidiora", "layerx-program-sdk");
  const bundledManifest = join(installed, "abi-manifest.tsv");
  assert.deepEqual(readFileSync(bundledManifest), readFileSync(join(sdk, "abi-manifest.tsv")));
  assert.equal(existsSync(join(consumer, "node_modules", "@sidiora", "abi-manifest.tsv")), false);
  const bin = join(consumer, "node_modules", ".bin", "layerx-determinism-lint");
  assert.equal(realpathSync(bin), join(installed, manifest.bin["layerx-determinism-lint"]));

  const source = join(consumer, "guest.ts");
  const wasm = join(consumer, "guest.wasm");
  cpSync(new URL("fixtures/packed-package/guest.ts", import.meta.url), source);
  const compiler = fileURLToPath(new URL("../node_modules/assemblyscript/bin/asc.js", import.meta.url));
  execFileSync(process.execPath, [
    compiler, source, "--config", join(installed, "asconfig.json"), "--outFile", wasm
  ], { cwd: consumer, encoding: "utf8" });
  assert.ok(WebAssembly.validate(readFileSync(wasm)));
  const module = new WebAssembly.Module(readFileSync(wasm));
  assert.deepEqual(WebAssembly.Module.imports(module), [
    { module: "layerx_v1", name: "storage_read", kind: "function" }
  ]);
  assert.ok(WebAssembly.Module.exports(module).some((entry) => entry.name === "memory" && entry.kind === "memory"));
  const passed = spawnSync(bin, [source, wasm], { cwd: consumer, encoding: "utf8" });
  assert.ifError(passed.error);
  assert.equal(passed.status, 0, passed.stderr);
  assert.equal(passed.stderr, "");
  assert.equal(passed.stdout, `determinism-lint: ${source}: passed\ndeterminism-lint: ${wasm}: passed\n`);

  const forbidden = join(consumer, "forbidden.ts");
  writeFileSync(forbidden, readFileSync(source, "utf8").replace('"layerx_v1", "storage_read"', '"layerx_v2", "response_write"'));
  const refused = spawnSync(bin, [forbidden], { cwd: consumer, encoding: "utf8" });
  assert.ifError(refused.error);
  assert.equal(refused.status, 1);
  assert.match(refused.stderr, /forbidden-import-declaration: layerx_v2::response_write/);

  const wrongSource = join(consumer, "wrong.ts");
  const wrongWasm = join(consumer, "wrong.wasm");
  writeFileSync(wrongSource, readFileSync(source, "utf8").replaceAll("capacity: i32", "capacity: i64"));
  execFileSync(process.execPath, [
    compiler, wrongSource, "--config", join(installed, "asconfig.json"), "--outFile", wrongWasm
  ], { cwd: consumer, encoding: "utf8" });
  assert.ok(WebAssembly.validate(readFileSync(wrongWasm)));
  const wrongSignature = spawnSync(bin, [wrongWasm], { cwd: consumer, encoding: "utf8" });
  assert.ifError(wrongSignature.error);
  assert.equal(wrongSignature.status, 1);
  assert.match(wrongSignature.stderr, /wrong-import-signature: layerx_v1::storage_read/);

  rmSync(bundledManifest);
  const missing = spawnSync(bin, [source, wasm], { cwd: consumer, encoding: "utf8" });
  assert.ifError(missing.error);
  assert.equal(missing.status, 2);
  assert.match(missing.stderr, /ABI manifest: .*ENOENT/);
  writeFileSync(bundledManifest, "invalid\n");
  const corrupt = spawnSync(bin, [source, wasm], { cwd: consumer, encoding: "utf8" });
  assert.ifError(corrupt.error);
  assert.equal(corrupt.status, 2);
  assert.match(corrupt.stderr, /ABI manifest: malformed-module: invalid ABI manifest row/);
}

test(installedLintUsesBundledManifestForSourceAndWasm);
