import assert from "node:assert/strict";
import { channel } from "node:diagnostics_channel";
import { existsSync, readFileSync } from "node:fs";
import { createServer, type Server } from "node:http";
import { registerHooks } from "node:module";
import test, { type TestContext } from "node:test";
import { fileURLToPath } from "node:url";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import ts from "typescript";
import type { NativeArchiveReceipt, NativeUnifiedAccount } from "../src/explorer/native-model.ts";

// Node runs the server modules and JSX directly, without replacing their transport.
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "server-only") return { url: "data:text/javascript,", shortCircuit: true };
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = new URL(candidate.href + suffix);
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
          compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const client = await import("../src/explorer/client.ts");
const model = await import("../src/explorer/native-model.ts");
const page = await import("../src/explorer/native-page.ts");
const { NativeAccountPage } = await import("../src/explorer/native-components.tsx");

interface NativeHttpTranscript {
  address: string;
  missingAddress: string;
  unavailableAddress: string;
  otherAddress: string;
  cursors: { first: string; next: string; expired: string; reorg: string };
  firstPage: NativeUnifiedAccount;
  secondPage: NativeUnifiedAccount;
  evmReceipt: NativeArchiveReceipt;
  kernelReceipt: NativeArchiveReceipt;
  expiredRefusal: { message: string; reason: string };
  snapshotRefusal: { message: string; reason: string };
  notFound: { message: string };
  unavailable: { message: string };
  invalidParameters: { message: string };
}

async function producer(context: TestContext) {
  const transcript: NativeHttpTranscript = JSON.parse(readFileSync(
    new URL("../src/explorer/fixtures/repair-22-31-383-native-http.json", import.meta.url), "utf8",
  ));
  // Validate the committed peer responses with the same codecs as the HTTP consumer.
  for (const document of [transcript.firstPage, transcript.secondPage]) {
    assert.deepEqual(model.decodeNativeUnifiedAccount(document), document);
  }
  for (const document of [transcript.evmReceipt, transcript.kernelReceipt]) {
    assert.deepEqual(model.decodeNativeArchiveReceipt(document), document);
  }
  const address = model.nativeAddress(transcript.address);
  const accountPath = `/api/v2/addresses/${address.toLowerCase()}/unified`;
  const continuationPath = (cursor: string): string => `${accountPath}?${new URLSearchParams({ cursor })}`;
  const responses = new Map<string, readonly [number, unknown]>([
    [accountPath, [200, transcript.firstPage]],
    [continuationPath(transcript.cursors.first), [200, transcript.firstPage]],
    [continuationPath(transcript.cursors.next), [200, transcript.secondPage]],
    [continuationPath(transcript.cursors.expired), [422, transcript.expiredRefusal]],
    [continuationPath(transcript.cursors.reorg), [422, transcript.snapshotRefusal]],
    [continuationPath("invalid-native-cursor"), [422, transcript.snapshotRefusal]],
    [continuationPath("invalid-parameters"), [422, transcript.invalidParameters]],
    [`/api/v2/addresses/${transcript.otherAddress}/unified?${new URLSearchParams({ cursor: transcript.cursors.next })}`, [422, transcript.snapshotRefusal]],
    [`/api/v2/addresses/${transcript.missingAddress}/unified`, [404, transcript.notFound]],
    [`/api/v2/addresses/${transcript.unavailableAddress}/unified`, [503, transcript.unavailable]],
    [`/api/v2/paxeer-x/archive/receipts/${transcript.evmReceipt.id}`, [200, transcript.evmReceipt]],
    [`/api/v2/paxeer-x/archive/receipts/${transcript.kernelReceipt.id}`, [200, transcript.kernelReceipt]],
    [`/api/v2/paxeer-x/archive/receipts/0x${"0".repeat(64)}`, [404, transcript.notFound]],
  ]);
  const unexpected: string[] = [];
  const createFixtureServer = (): Server => createServer((request, response) => {
    const exchange = responses.get(request.url ?? "");
    response.setHeader("Content-Type", "application/json");
    response.setHeader("Connection", "close");
    if (request.method !== "GET" || request.headers.authorization !== undefined || exchange === undefined) {
      unexpected.push(`${request.method} ${request.url}`);
      response.writeHead(500).end(JSON.stringify({ message: "Unrecorded native explorer request" }));
      return;
    }
    response.writeHead(exchange[0]).end(JSON.stringify(exchange[1]));
  });
  async function listen(server: Server, port: number): Promise<void> {
    await new Promise<void>((resolve, reject) => {
      server.once("error", reject);
      server.listen(port, "127.0.0.1", () => {
        server.off("error", reject);
        resolve();
      });
    });
  }
  async function close(server: Server): Promise<void> {
    if (!server.listening) return;
    await new Promise<void>((resolve, reject) => {
      server.close((error) => error === undefined ? resolve() : reject(error));
      server.closeAllConnections();
    });
  }
  let server = createFixtureServer();
  await listen(server, 0);
  const bound = server.address();
  assert.ok(bound !== null && typeof bound !== "string");
  const port = bound.port;
  const origin = new URL(`http://127.0.0.1:${port}`);
  const previous = process.env.PAXEER_X_EXPLORER_BASE_URL;
  // Supply all configuration locally; an inherited origin is never contacted.
  process.env.PAXEER_X_EXPLORER_BASE_URL = origin.origin;
  context.after(async () => {
    try {
      await close(server);
    } finally {
      if (previous === undefined) delete process.env.PAXEER_X_EXPLORER_BASE_URL;
      else process.env.PAXEER_X_EXPLORER_BASE_URL = previous;
    }
    assert.deepEqual(unexpected, [], "every HTTP request must match the native response transcript");
  });
  return {
    origin,
    address,
    transcript,
    async restart(): Promise<void> {
      await close(server);
      server = createFixtureServer();
      await listen(server, port);
    },
  };
}

