import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { ed25519 } from "@noble/curves/ed25519.js";
import { encodeSequencerReceipt } from "../src/sequencer-receipt-codec.js";
import { decodeLedgerTransaction } from "../src/ledger-payment.js";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer, type Server } from "node:http";
import { test } from "node:test";
import { isDeepStrictEqual } from "node:util";
import { WebSocketServer } from "ws";
import { JsonRpcClient } from "../src/rpc.js";
import { PxClient } from "../src/px.js";
import { ResumableStream, streamCursor, type StreamPageSource } from "../src/stream.js";
import { SubscriptionContinuity, subscriptionNotification } from "../src/rpc-subscription.js";
import { admitNativeProgramUpgrade, encodeNativeProgramDeploy, decodeNativeProgramDeploy,
  encodeNativeProgramUpgrade, decodeNativeProgramUpgrade, type NativeProgramGuestAbi } from "../src/program-lifecycle.js";
import { supportsProgramGuestAbi } from "../src/generated/receipt.js";
import { encodeSequencerLogEffect } from "../src/sequencer-log.js";
import { abiSelector, exchangePlaceOrderCall, precompileWriteCall, type AbiArgument } from "../src/exchange.js";
import { cometHeaderHash, cometCommitHash, cometValidatorsHash, unwrapCometEvmTransaction, type CometHeader, type CometProof } from "../src/committed-proof.js";
import { verifySequencerReceipt, verifyCommittedReceipt, verifyCometCommit, SequencerReceiptVerificationError,
  type CometBlockId, type CometValidator, type CommittedReceiptEvidence, type NodeCommittedReceiptEvidence, type SequencerReceiptKey } from "../src/verifier.js";
import { WebSearchClient, WebSearchError, WEB_SEARCH_PAYER_HEADER, webSearchPayerOwner, resolveWebLedgerAccount, bindLedgerPayment, verifyWebSearchPayment,
  decodePayerGrant, type WebSearchCurrency, type WebSearchOffer } from "../src/web-search.js";

const root = new URL("../../../../../../", import.meta.url);
const source = (path: string): string => readFileSync(new URL(path, root), "utf8");
const fixture = <T>(path: string): T => JSON.parse(source(path)) as T;
const hex = (b: Uint8Array): string => Buffer.from(b).toString("hex");
const unhex = (s: string): Uint8Array => new Uint8Array(Buffer.from(s.replace(/^0x/u, ""), "hex"));
const base64 = (s: string): Uint8Array => new Uint8Array(Buffer.from(s, "base64"));
const sellerFixtures = "services/interop/crates/x-websearch/tests/fixtures/";

interface RpcRecording {
  readonly method: string;
  readonly params: readonly unknown[];
  readonly result?: unknown;
  readonly error?: unknown;
}
interface GatewayRecording {
  readonly endpoints: { readonly gateway: { readonly "*": readonly RpcRecording[] } };
}
const metered = fixture<GatewayRecording>(`${sellerFixtures}gateway/metered.json`);
const recordedRules = metered.endpoints.gateway["*"];

async function listen(server: Server): Promise<string> {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address(); assert(address !== null && typeof address !== "string");
  return `http://127.0.0.1:${address.port}`;
}
async function close(server: Server): Promise<void> {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
}
async function recordedRpc(path = "/") {
  const calls: { path: string | undefined; method: string; params: readonly unknown[] }[] = [];
  const server = createServer(async (request, response) => {
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(Buffer.concat(chunks).toString("utf8")) as { id: string; method: string; params: readonly unknown[] };
    calls.push({ path: request.url, method: body.method, params: body.params });
    const entry = recordedRules.find(r => r.method === body.method && isDeepStrictEqual(r.params, body.params));
    if (request.url !== path || entry === undefined) { response.writeHead(404); response.end(); return; }
    response.writeHead(200, { "content-type": "application/json" });
    response.end(JSON.stringify({ jsonrpc: "2.0", id: body.id, ...(entry.error === undefined ? { result: entry.result } : { error: entry.error }) }));
  });
  const endpoint = await listen(server);
  return { server, endpoint: endpoint + (path === "/" ? "" : path), calls };
}

