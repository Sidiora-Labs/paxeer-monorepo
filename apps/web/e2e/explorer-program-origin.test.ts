import assert from "node:assert/strict";
import { once } from "node:events";
import { existsSync, readFileSync } from "node:fs";
import { createServer, type IncomingHttpHeaders } from "node:http";
import { registerHooks } from "node:module";
import test, { type TestContext } from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

import type { ProgramRecord } from "../src/explorer/model.ts";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js") ? new URL(candidate.href.slice(0, -3) + suffix) : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    if (specifier === "server-only") return { url: "data:text/javascript,", shortCircuit: true };
    if (specifier === "next/navigation") return nextResolve("next/navigation.js", context);
    if (specifier === "next/link") return nextResolve("next/link.js", context);
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      return {
        format: "module", shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const { ExplorerUnavailableError, programRecord, resolveName } = await import("../src/explorer/client.ts");

const PROGRAM = "a".repeat(64);
const NAMING_PROGRAM = "d".repeat(64);
const MISSING_PROGRAM = "b".repeat(64);
const DIGEST = "c".repeat(64);
const PROGRAM_DOCUMENT = {
  program: PROGRAM,
  upgrade_policy: { kind: "immutable" },
  lifecycle: "active",
  versions: [{ version: "1", code_hash: DIGEST, abi_version: "1", source: { status: "unpublished" } }],
  value_accounts: [],
  observed_sequence: "42",
  observed_at: "100",
  receipt_digest: DIGEST,
  state_root: DIGEST,
};
const EXPECTED_PROGRAM: ProgramRecord = {
  program: PROGRAM,
  upgradePolicy: { kind: "immutable" },
  lifecycle: "active",
  versions: [{ version: "1", codeHash: DIGEST, abiVersion: "1", source: { status: "unpublished" } }],
  valueAccounts: [],
  observedSequence: "42",
  observedAt: "100",
  receiptDigest: DIGEST,
  stateRoot: DIGEST,
};
const RESOLUTION = { name: "alice", did: DIGEST, expiry: "200" };

interface ExplorerRequest {
  readonly method: string | undefined;
  readonly path: string | undefined;
  readonly headers: IncomingHttpHeaders;
}

async function explorerServer(context: TestContext): Promise<{
  origin: string;
  requests: ExplorerRequest[];
}> {
  const requests: ExplorerRequest[] = [];
  const server = createServer((request, response) => {
    requests.push({ method: request.method, path: request.url, headers: request.headers });
    response.setHeader("Content-Type", "application/json");
    if (request.url === `/v1/programs/${PROGRAM}`) {
      response.end(JSON.stringify(PROGRAM_DOCUMENT));
    } else if (request.url === `/v1/programs/${NAMING_PROGRAM}/reads/resolve?name=alice`) {
      response.end(JSON.stringify(RESOLUTION));
    } else {
      response.writeHead(404);
      response.end();
    }
  });
  context.after(async () => {
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) => {
      server.close((error) => {
        if (error !== undefined) reject(error);
        else resolve();
      });
    });
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.ok(address !== null && typeof address !== "string");
  return { origin: `http://127.0.0.1:${address.port}`, requests };
}

function configureExplorer(context: TestContext, origin: string): void {
  const keys = [
    "PAXEER_X_EXPLORER_BASE_URL",
    "LAYERX_EXPLORER_PROGRAM_API_ORIGIN",
    "LAYERX_EXPLORER_PROGRAM_BEARER_TOKEN",
    "LAYERX_EXPLORER_NAMING_PROGRAM",
  ] as const;
  const previous = keys.map((key) => [key, process.env[key]] as const);
  context.after(() => {
    for (const [key, value] of previous) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
  });
  process.env.PAXEER_X_EXPLORER_BASE_URL = origin;
  process.env.LAYERX_EXPLORER_NAMING_PROGRAM = NAMING_PROGRAM.toUpperCase();
  delete process.env.LAYERX_EXPLORER_PROGRAM_API_ORIGIN;
  delete process.env.LAYERX_EXPLORER_PROGRAM_BEARER_TOKEN;
}

function assertPublicRequests(origin: string, requests: readonly ExplorerRequest[], paths: readonly string[]): void {
  assert.deepEqual(requests.map((request) => request.path), paths);
  for (const request of requests) {
    assert.equal(request.method, "GET");
    assert.equal(request.headers.host, new URL(origin).host);
    assert.equal(request.headers.accept, "application/json");
    assert.equal(request.headers.authorization, undefined);
  }
}

async function program_record_uses_base_url_without_bearer(context: TestContext): Promise<void> {
  const explorer = await explorerServer(context);
  configureExplorer(context, explorer.origin);

  assert.deepEqual(await programRecord(PROGRAM.toUpperCase()), EXPECTED_PROGRAM);
  assert.deepEqual(await resolveName(RESOLUTION.name), RESOLUTION);
  assert.equal(await programRecord(MISSING_PROGRAM), undefined);
  assert.equal(await resolveName("missing"), undefined);
  assertPublicRequests(explorer.origin, explorer.requests, [
    `/v1/programs/${PROGRAM}`,
    `/v1/programs/${NAMING_PROGRAM}/reads/resolve?name=alice`,
    `/v1/programs/${MISSING_PROGRAM}`,
    `/v1/programs/${NAMING_PROGRAM}/reads/resolve?name=missing`,
  ]);
}

async function program_record_ignores_retired_program_origin(context: TestContext): Promise<void> {
  const explorer = await explorerServer(context);
  const retired = await explorerServer(context);
  configureExplorer(context, explorer.origin);
  process.env.LAYERX_EXPLORER_PROGRAM_API_ORIGIN = retired.origin;
  process.env.LAYERX_EXPLORER_PROGRAM_BEARER_TOKEN = "retired-program-token".repeat(4);

  assert.deepEqual(await programRecord(PROGRAM), EXPECTED_PROGRAM);
  assert.deepEqual(await resolveName(RESOLUTION.name), RESOLUTION);
  delete process.env.PAXEER_X_EXPLORER_BASE_URL;
  await assert.rejects(() => programRecord(PROGRAM), ExplorerUnavailableError);
  await assert.rejects(() => resolveName(RESOLUTION.name), ExplorerUnavailableError);
  assertPublicRequests(explorer.origin, explorer.requests, [
    `/v1/programs/${PROGRAM}`,
    `/v1/programs/${NAMING_PROGRAM}/reads/resolve?name=alice`,
  ]);
  assert.deepEqual(retired.requests, []);
}

test(program_record_uses_base_url_without_bearer);
test(program_record_ignores_retired_program_origin);
