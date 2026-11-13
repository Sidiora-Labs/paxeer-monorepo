import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { createServer } from "node:http";
import { registerHooks } from "node:module";
import test, { type TestContext } from "node:test";
import { fileURLToPath } from "node:url";
import { renderToStaticMarkup } from "react-dom/server";
import ts from "typescript";
import type { NativeUnifiedAccount } from "../src/explorer/native-model.ts";

// Compile the actual route and UI sources in Node, using Next's real server marker.
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "server-only") return nextResolve("next/dist/compiled/server-only/empty.js", context);
    if (["next/navigation", "next/link", "next/server"].includes(specifier)) {
      return nextResolve(`${specifier}.js`, context);
    }
    if (specifier.startsWith("@layerx/ui/")) {
      const path = specifier === "@layerx/ui/cn" ? "lib/utils" : specifier.slice("@layerx/ui/".length);
      const base = new URL(`../packages/layerx-ui/src/${path}`, import.meta.url);
      for (const suffix of [".ts", ".tsx"]) {
        const source = new URL(base.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const base = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = new URL(base.href + suffix);
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

const { default: AccountPage } = await import("../src/app/explorer/accounts/[accountId]/page.tsx");
const { decodeNativeUnifiedAccount } = await import("../src/explorer/native-model.ts");
const { Window } = await import("happy-dom");

interface NativeTranscript {
  address: string;
  missingAddress: string;
  unavailableAddress: string;
  cursors: { first: string; next: string; expired: string; reorg: string };
  firstPage: NativeUnifiedAccount;
  secondPage: NativeUnifiedAccount;
  expiredRefusal: { message: string; reason: string };
  snapshotRefusal: { message: string; reason: string };
  notFound: { message: string };
  unavailable: { message: string };
}

type AccountQuery = Parameters<typeof AccountPage>[0]["searchParams"] extends Promise<infer Query> ? Query : never;

async function accountProducer(context: TestContext) {
  const transcript: NativeTranscript = JSON.parse(readFileSync(
    new URL("../src/explorer/fixtures/repair-22-31-383-native-http.json", import.meta.url), "utf8",
  ));
  for (const page of [transcript.firstPage, transcript.secondPage]) {
    assert.deepEqual(decodeNativeUnifiedAccount(page), page);
  }
  const accountPath = `/api/v2/addresses/${transcript.address}/unified`;
  const continuation = (cursor: string): string => `${accountPath}?${new URLSearchParams({ cursor })}`;
  const exchanges = new Map<string, readonly [number, unknown]>([
    [accountPath, [200, transcript.firstPage]],
    [continuation(transcript.cursors.first), [200, transcript.firstPage]],
    [continuation(transcript.cursors.next), [200, transcript.secondPage]],
    [continuation(transcript.cursors.expired), [422, transcript.expiredRefusal]],
    [continuation(transcript.cursors.reorg), [422, transcript.snapshotRefusal]],
    [`/api/v2/addresses/${transcript.missingAddress}/unified`, [404, transcript.notFound]],
    [`/api/v2/addresses/${transcript.unavailableAddress}/unified`, [503, transcript.unavailable]],
  ]);
  const requests: string[] = [];
  const unexpected: string[] = [];
  const server = createServer((request, response) => {
    const path = request.url ?? "";
    requests.push(path);
    const exchange = exchanges.get(path);
    response.setHeader("Content-Type", "application/json");
    response.setHeader("Connection", "close");
    if (request.method !== "GET" || request.headers.authorization !== undefined || exchange === undefined) {
      unexpected.push(`${request.method} ${path}`);
      response.writeHead(500).end(JSON.stringify({ message: "Unrecorded request" }));
      return;
    }
    response.writeHead(exchange[0]).end(JSON.stringify(exchange[1]));
  });
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      server.off("error", reject);
      resolve();
    });
  });
  const bound = server.address();
  assert.ok(bound !== null && typeof bound !== "string");
  const origin = `http://127.0.0.1:${bound.port}`;
  const previous = process.env.PAXEER_X_EXPLORER_BASE_URL;
  process.env.PAXEER_X_EXPLORER_BASE_URL = origin;
  context.after(async () => {
    try {
      await new Promise<void>((resolve, reject) => {
        server.close((error) => error === undefined ? resolve() : reject(error));
        server.closeAllConnections();
      });
    } finally {
      if (previous === undefined) delete process.env.PAXEER_X_EXPLORER_BASE_URL;
      else process.env.PAXEER_X_EXPLORER_BASE_URL = previous;
    }
    assert.deepEqual(unexpected, [], "the route must use only recorded native account exchanges");
  });
  async function render(query: AccountQuery = {}, address = transcript.address) {
    const html = renderToStaticMarkup(await AccountPage({
      params: Promise.resolve({ accountId: address }),
      searchParams: Promise.resolve(query),
    }));
    const window = new Window();
    context.after(() => window.happyDOM.close());
    window.document.body.innerHTML = html;
    return window.document;
  }
  return { transcript, origin, requests, accountPath, continuation, render };
}