async function nativeResponse(address: string, origin: URL, cursor?: string) {
  const url = new URL(`/api/v2/addresses/${address.toLowerCase()}/unified`, origin);
  if (cursor !== undefined) url.searchParams.set("cursor", cursor);
  return fetch(url, { headers: { Accept: "application/json" }, cache: "no-store", signal: AbortSignal.timeout(8_000) });
}

async function explorer_retired_transport_native_http(context: TestContext) {
  const { origin, address, transcript } = await producer(context);
  const receiptId = transcript.evmReceipt.id.slice(2);
  const requests: { origin: string; path: string; headers: string }[] = [];
  const diagnostics = channel("undici:request:create");
  const observe = (message: unknown): void => {
    const { request } = message as { request: { origin: string; path: string; headers: string } };
    requests.push({ origin: String(request.origin), path: request.path, headers: request.headers });
  };
  diagnostics.subscribe(observe);
  try {
    const account = await client.unifiedAccount(address);
    assert.ok(account);
    assert.equal(account.canonical, address.toLowerCase());
    const activity = await client.accountActivityPage(address, account.page_cursor);
    assert.deepEqual(activity.items, account.activity);
    const receipt = await client.archiveReceiptRecord(receiptId);
    assert.ok(receipt);
    assert.equal(receipt.id.toLowerCase(), `0x${receiptId.toLowerCase()}`);
    assert.equal(receipt.frozen, true);
    const raw = await fetch(new URL(`/api/v2/paxeer-x/archive/receipts/0x${receiptId.toLowerCase()}`, origin));
    assert.equal(raw.status, 200);
    assert.deepEqual(receipt, model.decodeNativeArchiveReceipt(await raw.json()));
    const kernelReceipt = await client.archiveReceiptRecord(transcript.kernelReceipt.id.slice(2));
    assert.deepEqual(kernelReceipt, transcript.kernelReceipt);
    assert.equal(await client.archiveReceiptRecord("0".repeat(64)), undefined);
    await assert.rejects(client.receiptRecord(receiptId), (error: unknown) => {
      assert.ok(error instanceof client.ExplorerInterfaceUnavailableError);
      assert.equal(error.capability, "canonical-receipt-bytes");
      return true;
    });
  } finally {
    diagnostics.unsubscribe(observe);
  }
  assert.ok(requests.length >= 6);
  assert.ok(requests.every((request) => request.origin === origin.origin));
  assert.ok(requests.every((request) => !request.path.startsWith("/v1/")));
  assert.ok(requests.every((request) => !/authorization:/iu.test(request.headers)));
  assert.ok(requests.some((request) => request.path.startsWith(`/api/v2/addresses/${address.toLowerCase()}/unified`)));
  assert.ok(requests.some((request) => request.path.startsWith("/api/v2/paxeer-x/archive/receipts/0x")));
}

