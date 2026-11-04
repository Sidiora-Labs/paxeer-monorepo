import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { createServer } from "node:http";
import { registerHooks } from "node:module";
import test, { type TestContext } from "node:test";
import { fileURLToPath } from "node:url";
import { renderToStaticMarkup } from "react-dom/server";
import ts from "typescript";

// Load production server components and UI sources, without a Next build or peer replacements.
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

const { default: ReceiptPage } = await import("../src/app/explorer/receipts/[receiptId]/page.tsx");
const { readNativeReceiptTransaction } = await import("../src/app/explorer/receipts/[receiptId]/native-receipt.ts");

async function recordedProducer(context: TestContext) {
  // Replay the committed native transaction transcript byte-for-byte.
  const transcript = JSON.parse(readFileSync(new URL(
    "../../../services/explorer/services/libs/blockscout-client/crate/tests/recorded/eth_blockscout_com/_api_v2_transactions_0x4dd7e3f4.json",
    import.meta.url,
  ), "utf8"));
  const archive = JSON.parse(readFileSync(new URL(
    "../src/explorer/fixtures/repair-22-31-383-native-http.json", import.meta.url,
  ), "utf8"));
  const transaction = JSON.parse(transcript.response.body);
  const transactionPath: string = transcript.request.urlPath;
  const responses = new Map<string, { status: number; body: string }>([
    [transactionPath, { status: transcript.response.status, body: transcript.response.body }],
    ...[archive.evmReceipt, archive.kernelReceipt].map((receipt): [string, { status: number; body: string }] => [
      `/api/v2/paxeer-x/archive/receipts/${receipt.id}`, { status: 200, body: JSON.stringify(receipt) },
    ]),
    [`/api/v2/transactions/0x${"0".repeat(64)}`, { status: 404, body: JSON.stringify(archive.notFound) }],
    [`/api/v2/paxeer-x/archive/receipts/0x${"0".repeat(64)}`, { status: 404, body: JSON.stringify(archive.notFound) }],
  ]);
  const requests: string[] = [];
  const unexpected: string[] = [];
  const server = createServer((request, response) => {
    requests.push(request.url ?? "");
    const recorded = responses.get(request.url ?? "");
    if (request.method !== "GET" || request.headers.authorization !== undefined
      || request.headers.accept !== "application/json" || recorded === undefined) {
      unexpected.push(`${request.method} ${request.url}`);
      response.writeHead(500).end();
      return;
    }
    response.writeHead(recorded.status, { "Content-Type": "application/json", Connection: "close" }).end(recorded.body);
  });
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => { server.off("error", reject); resolve(); });
  });
  const address = server.address();
  assert.ok(address !== null && typeof address !== "string");
  const origin = `http://127.0.0.1:${address.port}`;
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
    assert.deepEqual(unexpected, [], "only declared native HTTP paths may be requested");
  });
  return { transaction, archive, transactionPath, responses, requests, origin };
}

async function receipt_retired_batch_ui_native_committed_position(context: TestContext) {
  const fixture = await recordedProducer(context);
  const { transaction } = fixture;
  const html = renderToStaticMarkup(await ReceiptPage({
    params: Promise.resolve({ receiptId: transaction.hash.slice(2).toUpperCase() }),
  }));
  assert.match(html, /Native transaction position/u);
  assert.match(html, new RegExp(`<td[^>]*>Committed block</td><td[^>]*>${transaction.block_number}</td>`, "u"));
  assert.match(html, new RegExp(`<td[^>]*>Transaction position in block</td><td[^>]*>${transaction.position}</td>`, "u"));
  assert.match(html, /<td[^>]*>Commitment<\/td><td[^>]*>Committed<\/td>/u);
  assert.ok(html.includes(`href="${fixture.origin}/tx/${transaction.hash}"`));
  assert.ok(html.includes("?archive=1"));
  assert.doesNotMatch(html, /Canonical receipt bytes|batch_number|checkpoint|Indexed batch|Frozen historical evidence/u);
  assert.deepEqual(fixture.requests, [fixture.transactionPath]);
  const decoded = await readNativeReceiptTransaction(transaction.hash);
  assert.deepEqual(decoded, {
    hash: transaction.hash, block_number: transaction.block_number, position: transaction.position, status: transaction.status,
  });
}