function assertNativePage(document: InstanceType<typeof Window>["document"], account: NativeUnifiedAccount) {
  const article = document.querySelector('article[aria-label="Native account"][data-state="ready"]');
  assert.ok(article, "the real account route must render the native account, rather than the retired reader's refusal");
  const tables = article.querySelectorAll("table");
  assert.equal(tables.length, 2);
  const balances = tables[0]!;
  assert.equal(balances.querySelector("caption")?.textContent, "Account totals in base units");
  assert.deepEqual([...balances.querySelectorAll("thead th")].map((cell) => cell.textContent), ["Asset", "Total"]);
  assert.deepEqual([...balances.querySelectorAll("tbody tr")].map((row) =>
    [...row.querySelectorAll("th, td")].map((cell) => cell.textContent)),
  account.balances.map((balance) => [balance.asset.denom, balance.total]));
  const activity = tables[1]!;
  assert.equal(activity.querySelector("caption")?.textContent, "Committed transactions");
  assert.deepEqual([...activity.querySelectorAll("tbody tr")].map((row) =>
    [...row.querySelectorAll("th, td")].map((cell) => cell.textContent)),
  account.activity.map((item) => [
    item.hash, String(item.block_number), String(item.ordinal), "committed", item.result ?? "Unavailable",
    `${item.amount} ${item.asset.denom}`, item.counterparty ?? "Unavailable", item.timestamp,
  ]));
  const observation = document.querySelector('section[aria-label="Observation position"]');
  assert.ok(observation);
  const first = account.activity.at(0);
  const last = account.activity.at(-1);
  assert.ok(first && last, "the committed transcript must contain activity");
  assert.ok(observation.textContent.includes(`committed block ${first.block_number}, ordinal ${first.ordinal}`));
  assert.ok(observation.textContent.includes(`through committed block ${last.block_number}, ordinal ${last.ordinal}`));
  assert.ok(observation.textContent.includes("does not establish the current network head"));
  assert.doesNotMatch(document.body.textContent, /on Paxeer|on LayerX|in custody|settlement|sealed|anchor|checkpoint|batch|Archive/iu);
}

async function account_retired_settlement_ui_native_totals_and_committed_position(context: TestContext) {
  const fixture = await accountProducer(context);
  const document = await fixture.render();
  assertNativePage(document, fixture.transcript.firstPage);
  assert.deepEqual(fixture.requests, [fixture.accountPath]);
  const links = [...document.querySelectorAll('nav[aria-label="Account activity pages"] a')];
  assert.equal(links.length, 3);
  const expected = [fixture.transcript.firstPage.page_cursor, fixture.transcript.firstPage.first_page_cursor,
    fixture.transcript.cursors.next];
  for (const [index, link] of links.entries()) {
    const url = new URL(link.getAttribute("href")!, fixture.origin);
    assert.equal(url.pathname, `/explorer/accounts/${fixture.transcript.address}`);
    assert.deepEqual([...url.searchParams], [["cursor", expected[index]]]);
  }
}

