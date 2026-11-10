import assert from "node:assert/strict";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import { isDeepStrictEqual } from "node:util";
import { JsonRpcClient } from "../src/rpc.js";
import type { SequencerReceiptKey } from "../src/verifier.js";
import type { WebSearchCurrency } from "../src/web-search.js";

export const localWebFixture = <T>(path: string): T => JSON.parse(readFileSync(new URL(
  `../../test/fixtures/repair-22-33-24/${path}`, import.meta.url,
), "utf8")) as T;
export const sellerFixture = <T>(path: string): T => {
  if (path === "client-exchange.json") return localWebFixture<T>("client-exchange.json");
  if (path === "gateway/metered.json") return localWebFixture<T>("gateway-metered.json");
  return JSON.parse(readFileSync(new URL(
    `../../../../../../services/interop/crates/x-websearch/tests/fixtures/${path}`, import.meta.url,
  ), "utf8")) as T;
};
export const unhex = (value: string): Uint8Array => new Uint8Array(Buffer.from(value.replace(/^0x/u, ""), "hex"));
export interface SellerStep {
  readonly request: { readonly method: string; readonly target: string; readonly headers: Record<string, unknown> };
  readonly response: { readonly status: number; readonly headers: Record<string, unknown>; readonly body?: unknown; readonly bodyBase64?: string };
}
export interface RecordedRpc {
  readonly method: string;
  readonly params: readonly unknown[];
  readonly result?: unknown;
  readonly error?: unknown;
}
function immutableRecording<T>(value: T): T {
  if (value !== null && typeof value === "object") {
    for (const child of Object.values(value)) immutableRecording(child);
    Object.freeze(value);
  }
  return value;
}

export const sellerExchange = immutableRecording(sellerFixture<{ readonly exchange: readonly SellerStep[] }>("client-exchange.json").exchange);
export const buyer = sellerFixture<{
  readonly sequencerId: string; readonly sequencerPublicKey: string;
  readonly firstBatchNumber: number; readonly lastBatchNumber: number;
  readonly grants: Readonly<Record<WebSearchCurrency, string>>;
}>("gateway/buyer.json");
export const webReceiptKeys: readonly SequencerReceiptKey[] = [{
  keyId: unhex(buyer.sequencerId), publicKey: unhex(buyer.sequencerPublicKey),
  first_height: BigInt(buyer.firstBatchNumber), last_height: BigInt(buyer.lastBatchNumber),
}];
export const webConfig = sellerFixture<{
  readonly assets: Readonly<Record<WebSearchCurrency, { readonly asset_id: string; readonly price: string }>>;
}>("config/valid.json");
export const exactRecording = immutableRecording(sellerFixture<{
  readonly payments: Readonly<Record<WebSearchCurrency, { readonly rawTransaction: string; readonly idempotencyKey: string; readonly txHash: string }>>;
  readonly endpoints: { readonly gateway: { readonly "*": readonly RecordedRpc[] } };
}>("gateway/exact.json"));
export const meteredRules = immutableRecording(sellerFixture<{
  readonly endpoints: { readonly gateway: { readonly "*": readonly RecordedRpc[] } };
}>("gateway/metered.json").endpoints.gateway["*"]);

export const ledgerRecording = JSON.parse(readFileSync(new URL("../../test/recorded-web-ledger.json", import.meta.url), "utf8")) as {
  readonly producer: string;
  readonly sources: Readonly<Record<string, string>>;
  readonly calls: readonly RecordedRpc[];
};

// The read-only accountId capture supersedes the historical PAX lookup, which
// answered with a main account even though accountId derives an asset account.
// Raw lookup responses preserve the producer's exact recorded submitted bytes.
export const webRpcRules: readonly RecordedRpc[] = [
  ...ledgerRecording.calls,
  ...meteredRules,
  ...exactRecording.endpoints.gateway["*"],
  ...meteredRules.filter(rule => rule.method === "eth_sendRawTransaction").map(rule => ({
    method: "eth_getRawTransactionByHash", params: [rule.result], result: rule.params[0],
  })),
  ...Object.values(exactRecording.payments).map(payment => ({
    method: "eth_getRawTransactionByHash", params: [payment.txHash], result: payment.rawTransaction,
  })),
];

/** Replay only committed RPC messages; do not invent an answer for an unrecorded call. */
export async function recordedWebRpc() {
  const calls: { readonly path: string | undefined; readonly method: string; readonly params: readonly unknown[] }[] = [];
  const failures: unknown[] = [];
  const server = createServer(async (request, response) => {
    try {
      if (request.url !== "/") { response.writeHead(404); response.end(); return; }
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const message = JSON.parse(Buffer.concat(chunks).toString()) as { readonly id: string; readonly method: string; readonly params: readonly unknown[] };
      calls.push({ path: request.url, method: message.method, params: message.params });
      const rule = webRpcRules.find(entry => entry.method === message.method && isDeepStrictEqual(entry.params, message.params));
      assert(rule, `Unrecorded RPC ${JSON.stringify(message)}`);
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: "2.0", id: message.id,
        ...(rule.error === undefined ? { result: rule.result } : { error: rule.error }) }));
    } catch (error) {
      failures.push(error);
      response.writeHead(500); response.end(String(error));
    }
  });
  server.listen(0, "127.0.0.1"); await once(server, "listening");
  const address = server.address(); assert(address !== null && typeof address !== "string");
  const endpoint = `http://127.0.0.1:${address.port}`;
  return { endpoint, client: new JsonRpcClient(endpoint), calls,
    async close(): Promise<void> {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
      assert.deepEqual(failures, []);
    } };
}

/** Replay a seller recording, preserving its receipt, offer, request headers and body. */
export async function recordedSeller(challenge: SellerStep, paid: SellerStep) {
  const sent: { readonly target: string | undefined; readonly headers: import("node:http").IncomingHttpHeaders; readonly payment: unknown }[] = [];
  const failures: unknown[] = [];
  const server = createServer((request, response) => {
    try {
      const signature = request.headers["payment-signature"];
      const step = signature === undefined ? challenge : paid;
      assert.equal(request.method, step.request.method);
      assert.equal(request.url, step.request.target);
      assert.equal(request.headers["layerx-payer-did"], undefined);
      assert.equal(request.headers["layerx-payer-address"], step.request.headers["LAYERX-PAYER-ADDRESS"]);
      if (signature !== undefined) {
        assert.equal(typeof signature, "string");
        const payment = JSON.parse(Buffer.from(String(signature), "base64").toString("utf8")) as unknown;
        assert.deepEqual(payment, step.request.headers["PAYMENT-SIGNATURE"]);
        sent.push({ target: request.url, headers: request.headers, payment });
      }
      const headers: Record<string, string> = { "content-type": "application/json" };
      for (const [name, value] of Object.entries(step.response.headers)) headers[name] = typeof value === "string" ? value : Buffer.from(JSON.stringify(value)).toString("base64");
      response.writeHead(step.response.status, headers);
      response.end(step.response.bodyBase64 === undefined ? JSON.stringify(step.response.body) : Buffer.from(step.response.bodyBase64, "base64"));
    } catch (error) {
      failures.push(error); response.writeHead(500); response.end(String(error));
    }
  });
  server.listen(0, "127.0.0.1"); await once(server, "listening");
  const address = server.address(); assert(address !== null && typeof address !== "string");
  return { endpoint: `http://127.0.0.1:${address.port}`, sent,
    async close(): Promise<void> {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
      assert.deepEqual(failures, []);
    } };
}
