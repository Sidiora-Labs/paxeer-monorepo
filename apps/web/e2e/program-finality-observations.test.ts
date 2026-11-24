import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { createServer } from "node:http";
import { registerHooks } from "node:module";
import test, { type TestContext } from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

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

const { createElement } = await import("react");
const { renderToStaticMarkup } = await import("react-dom/server");
const { Window } = await import("happy-dom");
const { decodeProgramForIdentifier } = await import("../src/explorer/model.ts");
const { copyEntry } = await import("../copy/runtime.ts");
const { ProgramDetails } = await import("../src/app/explorer/programs/[programId]/program-details.tsx");
const { default: ProgramPage } = await import("../src/app/explorer/programs/[programId]/page.tsx");
const { programRecord, ExplorerInterfaceUnavailableError } = await import("../src/explorer/client.ts");

const PROGRAM_ID = "ab".repeat(32);
const OBSERVED_AT = "1700000000000";
const NOW = 1700000005000;

function programObservationDocument() {
  return {
    program: PROGRAM_ID,
    upgrade_policy: { kind: "immutable" },
    lifecycle: "active",
    versions: [{
      version: "1", code_hash: "56".repeat(32), abi_version: "1",
      source: { status: "unpublished" },
    }],
    value_accounts: [
      { account: "ef".repeat(32), asset: "78".repeat(32), balance: "7", frozen: false },
      { account: "12".repeat(32), asset: "78".repeat(32), balance: "9007199254740993", frozen: true },
    ],
    observed_sequence: "142",
    observed_at: OBSERVED_AT,
    receipt_digest: "cd".repeat(32),
    state_root: "90".repeat(32),
  };
}

async function observationStore(context: TestContext) {
  const observations = new Map<string, unknown>([[PROGRAM_ID, programObservationDocument()]]);
  const requests: string[] = [];
  const server = createServer((request, response) => {
    requests.push(`${request.method} ${request.url}`);
    const document = observations.get((request.url ?? "").slice("/observations/".length));
    if (request.method !== "GET" || !request.url?.startsWith("/observations/") || document === undefined) {
      response.writeHead(404).end();
      return;
    }
    response.writeHead(200, { "content-type": "application/json" });
    response.end(JSON.stringify(document));
  });
  context.after(() => new Promise<void>((resolve, reject) => {
    server.close((error) => error === undefined ? resolve() : reject(error));
    server.closeAllConnections();
  }));
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  const address = server.address();
  assert.ok(address !== null && typeof address !== "string");
  const origin = `http://127.0.0.1:${address.port}`;
  async function read() {
    const response = await fetch(`${origin}/observations/${PROGRAM_ID}`, { signal: AbortSignal.timeout(5000) });
    assert.equal(response.status, 200);
    return decodeProgramForIdentifier(await response.json(), PROGRAM_ID);
  }
  return { observations, requests, origin, read };
}

async function withRenderedProgram(
  context: TestContext,
  program: ReturnType<typeof decodeProgramForIdentifier>,
  now: number,
) {
  const browser = new Window();
  context.after(() => browser.happyDOM.close());
  browser.document.body.innerHTML = renderToStaticMarkup(createElement(ProgramDetails, { program, now }));
  const tables = [...browser.document.querySelectorAll("table")];
  assert.equal(tables.length, 3);
  const facts = tables.find((table) => table.caption?.textContent === copyEntry("explorer.program.facts").message);
  const accounts = tables.find((table) => table.caption?.textContent === copyEntry("explorer.program.accounts").message);
  assert.ok(facts);
  assert.ok(accounts);
  const status = browser.document.querySelector('[role="status"]');
  assert.ok(status);
  for (const table of [facts, accounts]) {
    const rows = [...table.querySelectorAll("tbody tr")];
    assert.equal(rows.length, table === facts ? 6 : program.valueAccounts.length);
    for (const row of rows) {
      assert.equal(row.lastElementChild?.textContent, "Reported");
      assert.equal(row.querySelector("[data-verification]"), null);
    }
  }
  assert.ok(status.textContent.includes("Reported program observation"));
  assert.ok(status.textContent.includes("Currentness unknown."));
  assert.equal(status.classList.contains("border-success"), false);
  assert.equal(status.classList.contains("border-warning"), true);
  assert.equal(browser.document.querySelectorAll('[data-verification="verified"]').length,
    program.versions.filter((version) => version.source.status === "verified").length);
  assert.ok(!browser.document.body.textContent.includes("checkpoint-finalised"));
  return { browser, facts, accounts, status };
}

