import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { createServer } from "node:http";
import { registerHooks } from "node:module";
import test, { type TestContext } from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

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

const {
  ExplorerUnavailableError,
  UnifiedAccountUnavailableError,
  receiptRecord,
  accountActivityPage,
  unifiedAccount,
  unifiedAccountAvailability,
} = await import("../src/explorer/client.ts");

const IDENTIFIER = "ab".repeat(32);
const freshness = {
  observed_chain_sequence: "12",
  observed_sealed_batch: "3",
  observed_finalised_checkpoint: "cd".repeat(32),
  batches_behind: "0",
  current: true,
};
const settlement = {
  network_id: "paxeer-x",
  chain_id: "1325",
  instant_block: "12",
  sealed_batch: "3",
  finalized_batch: "2",
  anchor_status: "2",
  anchor_status_name: "final",
};

function unifiedDocument() {
  return {
    requested: IDENTIFIER,
    canonical: IDENTIFIER,
    evidence: "gateway-reported",
    identities: { layerx_account: IDENTIFIER, bound: false },
    balances: { items: [], joined_limit: "1024" },
    settlement,
    freshness,
    paxeer_activity: { items: [], from_block: "1", to_block: "12" },
    layerx_activity: { items: [] },
  };
}

async function explorerServer(context: TestContext) {
  const requests: { path: string | undefined; profile: string | string[] | undefined; authorization: string | undefined }[] = [];
  const status = { unified: 200 };
  const server = createServer((request, response) => {
    requests.push({
      path: request.url,
      profile: request.headers["layerx-unified-profile"],
      authorization: request.headers.authorization,
    });
    response.setHeader("Content-Type", "application/json");
    const path = new URL(request.url ?? "/", "http://127.0.0.1").pathname;
    if (path === `/v1/explorer/receipts/${IDENTIFIER}`) {
      response.end(JSON.stringify({
        value: {
          receipt_id: IDENTIFIER, batch_number: "3", ordinal: "1",
          canonical_bytes: "AQID", verification_level: "sequencer-signed",
        },
        freshness,
      }));
    } else if (path === `/v1/explorer/accounts/${IDENTIFIER}`) {
      response.end(JSON.stringify({ items: [], next_before: "5", freshness }));
    } else if (path === `/v1/accounts/${IDENTIFIER}/unified`) {
      response.statusCode = status.unified;
      if (status.unified === 503) {
        response.end(JSON.stringify({
          profile: 2, availability: { account: { state: "unavailable", reason: "index_incomplete" } },
        }));
      } else if (request.headers["layerx-unified-profile"] === "2") {
        const reported = (value: string) => ({ state: "present", value, evidence: "gateway-reported" });
        response.end(JSON.stringify({
          ...unifiedDocument(), profile: 2,
          availability: {
            denominations: [],
            settlement: {
              finalized_batch: reported(settlement.finalized_batch),
              anchor_status: reported(settlement.anchor_status),
              anchor_status_name: reported(settlement.anchor_status_name),
            },
          },
        }));
      } else {
        response.end(JSON.stringify(unifiedDocument()));
      }
    } else {
      response.statusCode = 404;
      response.end(JSON.stringify({ error: "unknown route" }));
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  context.after(() => new Promise<void>((resolve, reject) => {
    server.close((error) => error === undefined ? resolve() : reject(error));
  }));
  const address = server.address();
  assert.ok(address !== null && typeof address !== "string");
  return { origin: `http://127.0.0.1:${address.port}`, requests, status };
}

function configureOrigins(context: TestContext, base: string, retired: string): void {
  const values = {
    PAXEER_X_EXPLORER_BASE_URL: base,
    LAYERX_EXPLORER_API_ORIGIN: retired,
    LAYERX_EXPLORER_PROGRAM_API_ORIGIN: retired,
    LAYERX_EXPLORER_PROGRAM_BEARER_TOKEN: "a".repeat(32),
  };
  for (const [key, value] of Object.entries(values)) {
    const previous = process.env[key];
    context.after(() => {
      if (previous === undefined) delete process.env[key];
      else process.env[key] = previous;
    });
    process.env[key] = value;
  }
}

async function explorer_reads_use_base_url_origin(context: TestContext) {
  const base = await explorerServer(context);
  const retired = await explorerServer(context);
  configureOrigins(context, base.origin, retired.origin);

  const receipt = await receiptRecord(IDENTIFIER.toUpperCase());
  assert.ok(receipt.value);
  assert.equal(receipt.value.receiptId, IDENTIFIER);
  assert.equal(receipt.value.canonicalBytes, "AQID");
  assert.equal(receipt.freshness.current, true);
  const activity = await accountActivityPage(IDENTIFIER.toUpperCase(), "9", "10");
  assert.deepEqual(activity.items, []);
  assert.equal(activity.nextBefore, "5");
  assert.equal((await unifiedAccount(IDENTIFIER.toUpperCase(), "12", "9", 10))?.canonical, IDENTIFIER);
  const availability = await unifiedAccountAvailability(IDENTIFIER.toUpperCase(), "12", "9", 10);
  assert.ok(availability);
  assert.equal(availability.profile, 2);
  assert.deepEqual(availability.settlement.finalizedBatch, {
    state: "present", value: "2", evidence: "gateway-reported",
  });
  assert.deepEqual(base.requests.map((request) => request.path), [
    `/v1/explorer/receipts/${IDENTIFIER}`,
    `/v1/explorer/accounts/${IDENTIFIER}?before=9&limit=10`,
    `/v1/accounts/${IDENTIFIER}/unified?before_block=12&before=9&limit=10`,
    `/v1/accounts/${IDENTIFIER}/unified?before_block=12&before=9&limit=10`,
  ]);
  assert.deepEqual(base.requests.map((request) => request.profile), [undefined, undefined, undefined, "2"]);
  assert.ok(base.requests.every((request) => request.authorization === undefined));

  base.status.unified = 404;
  assert.equal(await unifiedAccount(IDENTIFIER), undefined);
  assert.equal(await unifiedAccountAvailability(IDENTIFIER), undefined);
  base.status.unified = 503;
  await assert.rejects(unifiedAccountAvailability(IDENTIFIER), (error: unknown) => {
    assert.ok(error instanceof UnifiedAccountUnavailableError);
    assert.equal(error.reason, "index_incomplete");
    return true;
  });
  assert.equal(base.requests.length, 7);
  assert.deepEqual(retired.requests, []);
}

async function explorer_reads_fail_closed_without_base_url(context: TestContext) {
  const server = await explorerServer(context);
  configureOrigins(context, server.origin, server.origin);
  const readers = [receiptRecord, accountActivityPage, unifiedAccount, unifiedAccountAvailability];
  for (const configured of [
    undefined,
    "",
    "not a URL",
    `${server.origin}/nested`,
    `${server.origin}/?query=1`,
    `${server.origin}/#fragment`,
    server.origin.replace("http://", "http://user:password@"),
    server.origin.replace("http://", "ftp://"),
  ]) {
    if (configured === undefined) delete process.env.PAXEER_X_EXPLORER_BASE_URL;
    else process.env.PAXEER_X_EXPLORER_BASE_URL = configured;
    for (const read of readers) {
      await assert.rejects(read(IDENTIFIER), ExplorerUnavailableError);
      assert.deepEqual(server.requests, [], `${read.name} fetched with base ${String(configured)}`);
    }
  }
}

test(explorer_reads_use_base_url_origin);
test(explorer_reads_fail_closed_without_base_url);
