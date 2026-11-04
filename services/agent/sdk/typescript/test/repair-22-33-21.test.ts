import assert from "node:assert/strict";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer, type Server } from "node:http";
import { test } from "node:test";
import { isDeepStrictEqual } from "node:util";
import { PlatformSdkError } from "../src/production.js";
import {
  ResumableStream, STREAM_REPLAY_WINDOW, STREAM_EVENT_ID_MAX_LENGTH, streamCursor,
  type StreamEvent, type StreamPage, type StreamPageSource,
} from "../src/stream.js";
import { supportedProgramGuestAbi, type SequencerReceiptKey } from "../src/verifier.js";
import { supportsProgramGuestAbi } from "../src/generated/receipt.js";
import {
  decodeNativeProgramCallV1, encodeNativeProgramCallV1, type NativeProgramCallV1,
} from "../src/native-program-call.js";
import { JsonRpcClient } from "../src/rpc.js";
import {
  WebSearchClient, WebSearchError, verifyWebSearchPayment,
  type WebSearchClientOptions, type WebSearchExactPayment, type WebSearchOffer,
} from "../src/web-search.js";

const root = new URL("../../../../../../", import.meta.url);
const source = (path: string): string => readFileSync(new URL(path, root), "utf8");
const fixture = <T>(path: string): T => JSON.parse(source(path)) as T;
const unhex = (value: string): Uint8Array => new Uint8Array(Buffer.from(value.replace(/^0x/u, ""), "hex"));

function eventAt(index: number, eventId = `event-${index}`): StreamEvent<number> {
  return { eventId, previousCursor: streamCursor(`cursor-${index - 1}`), cursor: streamCursor(`cursor-${index}`), value: index };
}

function pageOf(events: readonly StreamEvent<number>[]): StreamPage<number> {
  assert(events.length > 0);
  return { requestedCursor: events[0]!.previousCursor, events, nextCursor: events.at(-1)!.cursor };
}

async function stream_memory_bounds_replay_and_preserves_continuity(): Promise<void> {
  assert.equal(STREAM_REPLAY_WINDOW, 1024);
  assert.equal(STREAM_EVENT_ID_MAX_LENGTH, 512);
  const total = STREAM_REPLAY_WINDOW * 40;
  const stream = new ResumableStream<number>(streamCursor("cursor-0"));
  let pages = 0;
  const source: StreamPageSource<number> = async cursor => {
    const start = Number(cursor.slice("cursor-".length)) + 1;
    assert(start <= total);
    pages++;
    return pageOf(Array.from({ length: Math.min(97, total - start + 1) }, (_, i) => eventAt(start + i)));
  };
  const iterator = stream.events(source);
  for (let index = 1; index <= total; index++) {
    const next = await iterator.next();
    assert.equal(next.done, false);
    assert.deepEqual(next.value, eventAt(index));
    assert.equal(stream.cursor, next.value.cursor);
    assert.equal(stream.retainedEventCount, Math.min(index, STREAM_REPLAY_WINDOW));
  }
  await iterator.return(undefined);
  assert(pages > 400);
  const cursor = stream.cursor;
  const retained = stream.retainedEventCount;
  const next = eventAt(total + 1);
  for (const duplicate of [total, total - STREAM_REPLAY_WINDOW + 1]) {
    assert.throws(() => stream.accept(pageOf([{ ...next, eventId: `event-${duplicate}` }])), PlatformSdkError);
  }
  const malformed: readonly StreamPage<number>[] = [
    { ...pageOf([next]), requestedCursor: streamCursor("stale") },
    { ...pageOf([next]), events: [{ ...next, previousCursor: streamCursor("gap") }] },
    { ...pageOf([next]), events: [{ ...next, cursor }] },
    { ...pageOf([next]), nextCursor: streamCursor("wrong-next") },
    pageOf([{ ...next, eventId: "" }]),
    pageOf([{ ...next, eventId: "x".repeat(STREAM_EVENT_ID_MAX_LENGTH + 1) }]),
    pageOf([next, { ...eventAt(total + 2), eventId: next.eventId }]),
  ];
  for (const page of malformed) {
    assert.throws(() => stream.accept(page), PlatformSdkError);
    assert.equal(stream.cursor, cursor);
    assert.equal(stream.retainedEventCount, retained);
  }
  // Whole-page validation still detects duplicates farther apart than the window.
  const large = Array.from({ length: STREAM_REPLAY_WINDOW * 2 }, (_, i) => eventAt(total + i + 1));
  const repeated = large.map((event, i) => i === large.length - 1 ? { ...event, eventId: large[0]!.eventId } : event);
  assert.throws(() => stream.accept(pageOf(repeated)), PlatformSdkError);
  assert.equal(stream.cursor, cursor);
  assert.equal(stream.retainedEventCount, retained);
  stream.accept(pageOf(large));
  assert.equal(stream.retainedEventCount, STREAM_REPLAY_WINDOW);
  assert.equal(stream.cursor, large.at(-1)!.cursor);
  // IDs outside the documented window may recur only with a continuous new cursor.
  const oldId = { ...eventAt(total + large.length + 1), eventId: "event-1" };
  assert.deepEqual(stream.accept(pageOf([oldId])), [oldId]);
  assert.throws(() => stream.accept(pageOf([{ ...eventAt(total + large.length + 2), eventId: "event-1" }])), PlatformSdkError);
  const saved = stream.cursor;
  const reopened = new ResumableStream<number>(saved);
  assert.equal(reopened.retainedEventCount, 0);
  const continuation = eventAt(total + large.length + 2);
  assert.deepEqual(reopened.accept(pageOf([continuation])), [continuation]);
}