async function receipt_retired_batch_ui_explicit_historical_archive(context: TestContext) {
  const fixture = await recordedProducer(context);
  for (const receipt of [fixture.archive.kernelReceipt, fixture.archive.evmReceipt]) {
    const html = renderToStaticMarkup(await ReceiptPage({
      params: Promise.resolve({ receiptId: receipt.id.slice(2) }),
      searchParams: Promise.resolve({ archive: "1" }),
    }));
    assert.match(html, /Historical receipt archive/u);
    assert.match(html, /Frozen historical evidence/u);
    assert.match(html, /do not describe a current committed transaction/u);
    assert.match(html, /Canonical receipt bytes are unavailable/u);
    assert.doesNotMatch(html, /Native transaction position|Committed block|Transaction position in block/u);
    if (receipt.origin === "kernel") {
      assert.ok(html.includes(`&quot;batch_number&quot;: &quot;${receipt.provenance.kernel.batch_number}&quot;`));
      assert.ok(html.includes(`&quot;sequence&quot;: &quot;${receipt.provenance.kernel.sequence}&quot;`));
      assert.ok(html.includes(receipt.provenance.kernel.canonical_sha256));
      assert.doesNotMatch(html, /View native transaction/u);
    } else {
      assert.ok(html.includes(`href="${fixture.origin}/tx/${receipt.transaction_hash}"`));
    }
  }
  assert.deepEqual(fixture.requests, [fixture.archive.kernelReceipt, fixture.archive.evmReceipt]
    .map((receipt) => `/api/v2/paxeer-x/archive/receipts/${receipt.id}`));
}

async function receipt_native_position_validation_and_availability(context: TestContext) {
  const fixture = await recordedProducer(context);
  const identifier: string = fixture.transaction.hash;
  const render = async (receiptId = identifier) => renderToStaticMarkup(await ReceiptPage({ params: Promise.resolve({ receiptId }) }));
  const initialRequests = fixture.requests.length;
  assert.match(await render("../other-route"), /native transaction reader is unavailable/u);
  assert.equal(fixture.requests.length, initialRequests, "invalid identifiers must not reach the transport");
  assert.match(await render("0".repeat(64)), /Native transaction not found/u);
  const missingArchive = renderToStaticMarkup(await ReceiptPage({
    params: Promise.resolve({ receiptId: "0".repeat(64) }), searchParams: Promise.resolve({ archive: "1" }),
  }));
  assert.match(missingArchive, /Archived receipt not found/u);

  // Corrupt the recorded transport document to exercise identity and position refusal.
  for (const fields of [
    { hash: `0x${"f".repeat(64)}` },
    { block_number: "20077921" },
    { block_number: -1 },
    { block_number: Number.MAX_SAFE_INTEGER + 1 },
    { position: -1 },
    { position: 1.5 },
    { block_number: null, position: 134 },
    { status: "made-up-status" },
  ]) {
    fixture.responses.set(fixture.transactionPath, { status: 200, body: JSON.stringify({ ...fixture.transaction, ...fields }) });
    await assert.rejects(readNativeReceiptTransaction(identifier), TypeError);
    const html = await render();
    assert.match(html, /native transaction reader is unavailable/u);
    assert.doesNotMatch(html, /Native transaction position/u);
  }
  fixture.responses.set(fixture.transactionPath, { status: 503, body: JSON.stringify(fixture.archive.unavailable) });
  assert.match(await render(), /native transaction reader is unavailable/u);
  fixture.responses.set(fixture.transactionPath, { status: 200, body: "{invalid json" });
  assert.match(await render(), /native transaction reader is unavailable/u);

  fixture.responses.set(fixture.transactionPath, { status: 200, body: JSON.stringify({
    ...fixture.transaction, block_number: null, position: null, status: null,
  }) });
  const pending = await render();
  assert.match(pending, /<td[^>]*>Commitment<\/td><td[^>]*>Pending<\/td>/u);
  assert.match(pending, /<td[^>]*>Committed block<\/td><td[^>]*>Not committed<\/td>/u);
  fixture.responses.set(fixture.transactionPath, { status: 200, body: JSON.stringify({
    ...fixture.transaction, block_number: 0, position: 0, status: "error",
  }) });
  const committedError = await render();
  assert.match(committedError, /<td[^>]*>Committed block<\/td><td[^>]*>0<\/td>/u);
  assert.match(committedError, /<td[^>]*>Transaction position in block<\/td><td[^>]*>0<\/td>/u);
  assert.match(committedError, /<td[^>]*>Execution status<\/td><td[^>]*>error<\/td>/u);
}

test(receipt_retired_batch_ui_native_committed_position);
test(receipt_retired_batch_ui_explicit_historical_archive);
test(receipt_native_position_validation_and_availability);
