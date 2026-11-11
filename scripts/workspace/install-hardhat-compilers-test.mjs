import assert from "node:assert/strict";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { createServer } from "node:http";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import path from "node:path";
import { test } from "node:test";
import { pathToFileURL } from "node:url";
import { installCompiler } from "./install-hardhat-compilers.mjs";

const version = "0.8.20";
const platform = "wasm";
const compilerName = "soljson-v0.8.20+commit.test.js";
const statedDigest = "0x4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45";
const require = createRequire(pathToFileURL(path.resolve("contracts/package.json")));

async function withLocalList(compilerBytes, run) {
  const compilersDir = await mkdtemp(path.join(tmpdir(), "hardhat-compilers-"));
  let compilerRequests = 0;
  const server = createServer((request, response) => {
    if (request.url === "/wasm/list.json") {
      response.setHeader("content-type", "application/json");
      response.end(JSON.stringify({ builds: [{ version, path: compilerName, keccak256: statedDigest }] }));
    } else if (request.url === `/wasm/${compilerName}`) {
      compilerRequests += 1;
      response.end(compilerBytes);
    } else {
      response.writeHead(404).end();
    }
  });
  try {
    await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
    const baseUrl = `http://127.0.0.1:${server.address().port}`;
    await run({ version, platform, compilersDir, baseUrl, require }, () => compilerRequests);
  } finally {
    await new Promise((resolve) => server.close(resolve));
    await rm(compilersDir, { recursive: true, force: true });
  }
}

async function places_a_compiler_with_the_stated_digest() {
  await withLocalList(Buffer.from("abc"), async (options, requests) => {
    await installCompiler(options);
    const compilerPath = path.join(options.compilersDir, platform, compilerName);
    assert.equal((await readFile(compilerPath)).toString(), "abc");
    assert.equal(requests(), 1);
    await installCompiler(options);
    assert.equal(requests(), 1, "a verified compiler is not fetched again");
  });
}

async function refuses_a_compiler_with_another_digest() {
  await withLocalList(Buffer.from("wrong compiler"), async (options) => {
    await assert.rejects(installCompiler(options), /solc 0\.8\.20.*digest mismatch/);
    await assert.rejects(readFile(path.join(options.compilersDir, platform, compilerName)), {
      code: "ENOENT",
    });
  });
}

test(places_a_compiler_with_the_stated_digest);
test(refuses_a_compiler_with_another_digest);