async function native_unified_shape_http_to_page(context: TestContext) {
  const { origin, address, transcript } = await producer(context);
  const response = await nativeResponse(address, origin);
  assert.equal(response.status, 200);
  const emitted = await response.json();
  const decoded = model.decodeNativeUnifiedAccount(emitted);
  assert.deepEqual(decoded, emitted, "the native producer's fields and encodings survive decoding unchanged");
  assert.deepEqual(decoded, transcript.firstPage);
  const loaded = await page.loadNativeAccountPage(address, decoded.page_cursor);
  assert.equal(loaded.state, "ready");
  if (loaded.state !== "ready") throw new Error("Native account did not reach the page");
  assert.deepEqual(loaded.account.activity, decoded.activity);
  assert.deepEqual(loaded.account.balances, decoded.balances);
  assert.ok(decoded.balances.length > 0);
  assert.ok(decoded.activity.length > 0, "the proof account must contain committed activity");
  const html = renderToStaticMarkup(createElement(NativeAccountPage, { model: loaded }));
  assert.ok(html.includes("Account totals in base units"));
  assert.ok(html.includes("Committed transactions"));
  for (const balance of decoded.balances) assert.ok(html.includes(balance.total));
  for (const activity of decoded.activity) {
    assert.equal(activity.status, "committed");
    assert.ok(html.includes(activity.hash));
    assert.ok(html.includes(activity.amount));
    assert.ok(html.includes(String(activity.block_number)));
  }
  for (const legacy of ["settlement", "paxeer_activity", "layerx_activity", "freshness", "evidence"]) {
    assert.equal(Object.hasOwn(loaded.account, legacy), false);
  }
  const nonCommitted = structuredClone(emitted);
  nonCommitted.activity[0].status = "submitted";
  assert.throws(() => model.decodeNativeUnifiedAccount(nonCommitted), /committed/u);
  const numericAmount = structuredClone(emitted);
  numericAmount.balances[0].total = 1;
  assert.throws(() => model.decodeNativeUnifiedAccount(numericAmount), /string/u);
}

async function native_unified_cursor_reopen_and_refusal(context: TestContext) {
  const fixture = await producer(context);
  const { origin, address, transcript } = fixture;
  let first = await page.loadNativeAccountPage(address);
  assert.equal(first.state, "ready");
  if (first.state !== "ready") throw new Error("Native first page unavailable");
  assert.ok(first.nextHref, "the proof account must span at least two native pages");
  assert.equal(first.account.activity.length, 50);
  const savedContinuation = new URL(first.nextHref, origin).href;
  const firstHashes = new Set(first.account.activity.map((item) => item.hash));
  // Discard the consumer instance. Its only retained state is its navigable URL.
  first = { state: "not-found" };
  const reopenedUrl = new URL(savedContinuation);
  const cursor = reopenedUrl.searchParams.get("cursor");
  assert.ok(cursor);
  assert.equal(cursor, transcript.cursors.next, "URL encoding must preserve the opaque continuation byte for byte");
  await fixture.restart();
  const reopened = await page.loadNativeAccountPage(address, cursor);
  assert.equal(reopened.state, "ready");
  if (reopened.state !== "ready") throw new Error("Native continuation unavailable after reopen");
  assert.equal(reopened.account.page_number, 2);
  assert.ok(reopened.account.activity.length > 0);
  assert.ok(reopened.account.activity.every((item) => !firstHashes.has(item.hash)));
  assert.deepEqual(reopened.account, transcript.secondPage);
  const producerPage = await nativeResponse(address, origin, cursor);
  assert.equal(producerPage.status, 200);
  const producerDocument = model.decodeNativeUnifiedAccount(await producerPage.json());
  assert.deepEqual(reopened.account.activity, producerDocument.activity);
  assert.equal(new URL(reopened.refreshHref, origin).searchParams.get("cursor"), reopened.account.page_cursor);
  assert.equal(new URL(reopened.firstHref, origin).searchParams.get("cursor"), reopened.account.first_page_cursor);
  if (reopened.nextHref !== undefined) {
    assert.equal(new URL(reopened.nextHref, origin).searchParams.get("cursor"), reopened.account.next_page_params?.cursor);
  }
  const replayed = await page.loadNativeAccountPage(address, reopened.account.first_page_cursor);
  assert.equal(replayed.state, "ready");
  if (replayed.state === "ready") assert.deepEqual(replayed.account, transcript.firstPage);
  const wrongAccount = await page.loadNativeAccountPage(transcript.otherAddress, cursor);
  assert.deepEqual(wrongAccount, {
    state: "cursor-refused", reason: transcript.snapshotRefusal.reason,
    resetHref: page.nativeAccountPagePath(transcript.otherAddress),
  });
  const invalid = await nativeResponse(address, origin, "invalid-native-cursor");
  assert.equal(invalid.status, 422);
  const reason = model.decodeNativeCursorRefusal(await invalid.json());
  const refused = await page.loadNativeAccountPage(address, "invalid-native-cursor");
  assert.deepEqual(refused, { state: "cursor-refused", reason, resetHref: page.nativeAccountPagePath(address) });
  const html = renderToStaticMarkup(createElement(NativeAccountPage, { model: refused }));
  assert.ok(html.includes("Start a new account snapshot"));
  assert.ok(!html.includes("Committed transactions"));
  const reset = await page.loadNativeAccountPage(address);
  assert.equal(reset.state, "ready");
  if (reset.state === "ready") assert.equal(reset.account.page_number, 1);
}

