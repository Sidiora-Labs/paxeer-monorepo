import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { createServer } from "node:http";
import { registerHooks } from "node:module";
import test, { type TestContext } from "node:test";
import { fileURLToPath } from "node:url";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import ts from "typescript";
import type { NativeExplorerFreshness } from "../src/explorer/model.ts";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "server-only") {
      return nextResolve("next/dist/compiled/server-only/empty.js", context);
    }
    if (["next/navigation", "next/link", "next/server"].includes(specifier)) {
      return nextResolve(`${specifier}.js`, context);
    }
    if (specifier.startsWith("@layerx/ui/")) {
      const path = specifier === "@layerx/ui/cn"
        ? "lib/utils"
        : specifier.slice("@layerx/ui/".length);
      const base = new URL(`../packages/layerx-ui/src/${path}`, import.meta.url);
      for (const suffix of [".ts", ".tsx"]) {
        const source = new URL(base.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const base = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = new URL(base.href.endsWith(".js")
          ? base.href.slice(0, -3) + suffix
          : base.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      return {
        format: "module",
        shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: {
            target: ts.ScriptTarget.ES2022,
            module: ts.ModuleKind.ESNext,
            jsx: ts.JsxEmit.ReactJSX,
          },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});


const client = await import("../src/explorer/client.ts");
const model = await import("../src/explorer/model.ts");
const links = await import("../src/explorer/links.ts");
const components = await import("../src/explorer/components.tsx");
const { loadNativeAccountPage } = await import("../src/explorer/native-page.ts");
const { NativeAccountPage } = await import("../src/explorer/native-components.tsx");
const { default: ProgramPage } = await import("../src/app/explorer/programs/[programId]/page.tsx");
// These assertions remain decoder unit tests, not evidence of an available program producer.
await import("../src/explorer/program-contract.test.ts");

interface Transcript {
  transactionHash: string;
  missingHash: string;
  committed: { rung: string; block_number: number; committed_height: number };
  olderCommitted: { rung: string; block_number: number; committed_height: number };
  submitted: { rung: string; block_number: null; committed_height: null };
  blocks: { items: { height: number }[]; next_page_params: null };
  staleBlocks: { items: { height: number }[]; next_page_params: null };
  emptyBlocks: { items: { height: number }[]; next_page_params: null };
  receipt: { transactionHash: string; blockNumber: string; status: string; logs: unknown[] };
  notFound: { message: string };
}

async function nativePeer(context: TestContext) {
  const transcript: Transcript = JSON.parse(readFileSync(
    new URL("../src/explorer/fixtures/repair-22-33-347-native-http.json", import.meta.url), "utf8",
  ));
  const account = JSON.parse(readFileSync(
    new URL("../src/explorer/fixtures/repair-22-31-383-native-http.json", import.meta.url), "utf8",
  )) as {
    address: string;
    missingAddress: string;
    firstPage: unknown;
    evmReceipt: { id: string; frozen: true };
  };
  const replies = new Map<string, readonly [number, unknown]>([
    [`/api/v2/transactions/${transcript.transactionHash}/status`, [200, transcript.committed]],
    [`/api/v2/transactions/${transcript.missingHash}/status`, [404, transcript.notFound]],
    ["/api/v2/blocks?type=block", [200, transcript.blocks]],
    [`/api/v2/addresses/${account.address}/unified`, [200, account.firstPage]],
    [`/api/v2/addresses/${account.missingAddress}/unified`, [404, transcript.notFound]],
    [`/api/v2/paxeer-x/archive/receipts/${account.evmReceipt.id}`, [200, account.evmReceipt]],
  ]);
  const requests: { method: string; path: string; authorization: string | undefined }[] = [];
  const unexpected: string[] = [];
  let receipt: unknown = transcript.receipt;
  const server = createServer(async (request, response) => {
    const path = request.url ?? "";
    requests.push({ method: request.method ?? "", path, authorization: request.headers.authorization });
    response.setHeader("Content-Type", "application/json");
    response.setHeader("Connection", "close");
    if (request.method === "POST" && path === "/rpc") {
      let body = "";
      for await (const chunk of request) body += String(chunk);
      const rpc = JSON.parse(body) as { id: unknown; method: string; params: unknown[] };
      if (rpc.method !== "eth_getTransactionReceipt" || rpc.params.length !== 1
        || (rpc.params[0] !== transcript.transactionHash && rpc.params[0] !== transcript.missingHash)) {
        unexpected.push(body);
        response.writeHead(500).end();
        return;
      }
      response.writeHead(200).end(JSON.stringify({ jsonrpc: "2.0", id: rpc.id,
        result: rpc.params[0] === transcript.missingHash ? null : receipt }));
      return;
    }
    const reply = replies.get(path);
    if (request.method !== "GET" || reply === undefined) {
      unexpected.push(`${request.method} ${path}`);
      response.writeHead(500).end();
      return;
    }
    response.writeHead(reply[0]).end(JSON.stringify(reply[1]));
  });
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  const bound = server.address();
  assert.ok(bound !== null && typeof bound !== "string");
  const origin = `http://127.0.0.1:${bound.port}`;
  const previousExplorer = process.env.PAXEER_X_EXPLORER_BASE_URL;
  const previousGateway = process.env.LAYERX_GATEWAY_URL;
  process.env.PAXEER_X_EXPLORER_BASE_URL = origin;
  process.env.LAYERX_GATEWAY_URL = `${origin}/rpc`;
  context.after(async () => {
    if (previousExplorer === undefined) delete process.env.PAXEER_X_EXPLORER_BASE_URL;
    else process.env.PAXEER_X_EXPLORER_BASE_URL = previousExplorer;
    if (previousGateway === undefined) delete process.env.LAYERX_GATEWAY_URL;
    else process.env.LAYERX_GATEWAY_URL = previousGateway;
    await new Promise<void>((resolve, reject) => {
      server.close((error) => error === undefined ? resolve() : reject(error));
      server.closeAllConnections();
    });
    assert.deepEqual(unexpected, [], "all peer exchanges must match the committed native contracts");
    assert.ok(requests.every((request) => request.authorization === undefined));
  });
  return { transcript, account, origin, replies, requests, setReceipt(value: unknown) { receipt = value; } };
}

async function retired_freshness_presentation_native_http_to_display(context: TestContext) {
  const peer = await nativePeer(context);
  const { transcript, replies } = peer;
  const statusPath = `/api/v2/transactions/${transcript.transactionHash}/status`;
  const render = (position: NativeExplorerFreshness): string => renderToStaticMarkup(
    createElement(components.FreshnessDisplay, { position }),
  );
  const current = await client.nativeExplorerFreshness(transcript.transactionHash);
  assert.deepEqual(current, { state: "current", committedHeight: 100, indexedHeight: 100 });
  const currentHtml = render(current);
  assert.match(currentHtml, /indexed the committed head/u);
  assert.match(currentHtml, /Committed block 100; indexed block 100/u);
  assert.doesNotMatch(currentHtml, /batch|checkpoint|mirror/iu);
  // An older transaction does not make an otherwise current index stale.
  replies.set(statusPath, [200, transcript.olderCommitted]);
  assert.deepEqual(await client.nativeExplorerFreshness(transcript.transactionHash), current);
  replies.set("/api/v2/blocks?type=block", [200, transcript.staleBlocks]);
  const stale = await client.nativeExplorerFreshness(transcript.transactionHash);
  assert.deepEqual(stale, { state: "stale", committedHeight: 100, indexedHeight: 99 });
  assert.match(render(stale), /index is behind the committed head/u);
  assert.match(render(stale), /Committed block 100; indexed block 99/u);
  replies.set("/api/v2/blocks?type=block", [200, transcript.emptyBlocks]);
  const empty = await client.nativeExplorerFreshness(transcript.transactionHash);
  assert.deepEqual(empty, { state: "unavailable", committedHeight: 100, indexedHeight: null });
  assert.match(render(empty), /freshness is unavailable/u);
  assert.match(render(empty), /Committed block 100; indexed block unavailable/u);
  replies.set(statusPath, [200, transcript.submitted]);
  assert.equal((await client.nativeExplorerFreshness(transcript.transactionHash)).state, "unavailable");
  replies.set(statusPath, [503, { message: "Service unavailable" }]);
  await assert.rejects(client.nativeExplorerFreshness(transcript.transactionHash), client.ExplorerHttpError);
  assert.match(renderToStaticMarkup(createElement(components.ExplorerUnavailable)), /freshness is unavailable/u);
  assert.equal("MirrorFreshnessDisplay" in components, false);
  assert.ok(peer.requests.some((request) => request.path === statusPath));
  assert.ok(peer.requests.some((request) => request.path === "/api/v2/blocks?type=block"));
}

async function retired_receipt_link_gateway_to_native_destination(context: TestContext) {
  const peer = await nativePeer(context);
  const { transcript, requests, origin } = peer;
  assert.equal(await client.currentReceiptLink(transcript.transactionHash), `${origin}/tx/${transcript.transactionHash}`);
  assert.equal(await client.currentReceiptLink(transcript.missingHash), undefined);
  peer.setReceipt({ ...transcript.receipt, transactionHash: transcript.missingHash });
  await assert.rejects(client.currentReceiptLink(transcript.transactionHash), /another transaction/u);
  peer.setReceipt({ ...transcript.receipt, transactionHash: "bad" });
  await assert.rejects(client.currentReceiptLink(transcript.transactionHash), /malformed receipt/u);
  const count = requests.length;
  await assert.rejects(client.currentReceiptLink("../receipt"), TypeError);
  assert.equal(requests.length, count, "invalid identifiers must fail before transport");
  assert.throws(() => links.explorerLinkPath({ kind: "receipt", receiptId: "invalid", transactionHash: transcript.transactionHash }), TypeError);
  await assert.rejects(links.resolveReceiptLink("ab".repeat(32), async () => ({ receiptId: "cd".repeat(32), transactionHash: transcript.transactionHash })), /identity/u);
  const resolved = await links.resolveReceiptLink("AB".repeat(32), async () => ({ receiptId: "ab".repeat(32), transactionHash: transcript.transactionHash }));
  assert.equal(links.explorerLinkPath(resolved), `/tx/${transcript.transactionHash}`);
  const archivePath = links.historicalReceiptLinkPath({ receiptId: peer.account.evmReceipt.id.slice(2), frozen: true });
  assert.equal(archivePath, `/api/v2/paxeer-x/archive/receipts/${peer.account.evmReceipt.id}`);
  const archived = await fetch(new URL(archivePath, origin));
  assert.equal(archived.status, 200);
  assert.equal(model.decodeNativeArchiveReceipt(await archived.json()).frozen, true);
  for (const configured of ["https://example.com/path", "https://user:pass@example.com", "https://example.com?x=1", "https://example.com#fragment", "http://example.com"]) {
    assert.equal(links.parseExplorerBaseUrl(configured), undefined);
    process.env.PAXEER_X_EXPLORER_BASE_URL = configured;
    await assert.rejects(client.currentReceiptLink(transcript.transactionHash), client.ExplorerUnavailableError);
  }
  assert.equal(links.parseExplorerBaseUrl(origin)?.origin, origin);
  assert.equal(links.parseExplorerBaseUrl("https://explorer.example")?.origin, "https://explorer.example");
  assert.equal(requests.length, count + 1, "unsafe origins must fail before any gateway request");
}

async function explorer_native_finality_and_account_facts(context: TestContext) {
  const { account, transcript } = await nativePeer(context);
  for (const wire of [transcript.committed, transcript.olderCommitted, transcript.submitted]) {
    assert.deepEqual(model.encodeNativeTransactionStatus(model.decodeNativeTransactionStatus(wire)), wire);
  }
  for (const wire of [
    { ...transcript.committed, block_number: 101 },
    { ...transcript.committed, committed_height: null },
    { ...transcript.committed, rung: "checkpoint-finalised" },
    { ...transcript.committed, committed_height: "100" },
    { ...transcript.committed, observed_sealed_batch: "1" },
  ]) assert.throws(() => model.decodeNativeTransactionStatus(wire), TypeError);
  assert.throws(() => model.nativeFreshnessFromPositions(-1, 100), TypeError);
  assert.throws(() => model.decodeNativeIndexedHeight({ items: [{ height: "100" }] }), TypeError);
  const loaded = await loadNativeAccountPage(account.address);
  assert.equal(loaded.state, "ready");
  if (loaded.state !== "ready") throw new Error("Native account did not load");
  const html = renderToStaticMarkup(createElement(NativeAccountPage, { model: loaded }));
  assert.match(html, /Committed transactions/u);
  assert.doesNotMatch(html, /custody|sealed batch|checkpoint|anchor status/iu);
  for (const activity of loaded.account.activity) {
    assert.equal(activity.status, "committed");
    assert.ok(html.includes(activity.hash));
    assert.ok(html.includes(String(activity.block_number)));
  }
  for (const balance of loaded.account.balances) assert.ok(html.includes(balance.total));
  for (const field of ["settlement", "custody", "layerx_activity", "freshness"]) {
    assert.equal(Object.hasOwn(loaded.account, field), false);
  }
  const missing = await loadNativeAccountPage(account.missingAddress);
  assert.deepEqual(missing, { state: "not-found" });
  assert.match(renderToStaticMarkup(createElement(NativeAccountPage, { model: missing })), /Account not found/u);
  assert.equal((await client.nativeExplorerFreshness(transcript.missingHash)).state, "unavailable");
  // Compatibility codecs remain historical and cannot supply a live freshness presentation.
  const historical = model.historicalExplorer.decodeFreshness({
    observed_chain_sequence: "1", observed_sealed_batch: "1", observed_finalised_checkpoint: "12".repeat(32),
    indexed_batch: "1", batches_behind: "0", current: true,
  });
  const refused = renderToStaticMarkup(createElement(components.FreshnessDisplay, { freshness: historical }));
  assert.match(refused, /freshness is unavailable/u);
  assert.doesNotMatch(refused, /batch|checkpoint|mirror/iu);
}

async function program_fixture_proof_preserves_missing_native_interface(context: TestContext) {
  const peer = await nativePeer(context);
  await assert.rejects(client.programRecord("11".repeat(32)), (error: unknown) => {
    assert.ok(error instanceof client.ExplorerInterfaceUnavailableError);
    assert.equal(error.capability, "program-read");
    return true;
  });
  const rendered = renderToStaticMarkup(await ProgramPage({ params: Promise.resolve({ programId: "11".repeat(32) }) }));
  assert.match(rendered, /freshness is unavailable/u);
  assert.doesNotMatch(rendered, /checkpoint|source verified|Program not found/iu);
  assert.deepEqual(peer.requests, [], "an absent producer must not be replaced by a guessed endpoint or synthetic program");
}

test(retired_freshness_presentation_native_http_to_display);
test(retired_receipt_link_gateway_to_native_destination);
test(explorer_native_finality_and_account_facts);
test(program_fixture_proof_preserves_missing_native_interface);
