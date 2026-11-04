// Proves: explorer/model.ts typed records and the test-owned HTTP producer -> explorer/client.ts programRecord, receiptRecord, resolveName, unifiedAccountAvailability -> AccountPage, ProgramPage, ReceiptPage and lookup GET; links.ts explorerLinkPath and components.tsx verificationLabel -> kit/explorer.tsx consumers.
// Repairs: client.ts cached receipt reads and accepted another receipt's ID; components.tsx
// explorerReceiptPath accepted malformed identifiers. Account and program pages now use that
// validated path. EXPLORER_ANCHOR_PATH remains a retired surface and is exercised as a refusal.
// Repairs: perf/budgets.ts classifyPerformanceRoute admitted retired explorer routes;
// it now admits only the hrefs offered by links.ts EXPLORER_NAVIGATION.
// No state persists in this adapter: two independent HTTP reads must observe producer changes.
import assert from "node:assert/strict";
import { channel } from "node:diagnostics_channel";
import { once } from "node:events";
import { createServer } from "node:http";
import { existsSync, readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import test, { after, type TestContext } from "node:test";
import { fileURLToPath } from "node:url";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import ts from "typescript";

import type { ExplorerLinkTarget } from "../src/explorer/links.ts";
import type {
  ExplorerFreshness,
  ExplorerVerificationLevel,
  NameResolutionRecord,
  ProgramRecord,
  ReceiptRecord,
  UnifiedAccountAvailabilityRecord,
} from "../src/explorer/model.ts";

// Resolve and compile the actual sources, including UI sources, so the verify command
// needs no unbuilt UI dist entry. Next's empty server-only marker is its server entry.
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

const PROGRAM_ID = "ab".repeat(32);
const RECEIPT_ID = "cd".repeat(32);
const ACCOUNT_ID = "ef".repeat(32);
const NAMING_PROGRAM = "12".repeat(32);
const NAME = "explorer-proof";
const UNKNOWN_ID = "00".repeat(32);

// These are the repository's actual record types. Only the HTTP representation uses
// snake_case; the client decodes it and the actual server components render the result.
function explorerProducerState() {
  const freshness: ExplorerFreshness = {
    observedChainSequence: "142", observedSealedBatch: "21",
    observedFinalisedCheckpoint: "34".repeat(32), indexedBatch: "21",
    batchesBehind: "0", current: true,
  };
  const receipt: ReceiptRecord = {
    receiptId: RECEIPT_ID, batchNumber: "21", ordinal: "1",
    canonicalBytes: Buffer.from("explorer receipt 1").toString("base64url"),
    verificationLevel: "receipt-verified",
  };
  const program: ProgramRecord = {
    program: PROGRAM_ID, upgradePolicy: { kind: "immutable" }, lifecycle: "active",
    versions: [{ version: "1", codeHash: "56".repeat(32), abiVersion: "1", source: { status: "unpublished" } }],
    valueAccounts: [{ account: ACCOUNT_ID, asset: "78".repeat(32), balance: "7", frozen: false }],
    observedSequence: "142", observedAt: "1001", receiptDigest: RECEIPT_ID,
    stateRoot: "90".repeat(32),
  };
  const present = (value: string) => ({ state: "present", value, evidence: "gateway-reported" } as const);
  const account: UnifiedAccountAvailabilityRecord = {
    profile: 2, requested: ACCOUNT_ID, canonical: ACCOUNT_ID, evidence: "gateway-reported",
    identities: { bound: true, layerxAccount: ACCOUNT_ID, layerxDid: `did:layerx:${ACCOUNT_ID}` },
    balances: {
      joinedLimit: "1024",
      items: [{ assetId: "78".repeat(32), denom: present("PAXD"), custody: "7", layerx: "2", paxeer: "5" }],
    },
    settlement: {
      networkId: "paxeer-x", chainId: "125", instantBlock: "120", sealedBatch: "21",
      finalizedBatch: present("20"), anchorStatus: present("2"), anchorStatusName: present("final"),
    },
    paxeerActivity: { items: [], fromBlock: "70", toBlock: "120" },
    layerxActivity: {
      items: [{
        receiptId: RECEIPT_ID, receiptDigest: RECEIPT_ID, batchNumber: "21", globalSequence: "142",
        activityId: "23".repeat(32), operation: "1", resultCode: "0", asset: "78".repeat(32),
        amount: "7", from: ACCOUNT_ID, to: ACCOUNT_ID, verificationLevel: "receipt-verified",
      }]
    },
    freshness,
  };
  const resolution: NameResolutionRecord = { name: NAME, did: ACCOUNT_ID, expiry: "9999999999" };
  return { freshness, receipt, program, account, resolution };
}

function explorerWireValue(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(explorerWireValue);
  if (typeof value !== "object" || value === null) return value;
  return Object.fromEntries(Object.entries(value).map(([key, item]) => [
    key.replace(/[A-Z]/gu, (letter) => `_${letter.toLowerCase()}`),
    key === "verificationLevel"
      ? item === "receipt-verified" ? "sequencer-signed" : item
      : explorerWireValue(item),
  ]));
}

function explorerAccountDocument(account: UnifiedAccountAvailabilityRecord) {
  const legacy = (item: UnifiedAccountAvailabilityRecord["settlement"]["finalizedBatch"]) =>
    item.state === "present" ? item.value : null;
  return {
    ...explorerWireObject(account),
    balances: {
      joined_limit: account.balances.joinedLimit,
      items: account.balances.items.map((item) => ({ ...explorerWireObject(item), denom: legacy(item.denom) })),
    },
    settlement: {
      ...explorerWireObject(account.settlement),
      finalized_batch: legacy(account.settlement.finalizedBatch),
      anchor_status: legacy(account.settlement.anchorStatus),
      anchor_status_name: legacy(account.settlement.anchorStatusName),
    },
    availability: {
      denominations: account.balances.items.map((item) => ({ asset_id: item.assetId, denom: item.denom })),
      settlement: {
        finalized_batch: account.settlement.finalizedBatch,
        anchor_status: account.settlement.anchorStatus,
        anchor_status_name: account.settlement.anchorStatusName,
      },
    },
    layerx_activity: {
      items: account.layerxActivity.items.map((item) => ({
        ...explorerWireObject(item), operation: Number(item.operation), result_code: Number(item.resultCode), verification: 1,
      })),
    },
  };
}

function explorerWireObject(value: object): Record<string, unknown> {
  return explorerWireValue(value) as Record<string, unknown>;
}

async function startExplorerProducer() {
  const state = explorerProducerState();
  const reads: { path: string; profile: string | undefined; }[] = [];
  let corruption: "receipt-id" | "receipt-bytes" | "program" | "account" | "name" | undefined;
  const server = createServer((request, response) => {
    const url = new URL(request.url!, "http://localhost");
    reads.push({ path: url.pathname, profile: request.headers["layerx-unified-profile"] as string | undefined });
    response.setHeader("Content-Type", "application/json");
    let body: unknown;
    if (url.pathname === `/v1/explorer/receipts/${RECEIPT_ID}`) {
      body = {
        value: {
          ...explorerWireObject(state.receipt),
          ...(corruption === "receipt-id" ? { receipt_id: UNKNOWN_ID } : {}),
          ...(corruption === "receipt-bytes" ? { canonical_bytes: "!" } : {}),
        },
        freshness: explorerWireValue(state.freshness),
      };
    } else if (url.pathname === `/v1/programs/${PROGRAM_ID}`) {
      body = { ...explorerWireObject(state.program), ...(corruption === "program" ? { lifecycle: "paused" } : {}) };
    } else if (url.pathname === `/v1/accounts/${ACCOUNT_ID}/unified`) {
      assert.equal(request.headers["layerx-unified-profile"], "2");
      assert.equal(url.searchParams.get("limit"), "25");
      body = { ...explorerAccountDocument(state.account), ...(corruption === "account" ? { profile: 1 } : {}) };
    } else if (url.pathname === `/v1/programs/${NAMING_PROGRAM}/reads/resolve` && url.searchParams.get("name") === NAME) {
      body = { ...state.resolution, ...(corruption === "name" ? { name: "another-name" } : {}) };
    } else {
      response.statusCode = 404;
      body = { freshness: explorerWireValue(state.freshness) };
    }
    response.end(JSON.stringify(body));
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.ok(address && typeof address === "object", "apps/web/src/explorer/client.ts needs the test-owned HTTP producer");
  return {
    state, reads, origin: `http://127.0.0.1:${address.port}`,
    corrupt(value: typeof corruption) { corruption = value; },
    async close() { await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve())); },
  };
}

// Start and configure our producer before importing the consumer's client module.
const producer = await startExplorerProducer();
const previousOrigin = process.env.PAXEER_X_EXPLORER_BASE_URL;
const previousNamingProgram = process.env.LAYERX_EXPLORER_NAMING_PROGRAM;
process.env.PAXEER_X_EXPLORER_BASE_URL = producer.origin;
process.env.LAYERX_EXPLORER_NAMING_PROGRAM = NAMING_PROGRAM;
after(async () => {
  await producer.close();
  if (previousOrigin === undefined) delete process.env.PAXEER_X_EXPLORER_BASE_URL;
  else process.env.PAXEER_X_EXPLORER_BASE_URL = previousOrigin;
  if (previousNamingProgram === undefined) delete process.env.LAYERX_EXPLORER_NAMING_PROGRAM;
  else process.env.LAYERX_EXPLORER_NAMING_PROGRAM = previousNamingProgram;
});

const client = await import("../src/explorer/client.ts");
const components = await import("../src/explorer/components.tsx");
const links = await import("../src/explorer/links.ts");
const model = await import("../src/explorer/model.ts");
const { ExplorerExternalLink, ExplorerVerificationBadge } = await import("../src/kit/explorer.tsx");
const { copyEntry } = await import("../copy/runtime.ts");
const { classifyPerformanceRoute } = await import("../src/perf/budgets.ts");
const { default: AccountPage } = await import("../src/app/explorer/accounts/[accountId]/page.tsx");
const { default: ProgramPage } = await import("../src/app/explorer/programs/[programId]/page.tsx");
const { default: ReceiptPage } = await import("../src/app/explorer/receipts/[receiptId]/page.tsx");
const { GET: lookup } = await import("../src/app/explorer/lookup/route.ts");
const { NextRequest } = await import("next/server.js");

function restoreEnvironment(context: TestContext, key: string, value: string | undefined): void {
  const previous = process.env[key];
  context.after(() => {
    if (previous === undefined) delete process.env[key];
    else process.env[key] = previous;
  });
  if (value === undefined) delete process.env[key];
  else process.env[key] = value;
}

function explorer_links_and_labels_reach_real_consumers(context: TestContext): void {
  restoreEnvironment(context, "PAXEER_X_EXPLORER_BASE_URL", producer.origin);
  const targets: readonly ExplorerLinkTarget[] = [
    { kind: "receipt", receiptId: "ab".repeat(32).toUpperCase(), transactionHash: `0x${"b".repeat(64)}` },
    { kind: "transaction", transactionHash: `0x${"cd".repeat(32)}` },
    { kind: "address", address: `0x${"ef".repeat(20)}` },
  ];
  for (const target of targets) {
    const path = links.explorerLinkPath(target);
    const href = client.explorerLink(target);
    const first = renderToStaticMarkup(createElement(ExplorerExternalLink, { href, children: path }));
    const second = renderToStaticMarkup(createElement(ExplorerExternalLink, {
      href: client.explorerLink(target), children: links.explorerLinkPath(target),
    }));
    assert.equal(first, second);
    assert.ok(first.includes(`href="${producer.origin}${path}"`));
    assert.ok(first.includes('data-explorer-link="external"'));
    assert.equal(classifyPerformanceRoute(path), undefined);
  }
  assert.equal(links.EXPLORER_ANCHOR_PATH, "/paxeer-x/anchors");
  assert.equal(classifyPerformanceRoute(links.EXPLORER_ANCHOR_PATH), undefined);
  for (const kind of ["anchor", "batch", "checkpoint"]) {
    // @ts-expect-error Deliberately exercise a retired target at the runtime boundary.
    assert.throws(() => links.explorerLinkPath({ kind }), TypeError);
  }
  for (const identifier of ["../anchors", "", `${"ab".repeat(32)}?tab=anchors`]) {
    assert.throws(() => components.explorerReceiptPath(identifier), TypeError);
    assert.throws(() => links.explorerLinkPath({ kind: "receipt", receiptId: identifier, transactionHash: `0x${"b".repeat(64)}` }), TypeError);
  }
  assert.equal(components.explorerReceiptPath("AB".repeat(32)), `/explorer/receipts/${"ab".repeat(32)}`);
  const levels: readonly [ExplorerVerificationLevel, string][] = [
    ["unverified", "verification.unverified"],
    ["receipt-verified", "verification.receipt_verified"],
    ["checkpoint-finalised", "verification.checkpoint_finalised"],
  ];
  for (const [level, key] of levels) {
    const label = components.verificationLabel(level);
    const html = renderToStaticMarkup(createElement(ExplorerVerificationBadge, {
      label, unverified: level === "unverified",
    }));
    assert.equal(label, copyEntry(key).message);
    assert.ok(html.includes(label));
    assert.ok(html.includes(`data-verification="${level === "unverified" ? "unverified" : "verified"}"`));
  }
}

async function explorer_pages_refuse_invalid_input(context: TestContext): Promise<void> {
  restoreEnvironment(context, "PAXEER_X_EXPLORER_BASE_URL", undefined);
  for (const read of [client.receiptRecord, client.programRecord, client.unifiedAccountAvailability]) {
    await assert.rejects(read("../anchors"), TypeError);
  }
  await assert.rejects(client.resolveName("INVALID NAME"), TypeError);
  for (const page of [
    () => ReceiptPage({ params: Promise.resolve({ receiptId: "../anchors" }) }),
    () => ProgramPage({ params: Promise.resolve({ programId: "../anchors" }) }),
  ]) {
    const html = renderToStaticMarkup(await page());
    assert.ok(html.includes(copyEntry("explorer.freshness.unavailable").message));
    assert.ok(html.includes('data-application="explorer"'));
    assert.ok(!html.includes('data-verification="verified"'));
  }
  const account = renderToStaticMarkup(await AccountPage({
    params: Promise.resolve({ accountId: "../anchors" }), searchParams: Promise.resolve({}),
  }));
  assert.ok(account.includes(copyEntry("explorer.account.invalid").message));
  const response = await lookup(new NextRequest("http://localhost/explorer/lookup?kind=receipt&identifier=../anchors"));
  assert.equal(response.status, 303);
  assert.equal(new URL(response.headers.get("location")!).search, "?lookup=invalid");
  assert.throws(() => client.decodeReceiptRecordForIdentifier(null, "ab".repeat(32)), TypeError);
}

function explorer_route_classifier_admits_only_navigation(): void {
  for (const item of links.EXPLORER_NAVIGATION) {
    assert.equal(classifyPerformanceRoute(item.href), "explorer", item.href);
  }
  for (const path of [
    "/explorer/batches", "/explorer/batches/21",
    "/explorer/checkpoints", "/explorer/checkpoints/34",
    `/explorer/receipts/${RECEIPT_ID}`,
    `/explorer/programs/${PROGRAM_ID}`,
    `/explorer/accounts/${ACCOUNT_ID}`,
    "/explorer/lookup", "/explorer/unknown", "/explorer/",
  ]) {
    assert.equal(classifyPerformanceRoute(path), undefined, path);
  }
}

async function explorer_real_producers_reach_pages_twice_and_refuse_malformed_results(context: TestContext): Promise<void> {
  const origin = producer.origin;
  const programId = PROGRAM_ID;
  const receiptId = RECEIPT_ID;
  const accountId = ACCOUNT_ID;
  const name = NAME;
  assert.ok(links.parseExplorerBaseUrl(origin));
  assert.ok(model.validExplorerIdentifier(programId));
  assert.ok(model.validExplorerIdentifier(receiptId));
  assert.ok(model.parseAccountIdentifier(accountId));
  assert.ok(model.validExplorerName(name));

  const observed: { origin: string; path: string; }[] = [];
  const requests = channel("undici:request:create");
  const observe = (message: unknown): void => {
    const { request } = message as { request: { origin: string; path: string; }; };
    observed.push({ origin: String(request.origin), path: request.path });
  };
  requests.subscribe(observe);
  context.after(() => requests.unsubscribe(observe));
  for (let pass = 0; pass < 2; pass += 1) {
    producer.state.program = { ...producer.state.program, observedAt: String(1001 + pass) };
    producer.state.receipt = { ...producer.state.receipt, canonicalBytes: Buffer.from(`explorer receipt ${pass}`).toString("base64url") };
    producer.state.account = {
      ...producer.state.account,
      balances: {
        ...producer.state.account.balances,
        items: producer.state.account.balances.items.map((item) => ({
          ...item, denom: { state: "present", value: `PAXD-${pass}`, evidence: "gateway-reported" } as const,
        })),
      },
    };
    producer.state.resolution = { ...producer.state.resolution, did: pass === 0 ? ACCOUNT_ID : "35".repeat(32) };
    const program = await client.programRecord(programId);
    const receipt = await client.receiptRecord(receiptId);
    const account = await client.unifiedAccountAvailability(accountId);
    const resolution = await client.resolveName(name);
    assert.ok(program);
    assert.ok(receipt.value);
    assert.ok(account);
    assert.ok(resolution);
    assert.equal(program.program, programId.toLowerCase());
    assert.equal(receipt.value.receiptId, receiptId.toLowerCase());
    assert.equal(account.requested, model.parseAccountIdentifier(accountId)!.canonical);
    assert.equal(resolution.name, name);
    assert.deepEqual(program, producer.state.program);
    assert.deepEqual(receipt.value, producer.state.receipt);
    assert.deepEqual(account, producer.state.account);
    assert.deepEqual(resolution, producer.state.resolution);
    const start = observed.length;
    const programHtml = renderToStaticMarkup(await ProgramPage({ params: Promise.resolve({ programId }) }));
    assert.ok(programHtml.includes(program.program));
    assert.ok(programHtml.includes(program.stateRoot));
    assert.ok(programHtml.includes(`at ${program.observedAt}.`));
    assert.ok(programHtml.includes(components.explorerReceiptPath(program.receiptDigest)));
    const receiptHtml = renderToStaticMarkup(await ReceiptPage({ params: Promise.resolve({ receiptId }) }));
    assert.ok(receiptHtml.includes(receipt.value.receiptId));
    assert.ok(receiptHtml.includes(receipt.value.canonicalBytes));
    assert.ok(receiptHtml.includes(components.verificationLabel(receipt.value.verificationLevel)));
    const accountHtml = renderToStaticMarkup(await AccountPage({
      params: Promise.resolve({ accountId: account.canonical }), searchParams: Promise.resolve({}),
    }));
    assert.ok(accountHtml.includes(account.canonical));
    assert.ok(accountHtml.includes(`PAXD-${pass}`));
    assert.ok(accountHtml.includes(copyEntry("explorer.account.gateway_reported").message));
    assert.ok(accountHtml.includes(components.explorerReceiptPath(receiptId)));
    const resolved = await lookup(new NextRequest(`http://localhost/explorer/lookup?kind=account&identifier=${encodeURIComponent(name)}`));
    assert.equal(resolved.status, 303);
    assert.equal(new URL(resolved.headers.get("location")!).pathname, model.accountIdentifierPath(resolution.did));
    assert.equal(observed.length - start, 4, "every consumer must read the producer again on each pass");
  }
  assert.ok(observed.every((request) => request.origin === new URL(origin).origin));

  assert.equal(producer.reads.length, 16, "all four producers must receive both reads on both passes");
  assert.equal(producer.reads.filter((read) => read.profile === "2").length, 4);

  // Corrupt freshly emitted documents at the real HTTP producer, then exercise
  // the same decoder boundary and pages rather than replacing the transport.
  for (const corruption of ["receipt-id", "receipt-bytes"] as const) {
    producer.corrupt(corruption);
    await assert.rejects(client.receiptRecord(receiptId), TypeError);
    const html = renderToStaticMarkup(await ReceiptPage({ params: Promise.resolve({ receiptId }) }));
    assert.ok(html.includes(copyEntry("explorer.freshness.unavailable").message));
    assert.ok(!html.includes('data-verification="verified"'));
  }
  producer.corrupt("program");
  await assert.rejects(client.programRecord(programId), TypeError);
  assert.ok(renderToStaticMarkup(await ProgramPage({ params: Promise.resolve({ programId }) })).includes(copyEntry("explorer.freshness.unavailable").message));
  producer.corrupt("account");
  await assert.rejects(client.unifiedAccountAvailability(accountId), TypeError);
  assert.ok(renderToStaticMarkup(await AccountPage({ params: Promise.resolve({ accountId }), searchParams: Promise.resolve({}) })).includes(copyEntry("explorer.freshness.unavailable").message));
  producer.corrupt("name");
  await assert.rejects(client.resolveName(name), /another name/u);
  const refusedName = await lookup(new NextRequest(`http://localhost/explorer/lookup?kind=account&identifier=${name}`));
  assert.equal(new URL(refusedName.headers.get("location")!).search, "?lookup=unavailable");
  producer.corrupt(undefined);
  assert.equal(await client.programRecord(UNKNOWN_ID), undefined);
  assert.equal((await client.receiptRecord(UNKNOWN_ID)).value, undefined);
  assert.equal(await client.unifiedAccountAvailability(UNKNOWN_ID), undefined);
  const unknownAccount = renderToStaticMarkup(await AccountPage({
    params: Promise.resolve({ accountId: UNKNOWN_ID }), searchParams: Promise.resolve({}),
  }));
  assert.ok(unknownAccount.includes(copyEntry("explorer.freshness.unavailable").message));
  assert.equal(await client.resolveName("unknown-name"), undefined);
  const unknownName = await lookup(new NextRequest("http://localhost/explorer/lookup?kind=account&identifier=unknown-name"));
  assert.equal(new URL(unknownName.headers.get("location")!).search, "?lookup=unknown-name");
  for (const html of [
    renderToStaticMarkup(await ProgramPage({ params: Promise.resolve({ programId: UNKNOWN_ID }) })),
    renderToStaticMarkup(await ReceiptPage({ params: Promise.resolve({ receiptId: UNKNOWN_ID }) })),
  ]) {
    assert.ok(html.includes(copyEntry("explorer.not_found.body").message));
  }

  // Also retain direct boundary assertions against documents read over HTTP.
  const receiptResponse = await fetch(new URL(`/v1/explorer/receipts/${receiptId.toLowerCase()}`, origin), { cache: "no-store" });
  assert.equal(receiptResponse.status, 200);
  const rawReceipt = await receiptResponse.json();
  assert.deepEqual(client.decodeReceiptRecordForIdentifier(rawReceipt, receiptId), await client.receiptRecord(receiptId));
  assert.throws(() => client.decodeReceiptRecordForIdentifier(rawReceipt, receiptId === "00".repeat(32) ? "11".repeat(32) : "00".repeat(32)), /another receipt/u);
  assert.throws(() => client.decodeReceiptRecordForIdentifier({ ...rawReceipt, value: { ...rawReceipt.value, canonical_bytes: "!" } }, receiptId), TypeError);
  const programResponse = await fetch(new URL(`/v1/programs/${programId.toLowerCase()}`, origin), { cache: "no-store" });
  assert.equal(programResponse.status, 200);
  const rawProgram = await programResponse.json();
  assert.throws(() => model.decodeProgramForIdentifier({ ...rawProgram, lifecycle: "paused" }, programId), TypeError);
  const accountResponse = await fetch(new URL(`/v1/accounts/${encodeURIComponent(accountId)}/unified?limit=25`, origin), {
    headers: { "LayerX-Unified-Profile": "2" }, cache: "no-store",
  });
  assert.equal(accountResponse.status, 200);
  const rawAccount = await accountResponse.json();
  assert.throws(() => model.decodeUnifiedAccountAvailability({ ...rawAccount, profile: 1 }), TypeError);
  await assert.rejects(client.programRecord("../program"), TypeError);
  await assert.rejects(client.resolveName("INVALID NAME"), TypeError);
}

test(explorer_links_and_labels_reach_real_consumers);
test(explorer_pages_refuse_invalid_input);
test(explorer_route_classifier_admits_only_navigation);
test(explorer_real_producers_reach_pages_twice_and_refuse_malformed_results);

// Execute the existing Node consumers of the same exported module as part of this proof.
await import("./explorer-admission.test.ts");
await import("./explorer.spec.ts");
