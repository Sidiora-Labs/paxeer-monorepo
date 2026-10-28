import assert from "node:assert/strict";
import { existsSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { createServer } from "node:http";
import { registerHooks } from "node:module";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { DatabaseSync } from "node:sqlite";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";
import type { SidioraGasQuote } from "../src/api/gas-station.ts";
import type { GatewayReceipt } from "../src/api/gateway.ts";
import type { SponsoredSubmissionEvidence } from "../../../sdk/wallet/src/modules/gas-station.ts";
import type { WireDigestConstruction } from "../../../sdk/wallet/src/types.ts";

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

const wallet = await import("../src/api/wallet.ts");
const sdk = await import("../src/api/sdk.ts");
const { PaxeerProvider, constructionDigest } = await import("@paxeer/wallet/provider");
const { privateKeyToAccount } = await import("viem/accounts");
const account = privateKeyToAccount(`0x${"0b".repeat(32)}`);
const sponsor = privateKeyToAccount(`0x${"0c".repeat(32)}`);
const HASH = `0x${"ab".repeat(32)}`;
const BLOCK = "0x20000000000001";
const LOG = {
  address: `0x${"44".repeat(20)}`, topics: [`0x${"66".repeat(32)}`], data: "0x1234",
  blockNumber: BLOCK, transactionHash: HASH, logIndex: "0x2",
};

class SubmissionStorage implements Storage {
  readonly #database: DatabaseSync;

  constructor(database: DatabaseSync) {
    this.#database = database;
    database.exec("CREATE TABLE storage (key TEXT PRIMARY KEY, value TEXT NOT NULL)");
  }
  get length(): number {
    return Number(this.#database.prepare("SELECT count(*) AS count FROM storage").get()!.count);
  }
  clear(): void { this.#database.exec("DELETE FROM storage"); }
  getItem(key: string): string | null {
    const row = this.#database.prepare("SELECT value FROM storage WHERE key = ?").get(key);
    return row === undefined ? null : String(row.value);
  }
  key(index: number): string | null {
    if (index < 0) return null;
    const row = this.#database.prepare("SELECT key FROM storage ORDER BY key LIMIT 1 OFFSET ?").get(index);
    return row === undefined ? null : String(row.key);
  }
  removeItem(key: string): void { this.#database.prepare("DELETE FROM storage WHERE key = ?").run(key); }
  setItem(key: string, value: string): void {
    this.#database.prepare("INSERT INTO storage VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value").run(key, value);
  }
}

async function signedQuote(): Promise<SidioraGasQuote> {
  const batch = {
    chainId: 1325n, account: account.address.toLowerCase(), nonce: 0n,
    calls: [{ to: `0x${"33".repeat(20)}`, value: 0n, data: "0x1234" }],
    quote: {
      sponsor: sponsor.address.toLowerCase(), token: sdk.SIDIORA_TOKEN, decimals: sdk.SIDIORA_DECIMALS,
      tokenAmount: 2_000_001n, maxTokenAmount: 2_100_000n,
      deadline: BigInt(Math.floor(Date.now() / 1000)) + 300n, quoteNonce: 7n, gasCost: 10n ** 18n,
    },
  };
  const digest = sdk.gasQuoteDigest(batch.chainId, batch.account, batch.quote);
  assert.ok(digest.ok);
  return {
    batch, paymaster: LOG.address,
    relayerSignature: await sponsor.sign({ hash: digest.value as `0x${string}` }),
  };
}

interface ReceiptSession {
  readonly quoted: SidioraGasQuote;
  readonly paths: string[];
  readonly receiptHashes: unknown[][];
  readonly storage: Storage;
  status: SponsoredSubmissionEvidence["status"];
  receiptStatus: "0x1" | "0x0" | null;
  receiptHttpStatus: number;
  submitHttpStatus: number;
}

async function withSubmission(run: (session: ReceiptSession) => Promise<void>): Promise<void> {
  const quoted = await signedQuote();
  const directory = mkdtempSync(join(tmpdir(), "wallet-sponsored-receipt-"));
  const database = new DatabaseSync(join(directory, "submission.sqlite"));
  const session: ReceiptSession = {
    quoted, paths: [], receiptHashes: [], storage: new SubmissionStorage(database),
    status: "pending", receiptStatus: null, receiptHttpStatus: 200, submitHttpStatus: 200,
  };
  const server = createServer(async (request, response) => {
    try {
      let text = "";
      for await (const chunk of request) text += String(chunk);
      session.paths.push(request.url ?? "");
      response.setHeader("content-type", "application/json");
      const body = text === "" ? null : JSON.parse(text) as Record<string, unknown>;
      if (request.url === "/v1/wallet/me") {
        response.end(JSON.stringify({ wallet: { address: quoted.batch.account, chain_id: 1325 }, chain: { id: 1325 } }));
        return;
      }
      if (request.url !== "/rpc") assert.equal(request.headers.authorization, "Bearer sponsored-receipt-test");
      if (request.url === "/v1/wallet/sign-digest") {
        const construction = body!.construction as WireDigestConstruction;
        response.end(JSON.stringify({ address: account.address, signature: await account.sign({ hash: constructionDigest(construction) }) }));
        return;
      }
      if (request.url === "/v1/wallet/sponsored/submit" || request.url === "/v1/wallet/sponsored/status") {
        if (request.url.endsWith("/submit")) response.statusCode = session.submitHttpStatus;
        const status = request.url.endsWith("/submit") ? "pending" : session.status;
        const station = status === "pending" ? null : {
          state: "completed", deadline: quoted.batch.quote.deadline.toString(),
          submission: { sponsorNonce: "0", transactionHash: HASH }, replacement: null,
          completion: {
            outcome: status === "confirmed" ? "included" : status, transactionHash: HASH,
            blockNumber: BigInt(BLOCK).toString(), sidCollected: "2000001", paxSpent: "21000",
          },
        };
        response.end(JSON.stringify({ status, tx_hash: HASH, station }));
        return;
      }
      assert.equal(request.url, "/rpc");
      const rpc = body as { id: string | number; method: string; params: unknown[] };
      let result: unknown;
      switch (rpc.method) {
        case "eth_getCode": result = `0xef0100${quoted.paymaster.slice(2)}`; break;
        case "eth_call": result = `0x${"0".repeat(64)}`; break;
        case "eth_getTransactionCount": result = "0x0"; break;
        case "pax_sequencerReceipt":
          assert.deepEqual(rpc.params, [HASH]);
          result = {
            rung: session.status === "pending" ? "submitted" : "committed", txHash: HASH, nonce: "0x0",
            blockNumber: BLOCK, blockHash: `0x${"77".repeat(32)}`,
            status: session.status === "reverted" ? "0x0" : "0x1",
          };
          break;
        case "eth_getTransactionReceipt":
          session.receiptHashes.push(rpc.params);
          assert.deepEqual(rpc.params, [HASH]);
          response.statusCode = session.receiptHttpStatus;
          result = session.receiptStatus === null ? null : {
            transactionHash: HASH, status: session.receiptStatus, blockNumber: BLOCK, logs: [LOG],
          };
          break;
        default: throw new Error(`Unexpected RPC method ${rpc.method}`);
      }
      response.end(JSON.stringify({ jsonrpc: "2.0", id: rpc.id, result }));
    } catch (error) {
      response.writeHead(500);
      response.end(JSON.stringify({ error: String(error) }));
    }
  });
  const previousWindow = Object.getOwnPropertyDescriptor(globalThis, "window");
  const previousGateway = process.env.LAYERX_GATEWAY_URL;
  try {
    await new Promise<void>((resolve, reject) => {
      server.once("error", reject);
      server.listen(0, "127.0.0.1", resolve);
    });
    const address = server.address();
    assert.ok(address !== null && typeof address !== "string");
    const endpoint = `http://127.0.0.1:${address.port}`;
    process.env.LAYERX_GATEWAY_URL = `${endpoint}/rpc`;
    const provider = new PaxeerProvider({
      gatewayUrl: endpoint, rpcUrl: `${endpoint}/rpc`, chainId: 1325, token: () => "sponsored-receipt-test",
    });
    await provider.request({ method: "eth_requestAccounts" });
    Object.defineProperty(globalThis, "window", {
      configurable: true, value: { ethereum: provider, localStorage: session.storage },
    });
    await run(session);
  } finally {
    if (previousWindow === undefined) delete (globalThis as { window?: unknown }).window;
    else Object.defineProperty(globalThis, "window", previousWindow);
    if (previousGateway === undefined) delete process.env.LAYERX_GATEWAY_URL;
    else process.env.LAYERX_GATEWAY_URL = previousGateway;
    await new Promise<void>((resolve, reject) => {
      server.close(error => error === undefined ? resolve() : reject(error));
      server.closeAllConnections();
    });
    database.close();
    rmSync(directory, { recursive: true });
  }
}

function expectedReceipt(status: "0x1" | "0x0"): GatewayReceipt {
  return {
    transactionHash: HASH, blockNumber: BigInt(BLOCK), status: status === "0x1" ? "success" : "reverted",
    logs: [{ ...LOG, blockNumber: BigInt(BLOCK), logIndex: 2n }],
  };
}

async function sponsored_batch_reports_receipt(): Promise<void> {
  for (const status of ["0x1", "0x0", null] as const) {
    await withSubmission(async session => {
      session.receiptStatus = status;
      const review = wallet.sidioraQuotePresentation(session.quoted);
      assert.ok(review);
      assert.deepEqual(await wallet.sendWalletSponsoredBatch(session.quoted, review.identity), status === null
        ? { outcome: "sent", transactionHash: HASH }
        : { outcome: status === "0x1" ? "sent" : "reverted", transactionHash: HASH, receipt: expectedReceipt(status) });
      assert.deepEqual(session.receiptHashes, [[HASH]]);
      assert.equal(session.paths.filter(path => path === "/v1/wallet/sponsored/submit").length, 1);
      assert.equal(session.paths.filter(path => path === "/v1/wallet/sign-digest").length, 2);
    });
  }
  await withSubmission(async session => {
    session.receiptHttpStatus = 503;
    const review = wallet.sidioraQuotePresentation(session.quoted);
    assert.ok(review);
    const result = await wallet.sendWalletSponsoredBatch(session.quoted, review.identity);
    assert.deepEqual(result, { outcome: "unconfirmed", transactionHash: HASH });
    assert.deepEqual(session.receiptHashes, [[HASH]]);
    assert.equal(session.paths.filter(path => path === "/v1/wallet/sponsored/submit").length, 1);
  });
  await withSubmission(async session => {
    session.submitHttpStatus = 403;
    const review = wallet.sidioraQuotePresentation(session.quoted);
    assert.ok(review);
    assert.deepEqual(await wallet.sendWalletSponsoredBatch(session.quoted, review.identity), { outcome: "rejected" });
    assert.deepEqual(session.receiptHashes, []);
  });
}

async function resumed_submission_reports_receipt(): Promise<void> {
  for (const status of ["confirmed", "reverted", "pending", "cancelled"] as const) {
    await withSubmission(async session => {
      const review = wallet.sidioraQuotePresentation(session.quoted);
      assert.ok(review);
      assert.deepEqual(await wallet.sendWalletSponsoredBatch(session.quoted, review.identity), { outcome: "sent", transactionHash: HASH });
      session.receiptHashes.length = 0;
      session.paths.length = 0;
      session.status = status;
      session.receiptStatus = status === "reverted" ? "0x0" : status === "confirmed" ? "0x1" : null;
      const result = await wallet.resumeWalletSponsoredSubmission(session.quoted.batch.account, session.quoted.batch.calls[0]);
      assert.deepEqual(result, status === "cancelled" ? { outcome: "rejected" }
        : session.receiptStatus === null ? { outcome: "sent", transactionHash: HASH }
          : { outcome: status === "reverted" ? "reverted" : "sent", transactionHash: HASH, receipt: expectedReceipt(session.receiptStatus) });
      assert.deepEqual(session.receiptHashes, status === "cancelled" ? [] : [[HASH]]);
      assert.ok(!session.paths.includes("/v1/wallet/sponsored/submit"));
      assert.ok(!session.paths.includes("/v1/wallet/sign-digest"));
      assert.equal(session.storage.getItem(`layerx:private-sponsored:v1:${session.quoted.batch.account}`) === null, status !== "pending");
    });
  }
  for (const status of ["pending", "confirmed"] as const) {
    await withSubmission(async session => {
      const review = wallet.sidioraQuotePresentation(session.quoted);
      assert.ok(review);
      await wallet.sendWalletSponsoredBatch(session.quoted, review.identity);
      session.receiptHashes.length = 0;
      session.status = status;
      session.receiptStatus = status === "confirmed" ? "0x1" : null;
      const foreignCall = { to: `0x${"99".repeat(20)}`, value: 0n, data: "0x5678" };
      assert.deepEqual(await wallet.resumeWalletSponsoredSubmission(session.quoted.batch.account, foreignCall),
        status === "pending" ? { outcome: "unavailable" } : undefined);
      assert.deepEqual(session.receiptHashes, []);
    });
  }
  await withSubmission(async session => {
    const review = wallet.sidioraQuotePresentation(session.quoted);
    assert.ok(review);
    await wallet.sendWalletSponsoredBatch(session.quoted, review.identity);
    session.status = "confirmed";
    session.receiptStatus = "0x1";
    assert.deepEqual(await wallet.sendWalletSponsoredBatch(session.quoted, review.identity), {
      outcome: "sent", transactionHash: HASH, receipt: expectedReceipt("0x1"),
    });
    assert.equal(session.paths.filter(path => path === "/v1/wallet/sponsored/submit").length, 1);
  });
  for (const status of ["pending", "confirmed", "reverted"] as const) {
    await withSubmission(async session => {
      const review = wallet.sidioraQuotePresentation(session.quoted);
      assert.ok(review);
      await wallet.sendWalletSponsoredBatch(session.quoted, review.identity);
      session.status = status;
      session.receiptStatus = status === "reverted" ? "0x0" : "0x1";
      session.receiptHttpStatus = 503;
      assert.deepEqual(await wallet.resumeWalletSponsoredSubmission(session.quoted.batch.account), { outcome: "unconfirmed", transactionHash: HASH });
      assert.equal(session.paths.filter(path => path === "/v1/wallet/sponsored/submit").length, 1);
    });
  }
  await withSubmission(async session => {
    const review = wallet.sidioraQuotePresentation(session.quoted);
    assert.ok(review);
    await wallet.sendWalletSponsoredBatch(session.quoted, review.identity);
    session.receiptHttpStatus = 503;
    assert.deepEqual(await wallet.sendWalletSponsoredBatch(session.quoted, review.identity), { outcome: "unconfirmed", transactionHash: HASH });
    assert.equal(session.paths.filter(path => path === "/v1/wallet/sponsored/submit").length, 1);
  });
  await withSubmission(async session => {
    const review = wallet.sidioraQuotePresentation(session.quoted);
    assert.ok(review);
    await wallet.sendWalletSponsoredBatch(session.quoted, review.identity);
    session.status = "reverted";
    assert.deepEqual(await wallet.resumeWalletSponsoredSubmission(session.quoted.batch.account), { outcome: "unconfirmed", transactionHash: HASH });
    assert.equal(session.paths.filter(path => path === "/v1/wallet/sponsored/submit").length, 1);
  });
}

test(sponsored_batch_reports_receipt);
test(resumed_submission_reports_receipt);