async function program_unearned_finality_reports_observations_without_finality(context: TestContext) {
  const store = await observationStore(context);
  const program = await store.read();
  const view = await withRenderedProgram(context, program, NOW);
  assert.ok(view.status.textContent.includes("Reported sequence 142"));
  assert.ok(view.status.textContent.includes(`at ${OBSERVED_AT} (Unix milliseconds)`));
  assert.ok(view.status.textContent.includes("Reported age: 5000 ms at page render."));
  assert.ok(view.facts.textContent.includes("immutable"));
  assert.ok(view.accounts.textContent.includes("9007199254740993"));
  assert.ok(view.accounts.textContent.includes("true"));
  assert.equal(view.facts.querySelector("a")?.getAttribute("href"), `/explorer/receipts/${program.receiptDigest}`);
  assert.ok(view.facts.textContent.includes(program.stateRoot));
  assert.deepEqual(store.requests, [`GET /observations/${PROGRAM_ID}`]);
}

async function program_unearned_finality_does_not_infer_currentness_from_age(context: TestContext) {
  const store = await observationStore(context);
  const program = await store.read();
  for (const [now, age] of [
    [Number(OBSERVED_AT), "0"],
    [NOW, "5000"],
    [NOW + 86_400_000, "86405000"],
  ] as const) {
    const view = await withRenderedProgram(context, program, now);
    assert.ok(view.status.textContent.includes(`Reported age: ${age} ms at page render.`));
  }
}

async function program_unearned_finality_future_timestamp_has_unknown_age(context: TestContext) {
  const store = await observationStore(context);
  const future = "18446744073709551615";
  store.observations.set(PROGRAM_ID, { ...programObservationDocument(), observed_at: future });
  const program = await store.read();
  assert.equal(program.observedAt, future);
  const view = await withRenderedProgram(context, program, NOW);
  const ahead = BigInt(future) - BigInt(NOW);
  assert.ok(view.status.textContent.includes(`Reported age: Unknown: reported timestamp is ${ahead} ms ahead`));
  assert.ok(view.status.textContent.includes(`at ${future}`));
}

async function program_unearned_finality_ignores_reported_verification_claims(context: TestContext) {
  const store = await observationStore(context);
  const document = programObservationDocument();
  store.observations.set(PROGRAM_ID, {
    ...document,
    verification_level: "checkpoint-finalised",
    current: true,
    observed_head: document.observed_sequence,
    versions: [{ ...document.versions[0], source: {
      status: "verified", source_digest: "34".repeat(32), environment_digest: "56".repeat(32),
    } }],
  });
  const program = await store.read();
  assert.equal("verificationLevel" in program, false);
  assert.equal("current" in program, false);
  const view = await withRenderedProgram(context, program, NOW);
  assert.equal(view.browser.document.querySelectorAll('[data-verification="verified"]').length, 1,
    "source reproduction status must not promote program facts or balances to committed evidence");
}

async function program_unearned_finality_missing_native_interface_stays_unavailable(context: TestContext) {
  const store = await observationStore(context);
  const previous = process.env.PAXEER_X_EXPLORER_BASE_URL;
  process.env.PAXEER_X_EXPLORER_BASE_URL = store.origin;
  context.after(() => {
    if (previous === undefined) delete process.env.PAXEER_X_EXPLORER_BASE_URL;
    else process.env.PAXEER_X_EXPLORER_BASE_URL = previous;
  });
  await assert.rejects(programRecord(PROGRAM_ID), (error: unknown) =>
    error instanceof ExplorerInterfaceUnavailableError && error.capability === "program-read");
  const html = renderToStaticMarkup(await ProgramPage({ params: Promise.resolve({ programId: PROGRAM_ID }) }));
  assert.ok(html.includes(copyEntry("explorer.freshness.unavailable").message));
  assert.ok(!html.includes("Reported program observation"));
  assert.ok(!html.includes('data-verification="verified"'));
  assert.deepEqual(store.requests, [], "the production client must not invent a native program endpoint");
}

test(program_unearned_finality_reports_observations_without_finality);
test(program_unearned_finality_does_not_infer_currentness_from_age);
test(program_unearned_finality_future_timestamp_has_unknown_age);
test(program_unearned_finality_ignores_reported_verification_claims);
test(program_unearned_finality_missing_native_interface_stays_unavailable);
