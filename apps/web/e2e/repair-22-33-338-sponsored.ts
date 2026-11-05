import assert from "node:assert/strict";
import { existsSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { createServer, type RequestListener } from "node:http";
import { createServer as createSecureServer } from "node:https";
import { registerHooks } from "node:module";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { DatabaseSync } from "node:sqlite";
import { getCACertificates, setDefaultCACertificates } from "node:tls";
import { fileURLToPath } from "node:url";
import ts from "typescript";
import type { SidioraGasQuote } from "../src/api/gas-station.ts";
import type { GatewayReceipt } from "../src/api/gateway.ts";
import type { SponsoredSubmissionEvidence } from "../../../sdk/wallet/src/modules/gas-station.ts";
import type { WireDigestConstruction, WireSponsoredBatch } from "../../../sdk/wallet/src/types.ts";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js") ? new URL(candidate.href.slice(0, -3) + suffix) : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    if (specifier === "server-only") return nextResolve("next/dist/compiled/server-only/empty.js", context);
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
const { PaxeerProvider, constructionDigest } = await import("../../../sdk/wallet/src/provider.ts");
const { committedReceipt, committedRecording } = await import("./repair-22-33-338-receipts.ts");
const { nodeVector, receiptWireKeys } = await import("../../../sdk/wallet/test/support/receipt-recordings.ts");
const { privateKeyToAccount } = await import("viem/accounts");
const account = privateKeyToAccount(`0x${"0b".repeat(32)}`);
const sponsor = privateKeyToAccount(`0x${"0c".repeat(32)}`);
const HASH = committedReceipt.txHash;
const SECOND_RECEIPT = nodeVector("V1-pax-transfer").receiptJson;
const REVERTED_RECEIPT = nodeVector("V2-layerxledger-revert").receiptJson;
const BLOCK = "0x20000000000001";
const LOG = {
  address: `0x${"44".repeat(20)}`, topics: [`0x${"66".repeat(32)}`], data: "0x1234",
  blockNumber: BLOCK, transactionHash: HASH, logIndex: "0x2",
};

class SubmissionStorage implements Storage {
  readonly #database: DatabaseSync;

  constructor(database: DatabaseSync) {
    this.#database = database;
    database.exec("CREATE TABLE IF NOT EXISTS storage (key TEXT PRIMARY KEY, value TEXT NOT NULL)");
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
    chainId: 125n, account: account.address.toLowerCase(), nonce: 0n,
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
  storage: Storage;
  restart(): Promise<void>;
  readonly nodeMethods: string[];
  readonly hostedPaths: string[];
  readonly batches: WireSponsoredBatch[];
  hash: string;
  nonce: bigint;
  status: SponsoredSubmissionEvidence["status"];
  receiptStatus: "0x1" | "0x0" | null;
  receiptHttpStatus: number;
  submitHttpStatus: number;
}

async function withSubmission(run: (session: ReceiptSession) => Promise<void>, reverted = false): Promise<void> {
  const quoted = await signedQuote();
  const directory = mkdtempSync(join(tmpdir(), "wallet-sponsored-receipt-"));
  let database = new DatabaseSync(join(directory, "submission.sqlite"));
  let provider: InstanceType<typeof PaxeerProvider> | undefined;
  const failures: unknown[] = [];
  const session: ReceiptSession = {
    quoted, paths: [], receiptHashes: [], storage: new SubmissionStorage(database), batches: [], hash: reverted ? REVERTED_RECEIPT.txHash : HASH, nonce: 0n,
    nodeMethods: [], hostedPaths: [],
    async restart() {
      assert.ok(provider);
      provider.disconnect();
      database.close();
      database = new DatabaseSync(join(directory, "submission.sqlite"));
      session.storage = new SubmissionStorage(database);
      provider = await openProvider();
      Object.assign(window, { ethereum: provider, localStorage: session.storage });
    },
    status: "pending", receiptStatus: null, receiptHttpStatus: 200, submitHttpStatus: 200,
  };
  const handler = (owner: "hosted" | "node"): RequestListener => async (request, response) => {
    try {
      let text = "";
      for await (const chunk of request) text += String(chunk);
      session.paths.push(request.url ?? "");
      if (owner === "hosted") {
        assert.ok(["/v1/wallet/me", "/v1/wallet/sign-digest", "/v1/wallet/sponsored/submit", "/v1/wallet/sponsored/status"].includes(request.url ?? ""));
        assert.equal(request.method, request.url === "/v1/wallet/me" ? "GET" : "POST");
        assert.equal(request.headers.authorization, "Bearer sponsored-receipt-test");
        session.hostedPaths.push(request.url!);
      } else {
        assert.equal(request.url, "/");
        assert.equal(request.method, "POST");
        assert.equal(request.headers.authorization, undefined);
      }
      response.setHeader("content-type", "application/json");
      const body = text === "" ? null : JSON.parse(text) as Record<string, unknown>;
      if (request.url === "/v1/wallet/me") {
        response.end(JSON.stringify({ wallet: { address: quoted.batch.account, chain_id: 125 }, chain: { id: 125 } }));
        return;
      }
      if (request.url === "/v1/wallet/sign-digest") {
        const construction = body!.construction as WireDigestConstruction;
        response.end(JSON.stringify({ address: account.address, signature: await account.sign({ hash: constructionDigest(construction) }) }));
        return;
      }
      if (request.url === "/v1/wallet/sponsored/submit" || request.url === "/v1/wallet/sponsored/status") {
        if (request.url.endsWith("/submit")) {
          response.statusCode = session.submitHttpStatus;
          session.batches.push(body!.construction as WireSponsoredBatch);
          session.hash = session.batches.length === 1 ? (reverted ? REVERTED_RECEIPT.txHash : HASH) : SECOND_RECEIPT.txHash;
        }
        const status = request.url.endsWith("/submit") ? "pending" : session.status;
        const station = status === "pending" ? null : {
          state: "completed", deadline: quoted.batch.quote.deadline.toString(),
          submission: { sponsorNonce: "0", transactionHash: session.hash }, replacement: null,
          completion: {
            outcome: status === "confirmed" ? "included" : status, transactionHash: session.hash,
            blockNumber: BigInt(BLOCK).toString(), sidCollected: "2000001", paxSpent: "21000",
          },
        };
        response.end(JSON.stringify({ status, tx_hash: session.hash, station }));
        return;
      }
      assert.equal(request.url, "/");
      const rpc = body as { id: string | number; method: string; params: unknown[] };
      if (owner === "node") {
        assert.ok(["eth_getCode", "eth_call", "eth_getTransactionCount", "eth_estimateGas", "eth_gasPrice", "pax_sequencerReceipt", "px_getReceiptKeys", "eth_getRawTransactionByHash", "eth_getTransactionReceipt", "px_resolveAccount"].includes(rpc.method), `Node does not own ${rpc.method}`);
        session.nodeMethods.push(rpc.method);
      } else {
        throw new Error(`Hosted wallet does not own JSON-RPC method ${rpc.method}`);
      }
      let result: unknown;
      const receipt = session.hash === HASH ? committedReceipt : session.hash === REVERTED_RECEIPT.txHash ? REVERTED_RECEIPT : SECOND_RECEIPT;
      switch (rpc.method) {
        case "eth_getCode": result = `0xef0100${quoted.paymaster.slice(2)}`; break;
        case "eth_call": result = `0x${session.nonce.toString(16).padStart(64, "0")}`; break;
        case "eth_getTransactionCount": result = `0x${session.nonce.toString(16)}`; break;
        case "eth_estimateGas": result = "0x1"; break;
        case "eth_gasPrice": result = "0xde0b6b3a7640000"; break;
        case "px_resolveAccount":
          assert.deepEqual(rpc.params, [quoted.batch.account]);
          result = { bound: true, evm_address: quoted.batch.account, layerx_did: `did:layerx:${"55".repeat(32)}` };
          break;
        case "pax_sequencerReceipt":
          assert.deepEqual(rpc.params, [session.hash]);
          result = session.receiptStatus === null && session.status === "pending"
            ? null
            : receipt;
          break;
        case "px_getReceiptKeys":
          assert.deepEqual(rpc.params, []);
          result = session.hash === HASH ? { keys: [committedRecording.pinnedKey] } : receiptWireKeys(receipt);
          break;
        case "eth_getRawTransactionByHash":
          assert.deepEqual(rpc.params, [session.hash]);
          result = session.hash === HASH ? committedRecording.rawTransaction
            : nodeVector(session.hash === REVERTED_RECEIPT.txHash ? "V2-layerxledger-revert" : "V1-pax-transfer").rawTransaction;
          break;
        case "eth_getTransactionReceipt":
          session.receiptHashes.push(rpc.params);
          assert.deepEqual(rpc.params, [session.hash]);
          response.statusCode = session.receiptHttpStatus;
          result = session.receiptStatus === null ? null : {
            transactionHash: session.hash, status: session.receiptStatus, blockNumber: BLOCK, logs: [{ ...LOG, transactionHash: session.hash }],
          };
          break;
        default: throw new Error(`Unexpected RPC method ${rpc.method}`);
      }
      response.end(JSON.stringify({ jsonrpc: "2.0", id: rpc.id, result }));
    } catch (error) {
      failures.push(error);
      response.writeHead(500);
      response.end(JSON.stringify({ error: String(error) }));
    }
  };
  const hostedServer = createServer(handler("hosted"));
  const nodeServer = createServer(handler("node"));
  let hostedOrigin: string;
  let nodeOrigin: string;
  async function openProvider() {
    const next = new PaxeerProvider({ gatewayUrl: hostedOrigin, rpcUrl: nodeOrigin, chainId: 125, token: () => "sponsored-receipt-test" });
    await next.request({ method: "eth_requestAccounts" });
    return next;
  }
  const previousWindow = Object.getOwnPropertyDescriptor(globalThis, "window");
  const previousGateway = process.env.LAYERX_GATEWAY_URL;
  try {
    for (const server of [hostedServer, nodeServer]) {
      await new Promise<void>((resolve, reject) => {
        server.once("error", reject);
        server.listen(0, "127.0.0.1", resolve);
      });
    }
    const hostedAddress = hostedServer.address();
    const nodeAddress = nodeServer.address();
    assert.ok(hostedAddress !== null && typeof hostedAddress !== "string");
    assert.ok(nodeAddress !== null && typeof nodeAddress !== "string");
    hostedOrigin = `http://127.0.0.1:${hostedAddress.port}`;
    nodeOrigin = `http://127.0.0.1:${nodeAddress.port}`;
    assert.notEqual(hostedOrigin, nodeOrigin);
    process.env.LAYERX_GATEWAY_URL = nodeOrigin;
    provider = await openProvider();
    Object.defineProperty(globalThis, "window", {
      configurable: true, value: { ethereum: provider, localStorage: session.storage },
    });
    await run(session);
    assert.deepEqual(failures, []);
    assert.ok(session.hostedPaths.includes("/v1/wallet/me"));
  } finally {
    if (previousWindow === undefined) delete (globalThis as { window?: unknown }).window;
    else Object.defineProperty(globalThis, "window", previousWindow);
    if (previousGateway === undefined) delete process.env.LAYERX_GATEWAY_URL;
    else process.env.LAYERX_GATEWAY_URL = previousGateway;
    provider?.disconnect();
    await Promise.all([hostedServer, nodeServer].map(server => new Promise<void>((resolve, reject) => {
      server.close(error => error === undefined ? resolve() : reject(error));
      server.closeAllConnections();
    })));
    database.close();
    rmSync(directory, { recursive: true });
  }
}

function expectedReceipt(status: "0x1" | "0x0", hash: string = HASH): GatewayReceipt {
  return {
    transactionHash: hash, blockNumber: BigInt(BLOCK), status: status === "0x1" ? "success" : "reverted",
    logs: [{ ...LOG, transactionHash: hash, blockNumber: BigInt(BLOCK), logIndex: 2n }],
  };
}

async function withSponsoredSends(run: (session: ReceiptSession, quotes: Record<string, unknown>[]) => Promise<void>): Promise<void> {
  await withSubmission(async session => {
    const quotes: Record<string, unknown>[] = [];
    const key = readFileSync(new URL("./fixtures/localhost-key.pem", import.meta.url), "utf8");
    const cert = readFileSync(new URL("./fixtures/localhost-cert.pem", import.meta.url), "utf8");
    const server = createSecureServer({ key, cert }, async (request, response) => {
      try {
        assert.equal(request.url, "/quote");
        let text = "";
        for await (const chunk of request) text += String(chunk);
        const body = JSON.parse(text) as Record<string, unknown>;
        quotes.push(body);
        assert.equal(body.account, session.quoted.batch.account);
        assert.equal(body.nonce, session.nonce.toString());
        assert.equal(body.did, `did:layerx:${"55".repeat(32)}`);
        assert.equal(body.gasCost, session.quoted.batch.quote.gasCost.toString());
        const quote = { ...session.quoted.batch.quote, quoteNonce: 7n + BigInt(quotes.length) };
        const digest = sdk.gasQuoteDigest(session.quoted.batch.chainId, session.quoted.batch.account, quote);
        assert.ok(digest.ok);
        const relayerSignature = await sponsor.sign({ hash: digest.value as `0x${string}` });
        response.writeHead(200, { "content-type": "application/json" });
        response.end(JSON.stringify({ quote, relayerSignature, feeDenom: "sidiora" },
          (_key, value: unknown) => typeof value === "bigint" ? value.toString() : value));
      } catch (error) {
        response.writeHead(500);
        response.end(JSON.stringify({ error: String(error) }));
      }
    });
    const environment = {
      PAXEER_GAS_STATION_QUOTE_URL: "",
      PAXEER_GAS_STATION_CHAIN_ID: session.quoted.batch.chainId.toString(),
      PAXEER_GAS_STATION_SPONSOR: session.quoted.batch.quote.sponsor,
      PAXEER_GAS_STATION_PAYMASTER: session.quoted.paymaster,
    };
    const previous = Object.fromEntries(Object.keys(environment).map(name => [name, process.env[name]]));
    const previousCertificates = getCACertificates("default");
    const previousPreference = wallet.walletFeePreference();
    try {
      await new Promise<void>((resolve, reject) => {
        server.once("error", reject);
        server.listen(0, "127.0.0.1", resolve);
      });
      const address = server.address();
      assert.ok(address !== null && typeof address !== "string");
      environment.PAXEER_GAS_STATION_QUOTE_URL = `https://127.0.0.1:${address.port}/quote`;
      Object.assign(process.env, environment);
      setDefaultCACertificates([...previousCertificates, cert]);
      Object.assign(window, { sessionStorage: session.storage, confirm: () => true });
      assert.equal(wallet.chooseWalletFee("sidiora", "2.100000"), true);
      await run(session, quotes);
    } finally {
      wallet.chooseWalletFee(previousPreference.currency, previousPreference.maximum === undefined ? "" : wallet.sidioraAmount(previousPreference.maximum));
      for (const [name, value] of Object.entries(previous)) {
        if (value === undefined) delete process.env[name];
        else process.env[name] = value;
      }
      setDefaultCACertificates(previousCertificates);
      await new Promise<void>((resolve, reject) => {
        server.close(error => error === undefined ? resolve() : reject(error));
        server.closeAllConnections();
      });
    }
  });
}


export { wallet, HASH, SECOND_RECEIPT, expectedReceipt, withSubmission, withSponsoredSends };