function abi_helper_drift_matches_host_vector_and_call_codec(): void {
  const schema = source("services/agent/schema/agent-api/programs.kvx").split("[type.ProgramGuestAbi]\n")[1]?.split("\n[")[0];
  assert(schema);
  const namesMatch = /^variants = (\[.*\])$/mu.exec(schema);
  const valuesMatch = /^wire_values = (\[.*\])$/mu.exec(schema);
  assert(namesMatch && valuesMatch);
  const names = JSON.parse(namesMatch[1]!) as string[];
  const versions = (JSON.parse(valuesMatch[1]!) as string[]).map(Number);
  assert.deepEqual(versions, [1, 2, 3, 4, 5]);
  const host = source("programs/sdk/rust/src/abi_policy.rs");
  assert.match(schema, /source_policy = "programs\/sdk\/rust\/src\/abi_policy.rs"/u);
  names.forEach((name, i) => assert(host.includes(`pub const ${name}: u16 = ${versions[i]};`)));
  const admission = /const fn supported\(requested: u16\)[\s\S]*?match requested \{\s*([^=]+)=>/u.exec(host);
  assert(admission);
  assert.deepEqual(admission[1]!.trim().split(/\s*\|\s*/u), names);
  for (let version = 0; version <= 65535; version++) {
    const admitted = versions.includes(version);
    assert.equal(supportedProgramGuestAbi(version), admitted, `verifier ABI ${version}`);
    assert.equal(supportsProgramGuestAbi(version), admitted, `generated ABI ${version}`);
  }
  for (const invalid of [-1, 1.5, 2.5, 5.5, 65536, NaN, Infinity, -Infinity]) {
    assert.equal(supportedProgramGuestAbi(invalid), false);
    assert.equal(supportsProgramGuestAbi(invalid), false);
  }
  const recorded = fixture<{ payload_hex: string }>("sdk/platform/conformance/fixtures/native-program-call-v4.json");
  const call = decodeNativeProgramCallV1(unhex(recorded.payload_hex));
  for (const guestAbi of [1, 2, 3, 4, 5] as const) {
    assert(supportedProgramGuestAbi(guestAbi));
    const encoded = encodeNativeProgramCallV1({ ...call, guestAbi });
    assert.deepEqual(decodeNativeProgramCallV1(encoded), { ...call, guestAbi });
  }
  for (const guestAbi of [0, 6, 65535]) {
    const encoded = new Uint8Array(encodeNativeProgramCallV1(call));
    new DataView(encoded.buffer).setUint16(32, guestAbi);
    assert.throws(() => decodeNativeProgramCallV1(encoded), TypeError);
    assert.throws(() => encodeNativeProgramCallV1({ ...call, guestAbi } as NativeProgramCallV1), TypeError);
  }
}

interface RpcEntry {
  readonly method: string;
  readonly params: readonly unknown[];
  readonly result?: unknown;
  readonly error?: unknown;
}
interface GatewayRecording {
  readonly endpoints: { readonly gateway: { readonly "*": readonly RpcEntry[] } };
  readonly payments: { readonly SID: { readonly rawTransaction: string; readonly idempotencyKey: string; readonly txHash: string } };
}
interface SellerStep {
  readonly request: { readonly method: string; readonly target: string; readonly headers: Readonly<Record<string, unknown>> };
  readonly response: { readonly status: number; readonly headers: Readonly<Record<string, unknown>>; readonly body?: unknown; readonly bodyBase64?: string };
}
interface Settlement {
  readonly transaction: string;
  readonly payer: string;
  readonly extensions: { readonly layerx: { receipt: string; receiptDigest: string } };
}
const sellerRoot = "services/interop/crates/x-websearch/tests/fixtures/";
const exchange = fixture<{ exchange: readonly SellerStep[] }>(`${sellerRoot}client-exchange.json`).exchange;
const exact = fixture<GatewayRecording>(`${sellerRoot}gateway/exact.json`);
const ledger = fixture<GatewayRecording>(`${sellerRoot}gateway/metered.json`).endpoints.gateway["*"];
const buyer = fixture<{ sequencerId: string; sequencerPublicKey: string; firstBatchNumber: number; lastBatchNumber: number }>(`${sellerRoot}gateway/buyer.json`);
const assets = fixture<{ assets: { PAX: { asset_id: string }; SID: { asset_id: string; price: string } } }>(`${sellerRoot}config/valid.json`).assets;

function receiptPins(): SequencerReceiptKey[] {
  return [{ keyId: unhex(buyer.sequencerId), publicKey: unhex(buyer.sequencerPublicKey), first_height: BigInt(buyer.firstBatchNumber), last_height: BigInt(buyer.lastBatchNumber) }];
}

function webError(code: string): (error: unknown) => boolean {
  return error => error instanceof WebSearchError && error.code === code;
}

async function listen(server: Server): Promise<string> {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert(address && typeof address !== "string");
  return `http://127.0.0.1:${address.port}`;
}

async function close(server: Server): Promise<void> {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
}

async function web_unused_authority_refuses_configuration_and_requires_exact_bytes(): Promise<void> {
  const challenge = exchange[2]!, paid = exchange[3]!;
  const payment = exact.payments.SID;
  const receiptReply = exact.endpoints.gateway["*"].find(entry => entry.method === "pax_sequencerReceipt" && entry.params[0] === payment.txHash)?.result as { receipt: string };
  assert(receiptReply);
  const receipt = unhex(receiptReply.receipt), raw = unhex(payment.rawTransaction);
  const offer = (paid.request.headers["PAYMENT-SIGNATURE"] as { accepted: WebSearchOffer & { extra: { layerx: { account: string; currency: "SID" } } } }).accepted;
  const typedOffer = { ...offer, account: offer.extra.layerx.account, currency: offer.extra.layerx.currency };
  const settlement = paid.response.headers["PAYMENT-RESPONSE"] as Settlement;
  let requests = 0, paidRequests = 0, preparations = 0, authorityCalls = 0;
  let mode: "valid" | "missing" | "empty" | "forged-payment" | "forged-settlement" = "valid";
  const rpcCalls: RpcEntry[] = [];
  const node = createServer(async (incoming, outgoing) => {
    const chunks: Buffer[] = [];
    for await (const chunk of incoming) chunks.push(Buffer.from(chunk));
    const request = JSON.parse(Buffer.concat(chunks).toString("utf8")) as { id: string; method: string; params: readonly unknown[] };
    const entry = ledger.find(row => row.method === request.method && isDeepStrictEqual(row.params, request.params));
    assert(entry, `unrecorded RPC ${JSON.stringify(request)}`);
    rpcCalls.push(entry);
    outgoing.writeHead(200, { "content-type": "application/json" });
    outgoing.end(JSON.stringify({ jsonrpc: "2.0", id: request.id, ...(entry.error === undefined ? { result: entry.result } : { error: entry.error }) }));
  });
  const seller = createServer((incoming, outgoing) => {
    requests++;
    const header = incoming.headers["payment-signature"];
    const step = header === undefined ? challenge : paid;
    assert.equal(incoming.method, step.request.method);
    assert.equal(incoming.url, step.request.target);
    if (header !== undefined) {
      assert.equal(typeof header, "string");
      assert.deepEqual(JSON.parse(Buffer.from(header as string, "base64").toString("utf8")), paid.request.headers["PAYMENT-SIGNATURE"]);
      paidRequests++;
    }
    const headers: Record<string, string> = { "content-type": "application/json" };
    for (const [key, value] of Object.entries(step.response.headers)) {
      let responseValue = value;
      if (header !== undefined && key.toLowerCase() === "payment-response" && mode === "forged-settlement") {
        const forged = structuredClone(value) as Settlement;
        const bytes = Buffer.from(forged.extensions.layerx.receipt, "base64");
        bytes[bytes.length - 1] = bytes.at(-1)! ^ 1;
        forged.extensions.layerx.receipt = bytes.toString("base64");
        responseValue = forged;
      }
      headers[key] = typeof responseValue === "string" ? responseValue : Buffer.from(JSON.stringify(responseValue)).toString("base64");
    }
    outgoing.writeHead(step.response.status, headers);
    outgoing.end(step.response.bodyBase64 === undefined ? JSON.stringify(step.response.body) : Buffer.from(step.response.bodyBase64, "base64"));
  });
  const nodeEndpoint = await listen(node);
  try {
    const endpoint = await listen(seller);
    try {
      const options: WebSearchClientOptions = {
        endpoint, network: "eip155:125", nativeAsset: assets.PAX.asset_id,
        assets: { SID: { assetId: assets.SID.asset_id, maxAmount: BigInt(assets.SID.price) } },
        ledgerRpc: new JsonRpcClient(nodeEndpoint), receiptKeys: receiptPins(),
        payer: { preferences: [{ currency: "SID", scheme: "exact" }], exact: async (): Promise<WebSearchExactPayment> => {
          preparations++;
          if (mode === "missing") return { receipt };
          const bytes = new Uint8Array(receipt);
          if (mode === "forged-payment") bytes[bytes.length - 1] = bytes.at(-1)! ^ 1;
          return { receipt: bytes, rawTransaction: mode === "empty" ? new Uint8Array() : raw, idempotencyKey: payment.idempotencyKey };
        } },
      };
      const authority: NonNullable<WebSearchClientOptions["authority"]> = async () => { authorityCalls++; throw new Error("obsolete authority called"); };
      for (const obsolete of [{ authority }, { protocolVersion: 2 as const }, { protocolVersion: 3 as const }, { authority, protocolVersion: 3 as const }]) {
        assert.throws(() => new WebSearchClient({ ...options, ...obsolete }), webError("obsolete-trust-configuration"));
      }
      const { receiptKeys: omitted, ...withoutPins } = options;
      assert(omitted);
      assert.throws(() => new WebSearchClient(withoutPins), webError("receipt-keys-required"));
      assert.throws(() => new WebSearchClient({ ...options, receiptKeys: [] }), webError("receipt-keys-required"));
      const pin = receiptPins()[0]!;
      for (const invalid of [
        { ...pin, keyId: new Uint8Array(31) }, { ...pin, publicKey: new Uint8Array(31) },
        { ...pin, first_height: -1n }, { ...pin, last_height: 1n << 64n },
        { ...pin, first_height: pin.last_height + 1n },
      ]) assert.throws(() => new WebSearchClient({ ...options, receiptKeys: [invalid] }), webError("invalid-receipt-key"));
      assert.equal(requests, 0);
      assert.equal(preparations, 0);
      assert.equal(authorityCalls, 0);
      assert.equal(rpcCalls.length, 0);

      const target = new URL(challenge.request.target, endpoint).searchParams.get("url");
      assert(target);
      for (mode of ["missing", "empty"] as const) {
        await assert.rejects(new WebSearchClient(options).fetch(target), webError("exact-raw-transaction-required"));
        assert.equal(paidRequests, 0);
      }
      mode = "forged-payment";
      await assert.rejects(new WebSearchClient(options).fetch(target), webError("receipt-unverified"));
      assert.equal(paidRequests, 0);
      mode = "valid";
      const wrongPin = { ...pin, publicKey: new Uint8Array(32).fill(17) };
      await assert.rejects(new WebSearchClient({ ...options, receiptKeys: [wrongPin] }).fetch(target), webError("receipt-unverified"));
      assert.equal(paidRequests, 0);

      // Mutating caller-owned trust bytes cannot replace pins after admission.
      const mutable = receiptPins();
      const client = new WebSearchClient({ ...options, receiptKeys: mutable });
      mutable[0]!.keyId.fill(0); mutable[0]!.publicKey.fill(0); mutable.length = 0;
      const fetched = await client.fetch(target);
      assert.equal(paidRequests, 1);
      assert.equal(fetched.settlement?.transaction, payment.txHash);
      assert.equal(fetched.settlement?.payer, settlement.payer);
      assert.equal(fetched.settlement?.receiptDigest, settlement.extensions.layerx.receiptDigest);
      assert.equal(fetched.settlement?.currency, "SID");
      assert.equal(fetched.url, target);
      assert(rpcCalls.every(entry => entry.method === "eth_call"));

      mode = "forged-settlement";
      await assert.rejects(client.fetch(target), webError("settlement-receipt-mismatch"));
      assert.equal(paidRequests, 2);
      // Exercise the same mandatory verifier directly with the recorded signed producer bytes.
      const verified = await verifyWebSearchPayment(receipt, raw, typedOffer, receiptPins(), payment.idempotencyKey);
      assert.equal(verified.payment.amount, BigInt(offer.amount));
      const forged = new Uint8Array(receipt);
      forged[forged.length - 1] = forged.at(-1)! ^ 1;
      await assert.rejects(verifyWebSearchPayment(forged, raw, typedOffer, receiptPins(), payment.idempotencyKey), webError("receipt-unverified"));
      await assert.rejects(verifyWebSearchPayment(receipt, raw, typedOffer, [wrongPin], payment.idempotencyKey), webError("receipt-unverified"));
      await assert.rejects(verifyWebSearchPayment(receipt, raw, typedOffer, [{ ...pin, first_height: pin.last_height + 1n, last_height: pin.last_height + 2n }], payment.idempotencyKey), webError("receipt-unverified"));
      assert.equal(requests, 8);
      assert.equal(preparations, 6);
      assert.equal(authorityCalls, 0);
    } finally { await close(seller); }
  } finally { await close(node); }
}

test(stream_memory_bounds_replay_and_preserves_continuity);
test(abi_helper_drift_matches_host_vector_and_call_codec);
test(web_unused_authority_refuses_configuration_and_requires_exact_bytes);