async function account_native_cursor_navigation_and_explicit_refusal(context: TestContext) {
  const fixture = await accountProducer(context);
  const first = await fixture.render();
  const older = first.querySelector('nav[aria-label="Account activity pages"] a:last-child');
  assert.ok(older);
  assert.equal(older.textContent, "Older transactions");
  const cursor = new URL(older.getAttribute("href")!, fixture.origin).searchParams.get("cursor");
  assert.equal(cursor, fixture.transcript.cursors.next);
  assert.ok(cursor);
  const second = await fixture.render({ cursor });
  assertNativePage(second, fixture.transcript.secondPage);
  assert.equal(second.querySelector('nav[aria-label="Account activity pages"]')?.textContent.includes("Older transactions"), false);
  assert.deepEqual(fixture.requests, [fixture.accountPath, fixture.continuation(cursor)]);
  for (const [refusedCursor, reason] of [
    [fixture.transcript.cursors.expired, fixture.transcript.expiredRefusal.reason],
    [fixture.transcript.cursors.reorg, fixture.transcript.snapshotRefusal.reason],
  ] as const) {
    const refused = await fixture.render({ cursor: refusedCursor });
    assert.ok(refused.querySelector('[data-state="cursor-refused"]')?.textContent.includes(reason));
    assert.equal(refused.querySelector("table"), null);
    assert.equal(refused.querySelector('[aria-label="Observation position"]'), null);
    const reset = refused.querySelector('a[href="/explorer/accounts/' + fixture.transcript.address + '"]');
    assert.equal(reset?.textContent, "Start a new account snapshot");
    const resetPage = await fixture.render();
    assertNativePage(resetPage, fixture.transcript.firstPage);
  }
}

async function account_native_missing_and_legacy_positions_do_not_invent_facts(context: TestContext) {
  const fixture = await accountProducer(context);
  const missing = await fixture.render({}, fixture.transcript.missingAddress);
  assert.equal(missing.querySelector('[data-state="not-found"]')?.textContent, "Account not found.");
  const unavailable = await fixture.render({}, fixture.transcript.unavailableAddress);
  assert.equal(unavailable.querySelector('[data-state="unavailable"]')?.textContent, "The account explorer is unavailable.");
  for (const document of [missing, unavailable]) {
    assert.equal(document.querySelector("table"), null);
    assert.equal(document.querySelector('[aria-label="Observation position"]'), null);
    assert.doesNotMatch(document.body.textContent, /settlement|batch|checkpoint|custody/iu);
  }
  const requestsBeforeInvalid = fixture.requests.length;
  for (const query of [{ before: "42" }, { beforeBlock: "51" }, { before_block: "51" }]) {
    const refused = await fixture.render(query);
    assert.ok(refused.querySelector('[data-state="cursor-refused"]'));
    assert.equal(refused.querySelector("a[href='/explorer/accounts/" + fixture.transcript.address + "']")?.textContent,
      "Start a new account snapshot");
    assert.equal(refused.querySelector("table"), null);
  }
  const invalid = await fixture.render({}, "not-an-account");
  assert.ok(invalid.body.textContent.includes("That is not an account of the Paxeer X Network"));
  assert.equal(fixture.requests.length, requestsBeforeInvalid, "invalid and retired positions must not silently start another snapshot");
  const upper = "0x" + "AB".repeat(20);
  await assert.rejects(AccountPage({
    params: Promise.resolve({ accountId: upper }),
    searchParams: Promise.resolve({ cursor: fixture.transcript.cursors.next }),
  }), (error: unknown) => {
    assert.ok(error instanceof Error && "digest" in error && typeof error.digest === "string");
    const target = `/explorer/accounts/${upper.toLowerCase()}?${new URLSearchParams({ cursor: fixture.transcript.cursors.next })}`;
    assert.equal(error.digest, `NEXT_REDIRECT;replace;${target};307;`);
    return true;
  });
  assert.equal(fixture.requests.length, requestsBeforeInvalid);
}

test(account_retired_settlement_ui_native_totals_and_committed_position);
test(account_native_cursor_navigation_and_explicit_refusal);
test(account_native_missing_and_legacy_positions_do_not_invent_facts);
