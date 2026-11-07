import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";

const upstreamCommit = "45b5ee275a8754a37fce4a6ce174f31e4a216ef4";
const upstreamRepository = "https://github.com/Nehanth/pooled";
const packageRoot = new URL("../", import.meta.url);
const snapshotRoot = new URL("engine/", packageRoot);

async function readJson(relativePath) {
  return JSON.parse(await readFile(new URL(relativePath, packageRoot), "utf8"));
}

async function snapshotFiles(directory = snapshotRoot, prefix = "") {
  const files = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = prefix + entry.name;
    if (entry.isDirectory()) {
      files.push(
        ...(await snapshotFiles(new URL(`${entry.name}/`, directory), `${path}/`)),
      );
    } else {
      assert.ok(entry.isFile(), `snapshot entry must be a regular file: ${path}`);
      files.push(path);
    }
  }
  return files.sort();
}

async function vendoredEngineModulesImport() {
  const publicApi = await import("@paxeer/inference-peer");
  const modules = [
    ["dense", "DenseEngine"],
    ["qwen35", "Qwen35Engine"],
    ["gguf", "ggufWeights"],
    ["gguf", "qwen35Weights"],
    ["tokenizer", "makeTokenizer"],
    ["sampling", "pickSampler"],
    ["generate", "createGenerator"],
    ["quant", "quantizeQ4"],
    ["safetensors", "weightsFromSafetensors"],
  ];
  for (const [path, symbol] of modules) {
    const module = await import(`@paxeer/inference-peer/${path}`);
    assert.equal(typeof module[symbol], "function", `${path} exports ${symbol}`);
  }
  const dense = await import("@paxeer/inference-peer/dense");
  const tokenizer = await import("@paxeer/inference-peer/tokenizer");
  assert.equal(publicApi.DenseEngine, dense.DenseEngine);
  assert.equal(publicApi.makeTokenizer, tokenizer.makeTokenizer);
  assert.equal(typeof publicApi.WGSL, "string");
  assert.match(publicApi.WGSL, /@compute/);

  const shaderDirectory = new URL("engine/wgsl/", snapshotRoot);
  const shaders = await readdir(shaderDirectory);
  assert.ok(shaders.includes("base.js"));
  assert.ok(shaders.includes("qwen35.js"));
  assert.ok(shaders.includes("dense.js"));
  for (const shader of shaders) {
    assert.ok(shader.endsWith(".js"), `unexpected shader asset: ${shader}`);
    const module = await import(new URL(shader, shaderDirectory).href);
    assert.ok(Object.keys(module).length > 0, `${shader} exports its shader API`);
  }
}

async function vendoredTokenizerRoundTripsFixture() {
  const { makeTokenizer } = await import("@paxeer/inference-peer/tokenizer");
  const fixture = await readJson("engine/tests/unit/tokenizer_llamacpp.json");
  assert.match(fixture.note, /ids are llama\.cpp's/);
  assert.ok(fixture.corpus.includes("Hello world"));
  assert.ok(fixture.corpus.some((text) => text.includes("😀")));
  assert.ok(fixture.corpus.some((text) => text.includes("\n")));
  assert.deepEqual(Object.keys(fixture.vocabs).sort(), [
    "gpt-2",
    "qwen2",
    "qwen35",
    "starcoder",
  ]);

  for (const [name, reference] of Object.entries(fixture.vocabs)) {
    const tokenizer = makeTokenizer({
      pre: reference.pre,
      model: { vocab: reference.vocab, merges: reference.merges },
    });
    assert.equal(reference.ids.length, fixture.corpus.length);
    for (const [index, text] of fixture.corpus.entries()) {
      const ids = tokenizer.encode(text);
      assert.ok(ids.length > 0, `${name} produces tokens for fixture ${index}`);
      assert.ok(ids.every(Number.isInteger), `${name} produces real token IDs`);
      assert.deepEqual(ids, reference.ids[index], `${name} reference ${index}`);
      assert.equal(tokenizer.decode(ids), text, `${name} round trip ${index}`);
    }
  }
}

async function vendoredNoticePinsUpstreamCommit() {
  const notice = await readFile(new URL("NOTICE", packageRoot), "utf8");
  const license = await readFile(new URL("LICENSE-ENGINE", packageRoot), "utf8");
  assert.ok(notice.includes("Upstream project: Pooled\n"));
  assert.ok(notice.includes(`Repository: ${upstreamRepository}\n`));
  assert.ok(notice.includes(`Commit: ${upstreamCommit}\n`));
  assert.ok(notice.includes(`${upstreamRepository}/tree/${upstreamCommit}`));
  assert.match(license, /^MIT License\n/);
  assert.ok(license.includes("Copyright (c) 2026 Nehanth Narendrula"));
  assert.ok(license.includes("Permission is hereby granted, free of charge"));
  assert.ok(license.includes('THE SOFTWARE IS PROVIDED "AS IS"'));
  const licenseHash = createHash("sha256").update(license).digest("hex");
  assert.ok(notice.includes(`Upstream LICENSE SHA-256: ${licenseHash}\n`));
}

async function vendoredSnapshotMatchesUpstream() {
  const notice = await readFile(new URL("NOTICE", packageRoot), "utf8");
  const records = [...notice.matchAll(/^([a-f0-9]{64}) {2}(.+)$/gm)];
  const expectedFiles = records.map((record) => record[2]);
  assert.equal(new Set(expectedFiles).size, expectedFiles.length);
  assert.deepEqual(await snapshotFiles(), expectedFiles.sort());
  for (const path of [
    "engine/dense.js",
    "engine/qwen35.js",
    "engine/gguf.js",
    "engine/safetensors.js",
    "engine/tokenizer.js",
    "engine/sampling.js",
    "engine/generate.js",
    "engine/wgsl/base.js",
    "room/wire.js",
    "room/sampling.js",
    "harness/prefix.js",
    "tests/unit/tokenizer_llamacpp.json",
  ]) {
    assert.ok(expectedFiles.includes(path), `snapshot includes ${path}`);
  }
  for (const [, digest, path] of records) {
    let bytes = await readFile(new URL(path, snapshotRoot));
    if (path.endsWith(".js")) {
      const header = Buffer.from(
        `// Vendored from ${upstreamRepository}/blob/${upstreamCommit}/${path} (MIT; see LICENSE-ENGINE).\n`,
      );
      assert.deepEqual(bytes.subarray(0, header.length), header, `${path} origin`);
      bytes = bytes.subarray(header.length);
    }
    assert.equal(
      createHash("sha256").update(bytes).digest("hex"),
      digest,
      `${path} preserves upstream bytes`,
    );
  }
}

async function standalonePackageMetadata() {
  const manifest = await readJson("package.json");
  const lock = await readJson("package-lock.json");
  assert.equal(manifest.type, "module");
  assert.equal(manifest.scripts.build, undefined);
  assert.equal(manifest.scripts.test, "node --test test/vendored_engine.test.mjs");
  assert.equal(lock.lockfileVersion, 3);
  assert.equal(lock.name, manifest.name);
  assert.equal(lock.version, manifest.version);
  assert.equal(lock.packages[""].name, manifest.name);
  assert.equal(lock.packages[""].license, manifest.license);
  assert.deepEqual(lock.packages[""].engines, manifest.engines);
}

test(vendoredEngineModulesImport);
test(vendoredTokenizerRoundTripsFixture);
test(vendoredNoticePinsUpstreamCommit);
test(vendoredSnapshotMatchesUpstream);
test(standalonePackageMetadata);