async function native_unified_expiry_and_reorg_refusal(context: TestContext) {
  const fixture = await producer(context);
  const { origin, address, transcript } = fixture;
  // Replay the native refusal documents; signing and consensus remain native producer responsibilities.
  for (const [cursor, expected] of [
    [transcript.cursors.expired, "expired"],
    [transcript.cursors.reorg, "malformed, wrong account, changed binding or reorganized snapshot"],
  ] as const) {
    const raw = await nativeResponse(address, origin, cursor);
    assert.equal(raw.status, 422);
    assert.equal(model.decodeNativeCursorRefusal(await raw.json()), expected);
    const refused = await page.loadNativeAccountPage(address, cursor);
    assert.deepEqual(refused, { state: "cursor-refused", reason: expected, resetHref: page.nativeAccountPagePath(address) });
    const html = renderToStaticMarkup(createElement(NativeAccountPage, { model: refused }));
    assert.ok(html.includes(expected));
    assert.ok(html.includes("Start a new account snapshot"));
    assert.ok(!html.includes("Committed transactions"));
    const reset = await page.loadNativeAccountPage(address);
    assert.equal(reset.state, "ready");
    if (reset.state === "ready") assert.equal(reset.account.page_number, 1);
  }
  const beforeRestart = await client.nativeUnifiedAccount(address);
  assert.ok(beforeRestart?.next_page_params);
  const restartCursor = beforeRestart.next_page_params.cursor;
  await fixture.restart();
  const restarted = await page.loadNativeAccountPage(address, restartCursor);
  assert.equal(restarted.state, "ready", "the returned continuation must survive reopening the fixture's HTTP listener");
  if (restarted.state === "ready") assert.deepEqual(restarted.account, transcript.secondPage);
}

async function native_explorer_validation_and_missing_interfaces(context: TestContext) {
  const { address, transcript } = await producer(context);
  assert.deepEqual(await page.loadNativeAccountPage(transcript.missingAddress), { state: "not-found" });
  assert.deepEqual(await page.loadNativeAccountPage(transcript.unavailableAddress), {
    state: "unavailable", status: 503, responseBody: transcript.unavailable,
  });
  assert.deepEqual(await page.loadNativeAccountPage(address, "invalid-parameters"), {
    state: "unavailable", status: 422, responseBody: transcript.invalidParameters,
  });
  await assert.rejects(client.nativeUnifiedAccount("a".repeat(64)), TypeError);
  await assert.rejects(client.nativeUnifiedAccount("0x" + "1".repeat(40), "a".repeat(4097)), TypeError);
  await assert.rejects(client.unifiedAccount("0x" + "1".repeat(40), undefined, "12"), /opaque cursor/u);
  await assert.rejects(client.accountActivityPage("0x" + "1".repeat(40), undefined, "25"), /fixed page size/u);
  assert.throws(() => model.nativeCursor(""), TypeError);
  const opaque = "opaque+cursor/with=padding&account%bound";
  assert.equal(new URL(page.nativeAccountPagePath("0x" + "1".repeat(40), opaque), "https://example.invalid").searchParams.get("cursor"), opaque);
  const previous = process.env.PAXEER_X_EXPLORER_BASE_URL;
  try {
    delete process.env.PAXEER_X_EXPLORER_BASE_URL;
    const unavailable = await page.loadNativeAccountPage("0x" + "1".repeat(40));
    assert.deepEqual(unavailable, { state: "unavailable" });
    process.env.PAXEER_X_EXPLORER_BASE_URL = "http://127.0.0.1";
    for (const [operation, capability] of [
      [() => client.programRecord("a".repeat(64)), "program-read"],
      [() => client.resolveName("alice"), "naming-program-read"],
      [() => client.unifiedAccountAvailability("0x" + "1".repeat(40)), "legacy-account-projection"],
    ] as const) {
      await assert.rejects(operation(), (error: unknown) => {
        assert.ok(error instanceof client.ExplorerInterfaceUnavailableError);
        assert.equal(error.capability, capability);
        return true;
      });
    }
  } finally {
    if (previous === undefined) delete process.env.PAXEER_X_EXPLORER_BASE_URL;
    else process.env.PAXEER_X_EXPLORER_BASE_URL = previous;
  }
}

test(explorer_retired_transport_native_http);
test(native_unified_shape_http_to_page);
test(native_unified_cursor_reopen_and_refusal);
test(native_unified_expiry_and_reorg_refusal);
test(native_explorer_validation_and_missing_interfaces);