async function rpc_root_preserves_node_and_gateway_routes(): Promise<void> {
  const node = source("rpc/rpcstack.go"), gateway = source("services/platform/hosted/gateway/src/main.rs");
  assert.match(node, /if path == "" \{\s*return r.URL.Path == "\/"/u);
  assert.match(gateway, /request.path == "\/" && request.method == "POST"/u);
  const recorded = recordedRules.find(r => r.method === "eth_getTransactionCount"); assert(recorded);
  for (const path of ["/", "/tenant/alpha", "/tenant/alpha/"]) {
    const replay = await recordedRpc(path);
    try {
      const rpc = new JsonRpcClient(replay.endpoint);
      assert.equal(await rpc.getIdentitySequence(String(recorded.params[0])), BigInt(String(recorded.result)));
      assert.equal(replay.calls[0]?.path, path);
    } finally { await close(replay.server); }
  }
  const requests: { path: string | undefined; body: unknown }[] = [];
  const observer = createServer(async (request) => {
    const chunks: Buffer[] = []; for await (const chunk of request) chunks.push(Buffer.from(chunk));
    requests.push({ path: request.url, body: JSON.parse(Buffer.concat(chunks).toString("utf8")) as unknown });
    request.socket.destroy();
  });
  const endpoint = await listen(observer);
  try {
    await assert.rejects(new JsonRpcClient(endpoint).getNodeInfo());
    await assert.rejects(new PxClient({ endpoint }).getNetwork());
    assert.deepEqual(requests.map(r => r.path), ["/", "/"]);
    assert.deepEqual(requests.map(r => (r.body as { method: string }).method), ["px_getNetwork", "px_getNetwork"]);
  } finally { await close(observer); }
}

async function websocket_root_preserves_handshake_and_reconciliation(): Promise<void> {
  for (const [topic, account] of [["heads", undefined], ["account", "0x000000000000000000000000000000000000101a"]] as const) {
    const server = createServer(), ws = new WebSocketServer({ server, path: "/" });
    const endpoint = await listen(server);
    const received: { path: string | undefined; jsonrpc: string; id: string; method: string; params: unknown }[] = [];
    ws.on("connection", (socket, request) => socket.on("message", data => {
      const message = JSON.parse(data.toString()) as { jsonrpc: string; id: string; method: string; params: unknown };
      received.push({ path: request.url, ...message });
      socket.close();
    }));
    try {
      await assert.rejects(new JsonRpcClient(endpoint).subscribeFrom(topic, 9n, account).next(), /Subscription closed/u);
      assert.deepEqual(received, [{ path: "/", jsonrpc: "2.0", id: "1", method: "eth_subscribe", params: account === undefined ? ["newHeads"] : ["logs", { address: account, topics: [] }] }]);
      const abort = new AbortController(); abort.abort();
      await assert.rejects(new JsonRpcClient(endpoint).subscribe(topic, account, abort.signal).next(), /Subscription cancelled/u);
    } finally {
      for (const client of ws.clients) client.terminate();
      await new Promise<void>(resolve => ws.close(() => resolve()));
      await close(server);
    }
  }
  const continuity = new SubscriptionContinuity("heads", 9n);
  const notification = subscriptionNotification({ jsonrpc: "2.0", method: "eth_subscription",
    params: { subscription: "heads", result: { number: "0xa" } } }, "heads");
  continuity.accept(notification);
  assert.throws(() => continuity.accept({ cursor: 12n, result: { number: "0xc" } }), /reconcile through reads/u);
  const reconciled = new SubscriptionContinuity("heads", 11n);
  reconciled.accept({ cursor: 12n, result: { number: "0xc" } });
  const logs = new SubscriptionContinuity("account", 9n);
  logs.accept({ cursor: 10n, result: { blockNumber: "0xa" } });
  logs.accept({ cursor: 10n, result: { blockNumber: "0xa" } });
  assert.throws(() => logs.accept({ cursor: 9n, result: { blockNumber: "0x9" } }), /regression/u);
}

interface RecordedBlockId { readonly Hash: string; readonly PartSetTotal: number; readonly PartSetHash: string }
interface RecordedHeader {
  readonly VersionBlock: number; readonly VersionApp: number; readonly ChainID: string; readonly Height: number; readonly Time: string;
  readonly LastBlockID: RecordedBlockId; readonly LastCommitHash: string; readonly DataHash: string;
  readonly ValidatorsHash: string; readonly NextValidatorsHash: string; readonly ConsensusHash: string;
  readonly AppHash: string; readonly LastResultsHash: string; readonly EvidenceHash: string; readonly ProposerAddress: string;
}
interface RecordedCommit {
  readonly Height: number; readonly Round: number; readonly BlockID: RecordedBlockId;
  readonly Signatures: readonly { readonly BlockIDFlag: 1 | 2 | 3; readonly ValidatorAddress: string;
    readonly Timestamp: string; readonly Signature: string }[];
}
interface RecordedValidator { readonly PublicKey: readonly number[]; readonly VotingPower: number }
interface RecordedProof { readonly Total: number; readonly Index: number; readonly Aunts: readonly (readonly number[])[] }
interface CommittedVector {
  readonly positive: {
    readonly receiptJson: { readonly logs: readonly RecordedLog[] };
    readonly signedReceipt: string; readonly rawTransaction: string;
    readonly pinnedKey: { readonly keyId: string; readonly publicKey: string; readonly firstHeight: number; readonly lastHeight: number };
    readonly blockHeader: RecordedHeader; readonly commit: RecordedCommit; readonly validatorSet: readonly RecordedValidator[];
    readonly receiptInclusion: { readonly Transaction: string; readonly TransactionProof: RecordedProof; readonly BatchTransaction: string;
      readonly BatchProof: RecordedProof; readonly Result: { readonly Code: number; readonly Data: string; readonly GasWanted: number; readonly GasUsed: number };
      readonly ResultProof: RecordedProof; readonly NextHeader: RecordedHeader; readonly NextCommit: RecordedCommit; readonly NextValidators: readonly RecordedValidator[] };
  };
}
const committedVector = fixture<CommittedVector>("sdk/platform/conformance/fixtures/vector-v4-committed.json").positive;
function time(s: string): CometHeader["time"] {
  const match = /^(.*?)(?:\.(\d{1,9}))?Z$/u.exec(s); assert(match);
  return { seconds: BigInt(Date.parse(`${match[1]}Z`)) / 1000n, nanos: Number((match[2] ?? "").padEnd(9, "0")) };
}
function blockId(b: RecordedBlockId): CometBlockId { return { hash: base64(b.Hash), partSetHeader: { total: b.PartSetTotal, hash: base64(b.PartSetHash) } }; }
function header(h: RecordedHeader): CometHeader {
  return { versionBlock: BigInt(h.VersionBlock), versionApp: BigInt(h.VersionApp), chainId: h.ChainID, height: BigInt(h.Height), time: time(h.Time),
    lastBlockId: blockId(h.LastBlockID), lastCommitHash: base64(h.LastCommitHash), dataHash: base64(h.DataHash), validatorsHash: base64(h.ValidatorsHash),
    nextValidatorsHash: base64(h.NextValidatorsHash), consensusHash: base64(h.ConsensusHash), appHash: base64(h.AppHash), lastResultsHash: base64(h.LastResultsHash),
    evidenceHash: base64(h.EvidenceHash), proposerAddress: base64(h.ProposerAddress) };
}
function commit(c: RecordedCommit): CommittedReceiptEvidence["commit"] {
  return { height: BigInt(c.Height), round: c.Round, blockId: blockId(c.BlockID), signatures: c.Signatures.map(s => ({
    blockIdFlag: s.BlockIDFlag, validatorAddress: base64(s.ValidatorAddress), timestamp: time(s.Timestamp), signature: base64(s.Signature),
  })) };
}
function validators(vs: readonly RecordedValidator[]): readonly CometValidator[] {
  return vs.map(v => { const publicKey = Uint8Array.from(v.PublicKey); return {
    publicKey, address: new Uint8Array(createHash("sha256").update(publicKey).digest().subarray(0, 20)), votingPower: BigInt(v.VotingPower),
  }; });
}
const proof = (p: RecordedProof): CometProof => ({ total: BigInt(p.Total), index: BigInt(p.Index), aunts: p.Aunts.map(a => Uint8Array.from(a)) });
function evidence(): CommittedReceiptEvidence {
  const v = committedVector, p = v.receiptInclusion, next = commit(p.NextCommit);
  return { chainId: v.blockHeader.ChainID, landingHeight: next.height, blockId: next.blockId, commit: next, validators: validators(p.NextValidators),
    inclusion: { executionHeader: header(v.blockHeader), successorHeader: header(p.NextHeader), executionCommit: commit(v.commit), executionValidators: validators(v.validatorSet),
      transaction: base64(p.Transaction), transactionProof: proof(p.TransactionProof), batchTransaction: base64(p.BatchTransaction), batchProof: proof(p.BatchProof),
      result: { code: p.Result.Code, data: base64(p.Result.Data), gasWanted: BigInt(p.Result.GasWanted), gasUsed: BigInt(p.Result.GasUsed) }, resultProof: proof(p.ResultProof) } };
}
function receiptKeys(): readonly SequencerReceiptKey[] {
  const k = committedVector.pinnedKey;
  return [{ keyId: unhex(k.keyId), publicKey: unhex(k.publicKey), first_height: BigInt(k.firstHeight), last_height: BigInt(k.lastHeight) }];
}
async function commit_inclusion_refuses_unrelated_valid_commit(): Promise<void> {
  const v = committedVector, verified = await verifySequencerReceipt(unhex(v.signedReceipt), unhex(v.rawTransaction), receiptKeys()), e = evidence(), p = e.inclusion!;
  assert.deepEqual(cometHeaderHash(p.executionHeader), p.executionCommit.blockId.hash);
  assert.deepEqual(cometHeaderHash(p.successorHeader), e.blockId.hash);
  assert.deepEqual(cometCommitHash(p.executionCommit), p.successorHeader.lastCommitHash);
  assert.deepEqual(cometValidatorsHash(p.executionValidators), p.executionHeader.validatorsHash);
  assert.deepEqual(unwrapCometEvmTransaction(p.transaction), unhex(v.rawTransaction));
  const result = await verifyCommittedReceipt(verified, e);
  assert.equal(result.level, "committed"); assert.equal(result.landingHeight, 11n);
  assert.equal(result.signedPower, result.totalPower);
  const nodeEvidence: NodeCommittedReceiptEvidence = { ...e, header: p.successorHeader,
    inclusion: { ...p, activityIds: [verified.receipt.activityId] } };
  assert.equal((await verifyCommittedReceipt(verified, nodeEvidence)).level, "committed");
  const unrelatedHeader: NodeCommittedReceiptEvidence = { ...nodeEvidence,
    header: { ...nodeEvidence.header, dataHash: new Uint8Array(32) } };
  const unrelatedActivity: NodeCommittedReceiptEvidence = { ...nodeEvidence,
    inclusion: { ...nodeEvidence.inclusion, activityIds: [new Uint8Array(32)] } };
  await assert.rejects(verifyCommittedReceipt(verified, unrelatedHeader));
  await assert.rejects(verifyCommittedReceipt(verified, unrelatedActivity));
  assert.deepEqual(verified.receipt.effects.slice(1).map(effect => effect.body), v.receiptJson.logs.map(logEffect));
  const unrelated: CommittedReceiptEvidence = { chainId: e.chainId, landingHeight: e.landingHeight, blockId: e.blockId, commit: e.commit, validators: e.validators };
  assert((await verifyCometCommit(unrelated)).signedPower > 0n);
  await assert.rejects(verifyCommittedReceipt(verified, unrelated), error => error instanceof SequencerReceiptVerificationError && error.check === "inclusion_mismatch");
  for (const inclusion of [
    { ...p, transaction: p.batchTransaction },
    { ...p, transactionProof: { ...p.transactionProof, index: 0n } },
    { ...p, batchProof: p.transactionProof },
    { ...p, result: { ...p.result, code: 45 } },
    { ...p, result: { ...p.result, gasUsed: p.result.gasUsed + 1n } },
    { ...p, successorHeader: { ...p.successorHeader, lastResultsHash: new Uint8Array(32) } },
    { ...p, executionHeader: { ...p.executionHeader, height: 9n } },
    { ...p, executionValidators: [] },
    { ...p, batchTransaction: p.batchTransaction.map((byte, index) => index === p.batchTransaction.length - 1 ? byte ^ 1 : byte) },
  ]) await assert.rejects(verifyCommittedReceipt(verified, { ...e, inclusion }));
  await assert.rejects(verifyCommittedReceipt({ ...verified }, e), error => error instanceof SequencerReceiptVerificationError && error.check === "inclusion_mismatch");
  const signatures = e.commit.signatures.map(s => ({ ...s, blockIdFlag: 1 as const }));
  await assert.rejects(verifyCommittedReceipt(verified, { ...e, commit: { ...e.commit, signatures } }), error => error instanceof SequencerReceiptVerificationError && error.check === "insufficient_power");
}

function lifecycle_abi_4_5_matches_registration_policy(): void {
  const policy = source("programs/sdk/rust/src/abi_policy.rs");
  assert.match(policy, /ABI_V4_VERSION: u16 = 4/u); assert.match(policy, /ABI_V5_VERSION: u16 = 5/u);
  const wasm = Uint8Array.of(0, 97, 115, 109, 1, 0, 0, 0), newHash = new Uint8Array(createHash("sha256").update(wasm).digest());
  const programId = new Uint8Array(32).fill(1);
  for (const guestAbi of [4, 5] as const) {
    const deploy = { programId, guestAbi, policy: 0 as const, authority: new Uint8Array(32), newHash, wasm };
    assert.equal(supportsProgramGuestAbi(guestAbi), true);
    const deployed = decodeNativeProgramDeploy(encodeNativeProgramDeploy(deploy));
    assert.equal(deployed.guestAbi, guestAbi);
    const upgrade = { programId, guestAbi, oldHash: newHash, newHash, wasm, migrationHook: new Uint8Array(), clearInterface: false };
    const upgraded = decodeNativeProgramUpgrade(encodeNativeProgramUpgrade(upgrade));
    assert.equal(upgraded.guestAbi, guestAbi);
    for (let current = 1; current <= 5; current++) {
      if (current <= guestAbi) admitNativeProgramUpgrade(current, upgraded.guestAbi);
      else assert.throws(() => admitNativeProgramUpgrade(current, upgraded.guestAbi), /ABI transition/u);
    }
    for (const abi of [0, 6, 65535]) {
      assert.throws(() => encodeNativeProgramDeploy({ ...deploy, guestAbi: abi as NativeProgramGuestAbi }));
      const payload = Buffer.from(encodeNativeProgramUpgrade(upgrade)); payload.writeUInt16BE(abi, 32);
      assert.throws(() => decodeNativeProgramUpgrade(payload));
    }
  }
  assert.throws(() => admitNativeProgramUpgrade(5, 4));
  assert.throws(() => admitNativeProgramUpgrade(6, 5));
}

async function stream_undelivered_survives_stop_and_reopen(): Promise<void> {
  const cursors = ["begin", "first", "second", "third"].map(streamCursor);
  const rows = [1, 2, 3].map((value, i) => ({ eventId: `event-${value}`, previousCursor: cursors[i]!, cursor: cursors[i + 1]!, value }));
  const source: StreamPageSource<number> = async cursor => {
    const index = cursors.indexOf(cursor); assert(index >= 0);
    return { requestedCursor: cursor, events: rows.slice(index), nextCursor: cursors[3]! };
  };
  const stream = new ResumableStream<number>(cursors[0]!);
  const iterator = stream.events(source);
  assert.equal((await iterator.next()).value?.value, 1);
  assert.equal(stream.cursor, cursors[1]);
  const saved = stream.cursor; await iterator.return(undefined);
  const reopened = new ResumableStream<number>(saved), remaining = reopened.events(source);
  const delivered = [(await remaining.next()).value?.value, (await remaining.next()).value?.value];
  await remaining.return(undefined);
  assert.deepEqual(delivered, [2, 3]); assert.equal(reopened.cursor, cursors[3]);
  const sameInstance = stream.events(source);
  assert.equal((await sameInstance.next()).value?.value, 2); await sameInstance.return(undefined);
  const invalid = new ResumableStream<number>(cursors[0]!);
  assert.throws(() => invalid.accept({ requestedCursor: cursors[0]!, events: [rows[0]!, rows[2]!], nextCursor: cursors[3]! }));
  assert.equal(invalid.cursor, cursors[0]);
  const abort = new AbortController(), cancelled = new ResumableStream<number>(cursors[0]!), generator = cancelled.events(source, abort.signal);
  assert.equal((await generator.next()).value?.value, 1); abort.abort();
  assert.equal((await generator.next()).done, true); assert.equal(cancelled.cursor, cursors[1]);
}

interface SellerStep {
  readonly request: { readonly method: string; readonly target: string; readonly headers: Readonly<Record<string, unknown>> };
  readonly response: { readonly status: number; readonly headers: Readonly<Record<string, unknown>>; readonly body?: unknown; readonly bodyBase64?: string };
}
interface RecordedLog {
  readonly address: string;
  readonly topics: readonly string[];
  readonly data: string;
}
function logEffect(log: RecordedLog): Uint8Array {
  return encodeSequencerLogEffect(unhex(log.address), log.topics.map(unhex), unhex(log.data));
}
const sellerRecording = fixture<{ readonly exchange: readonly SellerStep[] }>(`${sellerFixtures}client-exchange.json`).exchange;
interface ExactRecording extends GatewayRecording {
  readonly payments: Readonly<Record<WebSearchCurrency, { readonly rawTransaction: string; readonly idempotencyKey: string; readonly txHash: string }>>;
}
const exactRecording = fixture<ExactRecording>(`${sellerFixtures}gateway/exact.json`);
interface BuyerRecording { readonly sequencerId: string; readonly sequencerPublicKey: string; readonly firstBatchNumber: number; readonly lastBatchNumber: number; readonly grants: Readonly<Record<WebSearchCurrency, string>> }
const buyer = fixture<BuyerRecording>(`${sellerFixtures}gateway/buyer.json`);
const webKeys: readonly SequencerReceiptKey[] = [{ keyId: unhex(buyer.sequencerId), publicKey: unhex(buyer.sequencerPublicKey), first_height: BigInt(buyer.firstBatchNumber), last_height: BigInt(buyer.lastBatchNumber) }];
const config = fixture<{ readonly assets: Readonly<Record<WebSearchCurrency, { readonly asset_id: string; readonly price: string }>> }>(`${sellerFixtures}config/valid.json`);

async function web_payer_header_is_validated_evm_address(): Promise<void> {
  assert.equal(WEB_SEARCH_PAYER_HEADER, "LAYERX-PAYER-ADDRESS");
  assert.match(source("services/interop/crates/x-websearch/src/payment.rs"), /PAYER_ADDRESS: &str = "LAYERX-PAYER-ADDRESS"/u);
  assert.match(source("services/interop/crates/x-websearch/src/payment.rs"), /ledger_owner_for_address\(address\)/u);
  assert.equal(webSearchPayerOwner(`0x${"AB".repeat(20)}`), "00".repeat(12) + "ab".repeat(20));
  const recordedAddress = sellerRecording[0]!.request.headers[WEB_SEARCH_PAYER_HEADER]; assert.equal(typeof recordedAddress, "string");
  const observed: (string | string[] | undefined)[] = [];
  const server = createServer((request) => { observed.push(request.headers["layerx-payer-address"]); assert.equal(request.headers["layerx-payer-did"], undefined); request.socket.destroy(); });
  const endpoint = await listen(server);
  const options = { endpoint, network: "eip155:125", nativeAsset: config.assets.PAX.asset_id,
    assets: { SID: { assetId: config.assets.SID.asset_id, maxAmount: 10000n } },
    payer: { address: recordedAddress as string, preferences: [{ currency: "SID" as const, scheme: "metered" as const }] } };
  try {
    await assert.rejects(new WebSearchClient(options).search("paxeer"));
    assert.deepEqual(observed, [recordedAddress]);
    for (const address of ["did:layerx:buyer", "0x1234", "ab".repeat(20), `0x${"g".repeat(40)}`]) assert.throws(() => new WebSearchClient({ ...options, payer: { ...options.payer, address } }), /invalid-payer-address/u);
  } finally { await close(server); }
}

async function web_ledger_account_purchases_recorded_pax_and_sid(): Promise<void> {
  const rpc = await recordedRpc();
  try {
    const ledgerRpc = new JsonRpcClient(rpc.endpoint);
    for (const [currency, paidIndex] of [["SID", 3], ["PAX", 5]] as const) {
      const paid = sellerRecording[paidIndex]!, challenge = sellerRecording[paidIndex - 1]!;
      const offer = ((paid.request.headers["PAYMENT-SIGNATURE"] as { accepted: unknown }).accepted) as { asset: string; payTo: string; extra: { layerx: { account: string } } };
      const owner = offer.extra.layerx.account.slice("agent:did:layerx:".length, "agent:did:layerx:".length + 64);
      assert.equal(await resolveWebLedgerAccount(ledgerRpc, owner, offer.asset, config.assets.PAX.asset_id), offer.payTo);
      const payment = exactRecording.payments[currency];
      const answer = exactRecording.endpoints.gateway["*"].find(r => r.method === "pax_sequencerReceipt" && r.params[0] === payment.txHash)?.result as { receipt: string };
      assert(answer);
      const signed = unhex(answer.receipt), raw = unhex(payment.rawTransaction);
      const rawOffer = (paid.request.headers["PAYMENT-SIGNATURE"] as { accepted: WebSearchOffer & { extra: { layerx: { account: string; currency: WebSearchCurrency } } } }).accepted;
      const verified = await verifyWebSearchPayment(signed, raw, { ...rawOffer, account: rawOffer.extra.layerx.account, currency }, webKeys, payment.idempotencyKey);
      const response = paid.response.headers["PAYMENT-RESPONSE"] as { extensions: { layerx: { logs: readonly RecordedLog[] } } };
      assert.deepEqual(verified.receipt.effects.slice(1).map(effect => effect.body), response.extensions.layerx.logs.map(logEffect));
      assert.equal(hex(verified.payment.to), rawOffer.payTo);
      assert.equal(hex(verified.payment.asset), rawOffer.asset);
      assert.equal(verified.payment.amount, BigInt(rawOffer.amount));
      await assert.rejects(verifyWebSearchPayment(signed, raw, { ...rawOffer, amount: (BigInt(rawOffer.amount) + 1n).toString() }, webKeys, payment.idempotencyKey), WebSearchError);
      const requests: unknown[] = [];
      const server = createServer((request, response) => {
        const signature = request.headers["payment-signature"];
        const step = signature === undefined ? challenge : paid;
        assert.equal(request.url, step.request.target);
        if (typeof signature === "string") requests.push(JSON.parse(Buffer.from(signature, "base64").toString("utf8")) as unknown);
        const headers: Record<string, string> = { "content-type": "application/json" };
        for (const [key, value] of Object.entries(step.response.headers)) headers[key] = typeof value === "string" ? value : Buffer.from(JSON.stringify(value)).toString("base64");
        response.writeHead(step.response.status, headers);
        response.end(step.response.bodyBase64 === undefined ? JSON.stringify(step.response.body) : Buffer.from(step.response.bodyBase64, "base64"));
      });
      const endpoint = await listen(server);
      try {
        const client = new WebSearchClient({ endpoint, network: "eip155:125", nativeAsset: config.assets.PAX.asset_id, ledgerRpc, receiptKeys: webKeys,
          assets: { [currency]: { assetId: config.assets[currency].asset_id, maxAmount: BigInt(config.assets[currency].price) } },
          payer: { preferences: [{ currency, scheme: "exact" }], exact: async () => ({ receipt: signed, rawTransaction: raw, idempotencyKey: payment.idempotencyKey }) } });
        const target = new URL(challenge.request.target, endpoint).searchParams.get("url"); assert(target);
        const result = await client.fetch(target);
        assert.equal(result.settlement?.currency, currency); assert.equal(result.settlement?.transaction, payment.txHash);
        assert.equal(result.settlement?.payer, hex(verified.payment.from));
        assert.deepEqual(requests, [paid.request.headers["PAYMENT-SIGNATURE"]]);
        const expected = paid.response.headers["PAYMENT-RESPONSE"] as { extensions: { layerx: { receipt: string } } };
        assert.deepEqual(base64(expected.extensions.layerx.receipt), signed);
      } finally { await close(server); }
    }
    const calls = rpc.calls.filter(c => c.method === "eth_call"); assert(calls.length >= 4);
    assert(calls.every(c => JSON.stringify(c.params).includes("0x3f050cbc")));
  } finally { await close(rpc.server); }
}

async function web_metered_account_binds_recorded_grant_transaction_and_receipt(): Promise<void> {
  const paid = sellerRecording[1]!, sent = paid.request.headers["PAYMENT-SIGNATURE"] as {
    accepted: { scheme: "metered"; network: string; amount: string; asset: string; payTo: string; maxTimeoutSeconds: number;
      extra: { layerx: { account: string; currency: "SID"; payer: string; purposeHash: string } } };
    payload: { grantId: string; idempotencyKey: string };
  };
  const accepted = sent.accepted, terms = accepted.extra.layerx;
  const offer: WebSearchOffer = { ...accepted, currency: terms.currency, account: terms.account, payer: terms.payer, purposeHash: terms.purposeHash };
  const grant = decodePayerGrant(unhex(buyer.grants.SID));
  assert.equal(grant.grant_id, sent.payload.grantId); assert.equal(grant.from, terms.payer);
  assert.equal(grant.recipient, accepted.payTo); assert.equal(grant.asset, accepted.asset);
  const transaction = recordedRules.find(r => r.method === "eth_sendRawTransaction" && r.result === (paid.response.headers["PAYMENT-RESPONSE"] as { transaction: string }).transaction);
  assert(transaction); const raw = unhex(String(transaction.params[0]));
  bindLedgerPayment(raw, offer, grant.from, sent.payload.idempotencyKey, grant);
  const response = paid.response.headers["PAYMENT-RESPONSE"] as { extensions: { layerx: { receipt: string; logs: readonly RecordedLog[] } } };
  const original = base64(response.extensions.layerx.receipt);
  const recorded = await verifySequencerReceipt(original, raw, webKeys);
  assert.deepEqual(encodeSequencerReceipt(recorded.receipt), original);
  assert.deepEqual(recorded.receipt.effects.slice(1).map(effect => effect.body), response.extensions.layerx.logs.map(logEffect));
  await assert.rejects(verifyWebSearchPayment(original, raw, offer, webKeys, sent.payload.idempotencyKey, grant),
    error => error instanceof WebSearchError && error.code === "receipt-offer-mismatch");
  const calldata = decodeLedgerTransaction(raw).data;
  const context = calldata.subarray(4 + 6 * 32, 4 + 7 * 32);
  assert.notEqual(hex(context), "00".repeat(32));
  const logs = response.extensions.layerx.logs.map(log => ({ ...log,
    data: `0x${log.data.slice(2, -64)}${hex(context)}` }));
  const effects = [recorded.receipt.effects[0]!, ...logs.map((log, i) => ({
    ...recorded.receipt.effects[i + 1]!, body: logEffect(log),
  }))];
  // Reissue the fixture over the actual draw context with the shared canonical codec.
  // The deterministic fixture key is defined by the seller's test_key("sequencer").
  const seed = new Uint8Array(createHash("sha256").update("x-websearch-test/key/sequencer").digest());
  assert.deepEqual(ed25519.getPublicKey(seed), webKeys[0]!.publicKey);
  const issued = { ...recorded.receipt, effects };
  const unsigned = encodeSequencerReceipt(issued, false);
  const digest = new Uint8Array(createHash("sha256").update("LXP/v1/receipt\0").update(unsigned).digest());
  const signed = encodeSequencerReceipt({ ...issued, sequencerSignature: ed25519.sign(digest, seed) });
  const verified = await verifyWebSearchPayment(signed, raw, offer, webKeys, sent.payload.idempotencyKey, grant);
  assert.deepEqual(verified.receipt.effects.slice(1).map(effect => effect.body), logs.map(logEffect));
  assert.deepEqual(verified.receiptDigest, digest);
  assert.equal(hex(verified.receipt.from), "00".repeat(12) + "6813eb9362372eef6200f3b1dbc3f819671cba69");
  assert.equal(hex(verified.receipt.to), "00".repeat(30) + "101a");
  assert.equal(hex(verified.payment.from), grant.from); assert.equal(hex(verified.payment.to), accepted.payTo);
  assert.equal(verified.payment.amount, BigInt(offer.amount));
  await assert.rejects(verifyWebSearchPayment(signed, raw, { ...offer, payer: "11".repeat(32), payTo: "11".repeat(32) }, webKeys, sent.payload.idempotencyKey, grant), WebSearchError);
  assert.throws(() => bindLedgerPayment(raw, { ...offer, payTo: "11".repeat(32) }, grant.from, sent.payload.idempotencyKey, grant), WebSearchError);
  assert.throws(() => bindLedgerPayment(raw, offer, grant.from, "22".repeat(32), grant), WebSearchError);
}

function exchange_account_abi_matches_account_bearing_calldata(): void {
  const entries = fixture<readonly { readonly type: string; readonly name?: string; readonly inputs?: readonly { readonly type: string; readonly name: string }[] }[]>("precompiles/layerxexchange/abi.json");
  const orders = entries.filter(e => e.type === "function" && e.name === "placeOrder");
  const current = orders.find(e => e.inputs?.length === 6), legacy = orders.find(e => e.inputs?.length === 5);
  assert(current?.inputs); assert(legacy?.inputs);
  assert.equal(current.inputs[1]?.name, "accountId");
  const types = current.inputs.map(i => i.type);
  assert.deepEqual(types, ["bytes32", "bytes32", "uint8", "uint256", "uint256", "uint8"]);
  const order = { marketId: `0x${"11".repeat(32)}`, accountId: `0x${"22".repeat(32)}`, side: 2, price: 5n, quantity: 3n, timeInForce: 0 };
  const call = exchangePlaceOrderCall(order), signature = `placeOrder(${types.join(",")})`;
  const values: readonly AbiArgument[] = [order.marketId, order.accountId, BigInt(order.side), order.price, order.quantity, BigInt(order.timeInForce)];
  assert.deepEqual(precompileWriteCall(call.to, signature, { types, payable: false }, values), call);
  assert.equal(call.data.slice(0, 10), abiSelector(signature));
  assert.equal(call.data.length, 10 + 6 * 64); assert.equal(`0x${call.data.slice(74, 138)}`, order.accountId);
  assert.notEqual(abiSelector(`placeOrder(${legacy.inputs.map(i => i.type).join(",")})`), abiSelector(signature));
}

test(rpc_root_preserves_node_and_gateway_routes);
test(websocket_root_preserves_handshake_and_reconciliation);
test(commit_inclusion_refuses_unrelated_valid_commit);
test(lifecycle_abi_4_5_matches_registration_policy);
test(stream_undelivered_survives_stop_and_reopen);
test(web_payer_header_is_validated_evm_address);
test(web_ledger_account_purchases_recorded_pax_and_sid);
test(web_metered_account_binds_recorded_grant_transaction_and_receipt);
test(exchange_account_abi_matches_account_bearing_calldata);
