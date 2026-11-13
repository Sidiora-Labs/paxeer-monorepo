import assert from "node:assert/strict";
import { test } from "node:test";
import { once } from "node:events";
import { createHash } from "node:crypto";
import { mkdtemp, readdir, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createServer, type Server } from "node:http";
import { setImmediate as nextTurn } from "node:timers/promises";
import { WebSocketServer, type WebSocket } from "ws";
import { ed25519 } from "@noble/curves/ed25519.js";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { keccak_256 } from "@noble/hashes/sha3.js";
import { AgentEnvelopeTransport, AgentSessionCredential, LayerXKeyCredential, AGENT_DAEMON_RPC_PATH } from "../src/agent-http.js";
import { ProductionClient, PlatformSdkError, SecretBytes, idempotencyKey, type SdkTelemetryEvent } from "../src/production.js";
import { JsonRpcClient } from "../src/rpc.js";
import { Rung } from "../src/generated/client.js";
import { boundDidCall, bindNonceCall, layerXDidCall, deriveFromSeed, type Eip1193Requester, type BindFees } from "../src/account-derivation.js";
import { TradingWriter, TradingPayloadError } from "../src/perps.js";
import { spotCall } from "../src/spot.js";
import { decodeRawEip1559, recoverEip1559Sender } from "../src/evm-transaction.js";
import { OperationJournal, AiMarketError, FUND, encodeFundRequest, prepareOperation, snapshotIdOf, type SnapshotBinding } from "../src/ai_market.js";
import { LocalMirrorExecutableVerifier, ExecutableMirrorError } from "../src/node-mirror.js";
import { encodeNativeProgramDeploy, decodeNativeProgramDeploy, encodeNativeProgramUpgrade, decodeNativeProgramUpgrade, NATIVE_PROGRAM_REGISTRATION_MAX_BYTES } from "../src/program-lifecycle.js";
import { subscribeRpc, SubscriptionReconciliationRequiredError, SUBSCRIPTION_MAX_BACKLOG } from "../src/rpc-subscription.js";
import { requestGasQuote, requestGasSubmissionStatus, submitSponsoredGasBatch, gasQuoteDigest, sponsoredBatchDigest, eip7702AuthorizationDigest, assembleEip7702Authorization, GAS_RESPONSE_LIMITS, SIDIORA_TOKEN, type GasStationConfig, type GasResult, type SponsoredBatch } from "../src/gas-station.js";

const hex = (bytes: Uint8Array): string => Buffer.from(bytes).toString("hex");
const id = (byte: number): string => byte.toString(16).padStart(2, "0").repeat(32);
const word = (value: bigint): string => value.toString(16).padStart(64, "0");
const account = deriveFromSeed(Buffer.from("000102030405060708090a0b0c0d0e0f", "hex"));
const fees: BindFees = { evmNonce: 0n, maxPriorityFeePerGas: 1n, maxFeePerGas: 9n, gasLimit: 1_000_000n };
async function listen(server: Server): Promise<string> {
  server.listen(0, "127.0.0.1"); await once(server, "listening");
  const address = server.address(); assert(address && typeof address !== "string");
  return `http://127.0.0.1:${address.port}`;
}
async function close(server: Server): Promise<void> {
  server.closeAllConnections(); await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
}
function value<T>(result: GasResult<T>): T { assert(result.ok, result.ok ? undefined : JSON.stringify(result.refusal)); return result.value; }
function signature(digest: string, key: Uint8Array): string {
  const signed = secp256k1.sign(Buffer.from(digest.slice(2), "hex"), key, { prehash: false, format: "recovered" });
  return `0x${hex(signed.slice(1))}${(27 + signed[0]!).toString(16)}`;
}
function addressOf(key: Uint8Array): string { return `0x${hex(keccak_256(secp256k1.getPublicKey(key, false).slice(1)).slice(12))}`; }

/** Loopback node peer with independent committed and pending nonces and real transaction admission. */
async function nodePeer() {
  const accepted: Uint8Array[] = [];
  let wrongHash = false;
  const methods: string[] = [];
  const server = createServer(async (incoming, outgoing) => {
    const chunks: Buffer[] = []; for await (const chunk of incoming) chunks.push(Buffer.from(chunk));
    const rpc = JSON.parse(Buffer.concat(chunks).toString());
    assert.equal(incoming.url, "/"); methods.push(rpc.method);
    let result: unknown;
    if (rpc.method === "eth_call") {
      const data = rpc.params[0].data;
      if (data === boundDidCall(account.evmAddress)) result = `0x${"00".repeat(64)}${account.layerxPublicKey}${"00".repeat(32)}`;
      else if (data === bindNonceCall(account.evmAddress)) result = `0x${word(1n)}`;
      else {
        assert.equal(data, layerXDidCall(account.evmAddress));
        const did = Buffer.from(`did:layerx:${account.layerxPublicKey}`);
        result = `0x${account.layerxPublicKey}${word(64n)}${word(BigInt(did.length))}${hex(Buffer.concat([did, Buffer.alloc(96 - did.length)]))}`;
      }
    } else if (rpc.method === "eth_getTransactionCount") {
      assert.equal(rpc.params[0], account.evmAddress);
      assert(["latest", "pending"].includes(rpc.params[1]));
      result = rpc.params[1] === "latest" ? "0x0" : `0x${accepted.length.toString(16)}`;
    } else if (rpc.method === "eth_sendRawTransaction") {
      const raw = Buffer.from(rpc.params[0].slice(2), "hex");
      const decoded = decodeRawEip1559(raw);
      assert.equal(decoded.nonce, BigInt(accepted.length));
      assert.equal(decoded.chainId, 125n);
      assert.equal(recoverEip1559Sender(raw), account.evmAddress);
      accepted.push(raw);
      result = wrongHash ? `0x${id(255)}` : `0x${hex(keccak_256(raw))}`;
    } else if (rpc.method === "pax_sequencerReceipt") result = null;
    else if (rpc.method === "px_getNetwork") result = { chainId: "125" };
    else throw new Error(`unexpected node method ${rpc.method}`);
    outgoing.writeHead(200, { "content-type": "application/json" });
    outgoing.end(JSON.stringify({ jsonrpc: "2.0", id: rpc.id, result }));
  });
  const endpoint = await listen(server);
  const provider: Eip1193Requester = { async request(request) {
    const response = await fetch(endpoint, { method: "POST", body: JSON.stringify({ jsonrpc: "2.0", id: "1", ...request }) });
    const rpc = await response.json() as { id: string; jsonrpc: string; result: unknown };
    assert.equal(rpc.id, "1"); assert.equal(rpc.jsonrpc, "2.0"); return rpc.result;
  } };
  return { server, endpoint, provider, accepted, methods, wrongHash: () => { wrongHash = true; } };
}

async function daemon_surface_hosted_authentication_and_node_routing(): Promise<void> {
  const session = new AgentSessionCredential("repair", id(1), new SecretBytes(Uint8Array.from(Buffer.alloc(32, 2))), 1n);
  const credential = new LayerXKeyCredential("key_1", new SecretBytes(Uint8Array.from(Buffer.from(`lxp_live_${id(3)}`))));
  let calls = 0;
  const server = createServer(async (incoming, outgoing) => {
    const chunks: Buffer[] = []; for await (const chunk of incoming) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(Buffer.concat(chunks).toString());
    assert.equal(incoming.url, "/v1/agent/rpc");
    assert.equal(incoming.headers.authorization, `LayerX-Key key_1:lxp_live_${id(3)}`);
    assert.deepEqual(body.credential, session.encode()); calls++;
    outgoing.writeHead(200, { "content-type": "application/json" });
    outgoing.end(JSON.stringify({ request_id: body.request_id, value: { sessions: [] }, verification_status: { state: "achieved", rung: "submitted" } }));
  });
  const endpoint = await listen(server);
  try {
    assert.equal(AGENT_DAEMON_RPC_PATH, null);
    assert.throws(() => new AgentEnvelopeTransport({ endpoint, surface: "daemon", session }), PlatformSdkError);
    assert.throws(() => new AgentEnvelopeTransport({ endpoint, session }), PlatformSdkError);
    const transport = new AgentEnvelopeTransport({ endpoint, session, gatewayCredential: credential });
    const response = await transport.call<{}, { value: { sessions: unknown[] } }>({ plane: "agent", operation: "session.list", request: {} });
    assert.deepEqual(response.value, { sessions: [] }); assert.equal(calls, 1);
    const node = await nodePeer();
    try { assert.deepEqual(await new JsonRpcClient(node.endpoint).getNodeInfo(), { chainId: "125" }); assert.deepEqual(node.methods, ["px_getNetwork"]); }
    finally { await close(node.server); }
  } finally { await close(server); }
}

async function journal_concurrent_writers_restart_preserves_terminal_revision(): Promise<void> {
  const receiptPins = [{ sequencerId: Buffer.alloc(32, 119), publicKey: ed25519.getPublicKey(Buffer.alloc(32, 23)), firstBatchNumber: 1n, lastBatchNumber: 1000n }];
  const directory = await mkdtemp(join(tmpdir(), "repair-journal-"));
  const node = await nodePeer();
  try {
    const binding: SnapshotBinding = { chain: id(1), program: id(2), market: id(3), observedSequence: 1000n, executionHeight: 500n, batchId: id(4), nativeStateRoot: id(5), revision: 7n, stateDigest: id(6), epoch: 0n, config: 2n, policy: id(7), roster: null, checkpoint: id(9), settlement: null, rank: 4, publicationTimeMs: 1_700_000_000_000n };
    const key = account.evmPrivateKey; assert(key);
    const publicKey = secp256k1.getPublicKey(key, true);
    const prepared = prepareOperation({ binding, snapshotId: snapshotIdOf(binding) }, 500n, {
      selector: FUND, actor: id(12), roster: "absent", sequence: 1n, expiry: 900n, request: id(13), requiredRank: 4,
      payload: encodeFundRequest({ amount: 10n, refundRecipient: id(14), policyVersion: 1n, consent: true }),
    }, { ...fees, networkId: 125, actorDid: `did:layerx:${account.layerxPublicKey}`, ownerPublicKey: publicKey, accountSequence: 0n, idempotencyKey: id(13), notBefore: 0n, notAfter: 900n, feeLimit: 1000n, capabilities: new Uint8Array(), accessDeclaration: new Uint8Array(), responseCapacity: 4096, resources: [1_000_000n, 16_777_216n, 1_048_576n, 1_048_576n, 64n, 1_048_576n, 4096n] });
    const canonical = prepared.canonicalBytes();
    const review = prepared.review({ version: "1", preparation_id: hex(createHash("sha256").update(canonical).digest()), canonical_bytes: hex(canonical), signing_preimage: hex(keccak_256(canonical)), activity: { version: "1", module: "9", ordinal: "3" }, approval_required: false, approval_id: null });
    const signed = await review.approve(review.terms, publicKey).sign({ publicKey, sign(preimage) {
      const sig = secp256k1.sign(preimage, key, { prehash: false, format: "recovered" });
      return Uint8Array.from([...sig.slice(1), 27 + sig[0]!]);
    } }, 500n, node.provider, account.layerxPublicKey);
    const first = await OperationJournal.open(directory, receiptPins), second = await OperationJournal.open(directory, receiptPins);
    await first.recordSigned(signed);
    const stale = await second.load(signed.activityId);
    const results = await Promise.allSettled([first.expire(signed, 901n), second.persist(stale)]);
    assert.equal(results[0]!.status, "fulfilled");
    await assert.rejects(second.persist(stale), (error: unknown) => error instanceof AiMarketError && error.code === "InvalidTransition");
    const reopened = await OperationJournal.open(directory, receiptPins);
    assert.equal((await reopened.load(signed.activityId)).state, "failed");
    await assert.rejects(reopened.recordSigned(signed), AiMarketError);
    assert.deepEqual(await readdir(directory), [`${signed.activityId}.op`]);

    // Stop between durable submitting and admission completion, then reopen the real journal.
    const recoveryDirectory = join(directory, "recovery");
    const writer = await OperationJournal.open(recoveryDirectory, receiptPins);
    await writer.recordSigned(signed);
    let entered!: () => void, release!: () => void;
    const waiting = new Promise<void>(resolve => { entered = resolve; });
    const released = new Promise<void>(resolve => { release = resolve; });
    const submission = writer.submit(signed, { async request(request) { entered(); await released; return node.provider.request(request); } });
    await waiting;
    const recovery = await OperationJournal.open(recoveryDirectory, receiptPins);
    const recovered = await recovery.load(signed.activityId);
    assert.equal(recovered.state, "unknown"); assert.deepEqual(recovered.signedBytes, signed.signedBytes);
    release();
    await assert.rejects(submission, (error: unknown) => error instanceof AiMarketError && error.code === "InvalidTransition");
    assert.equal((await (await OperationJournal.open(recoveryDirectory, receiptPins)).load(signed.activityId)).state, "unknown");
    assert.equal(node.accepted.length, 1);
  } finally { await close(node.server); await rm(directory, { recursive: true, force: true }); }
}

async function gas_response_bound_quote_submission_status_streaming(): Promise<void> {
  const accountKey = Buffer.alloc(32, 4), sponsorKey = Buffer.alloc(32, 5);
  const batch: SponsoredBatch = { chainId: 125n, account: addressOf(accountKey), nonce: 0n, calls: [{ to: `0x${"33".repeat(20)}`, value: 0n, data: "0x1234" }], quote: { sponsor: addressOf(sponsorKey), token: SIDIORA_TOKEN, maxTokenAmount: 2_100_000n, tokenAmount: 2_000_000n, deadline: 1000n, quoteNonce: 7n, gasCost: 1000n, decimals: 6 } };
  const relayerSignature = signature(value(gasQuoteDigest(batch.chainId, batch.account, batch.quote)), sponsorKey);
  const accountSignature = signature(value(sponsoredBatchDigest(batch)), accountKey);
  let mode: "exact" | "over" | "stream" = "exact";
  let sent = 0;
  const requests: string[] = [];
  const server = createServer(async (incoming, outgoing) => {
    const chunks: Buffer[] = []; for await (const chunk of incoming) chunks.push(Buffer.from(chunk));
    const request = JSON.parse(Buffer.concat(chunks).toString());
    const route = incoming.url;
    assert.equal(incoming.method, "POST");
    assert(route === "/quote" || route === "/submit" || route === "/status");
    requests.push(route);
    if (route === "/quote") {
      assert.equal(request.account, batch.account); assert.equal(request.chainId, "125");
    } else if (route === "/submit") {
      assert.equal(request.batch.account, batch.account); assert.equal(request.accountSignature, accountSignature);
      assert.equal(request.relayerSignature, relayerSignature);
    } else {
      assert.equal(request.account, batch.account); assert.equal(request.quoteNonce, "7");
      assert.equal(request.relayerSignature, relayerSignature);
    }
    const limit = route === "/quote" ? GAS_RESPONSE_LIMITS.quote : route === "/submit" ? GAS_RESPONSE_LIMITS.submission : GAS_RESPONSE_LIMITS.status;
    const payload = JSON.stringify(route === "/quote" ? { quote: batch.quote, relayerSignature, feeDenom: "uhpx" } : route === "/submit" ? { transactionHash: `0x${id(8)}` } : { state: "quoted", deadline: "1000", submission: null, replacement: null, completion: null }, (_key, item: unknown) => typeof item === "bigint" ? item.toString() : item);
    outgoing.setHeader("content-type", "application/json");
    if (mode !== "stream") {
      const body = payload.padEnd(limit + Number(mode === "over"), " "); outgoing.setHeader("content-length", Buffer.byteLength(body)); outgoing.end(body); return;
    }
    // No content-length: stop on cancellation, before producing the complete 8 MiB body.
    for (let offset = 0; offset < 8 * 1048576 && !outgoing.destroyed; offset += 1024) {
      outgoing.write(" ".repeat(1024)); sent += 1024; await nextTurn();
    }
    if (!outgoing.destroyed) outgoing.end(payload);
  });
  const endpoint = await listen(server);
  try {
  const config: GasStationConfig = { chainId: 125n, sponsor: batch.quote.sponsor, token: batch.quote.token, decimals: 6, paymaster: `0x${"44".repeat(20)}`, quoteUrl: `${endpoint}/quote` };
  const auth = { chainId: 125n, address: config.paymaster, nonce: 1n };
  const authorization = value(assembleEip7702Authorization(config, batch.account, auth.nonce, signature(value(eip7702AuthorizationDigest(auth)), accountKey)));
  const identity = { sponsor: config.sponsor, quoteNonce: batch.quote.quoteNonce, account: batch.account, relayerSignature };
  const calls = [() => requestGasQuote(config, { account: batch.account, did: `did:layerx:${id(9)}`, nonce: 0n, calls: batch.calls, maxTokenAmount: batch.quote.maxTokenAmount, gasCost: batch.quote.gasCost }, { now: 999n }), () => submitSponsoredGasBatch(config, batch, accountSignature, relayerSignature, authorization, { now: 999n }), () => requestGasSubmissionStatus(config, identity)];
    for (const call of calls) {
      mode = "exact"; value<unknown>(await call());
      for (mode of ["over", "stream"] as const) {
        sent = 0; const result = await call(); assert.equal(result.ok, false);
        if (result.ok) throw new Error("oversized body accepted");
        assert.equal(result.refusal.code, "invalid_response");
        if (mode === "stream") { await nextTurn(); assert(sent < 8 * 1048576, "read the entire body before refusal"); }
      }
    }
    assert.deepEqual(requests, ["/quote", "/quote", "/quote", "/submit", "/submit", "/submit", "/status", "/status", "/status"]);
  } finally { await close(server); }
}

async function mirror_stdin_early_exit_is_typed_and_parent_survives(): Promise<void> {
  const directory = await mkdtemp(join(tmpdir(), "repair-mirror-"));
  try {
    const executableInput = join(directory, "exit.cjs");
    await writeFile(executableInput, "process.exit(0);\n");
    const verifier = new LocalMirrorExecutableVerifier(process.execPath, executableInput, 5000);
    await assert.rejects(verifier.receipt(1n, { kind: "exact", candidate: { source: 0, commitment: Buffer.alloc(32, 1) } }, Buffer.alloc(8 * 1048576, 1)), (error: unknown) => error instanceof ExecutableMirrorError && ["unavailable", "malformed"].includes(error.code));
    await nextTurn(); assert.equal(process.kill(process.pid, 0), true);
  } finally { await rm(directory, { recursive: true, force: true }); }
}

async function trading_pending_nonce_admits_second_trade_and_preserves_refusals(): Promise<void> {
  const node = await nodePeer();
  const call = spotCall("pecor", "marketBuy(address,address,uint256,uint256,uint256)", [`0x${"31".repeat(20)}`, `0x${"32".repeat(20)}`, 1000n, 900n, 1700000000n]);
  try {
    await TradingWriter.send(node.provider, account, 125n, call, fees);
    assert.equal(await node.provider.request({ method: "eth_getTransactionCount", params: [account.evmAddress, "latest"] }), "0x0");
    const pending = await new JsonRpcClient(node.endpoint).getIdentitySequence(account.evmAddress);
    assert.equal(pending, 1n);
    await TradingWriter.send(node.provider, account, 125n, call, { ...fees, evmNonce: pending });
    assert.deepEqual(node.accepted.map(raw => decodeRawEip1559(raw).nonce), [0n, 1n]);
    await assert.rejects(TradingWriter.send(node.provider, account, 125n, call, { ...fees, evmNonce: 0n }), TradingPayloadError);
    assert.equal(node.accepted.length, 2);
    node.wrongHash();
    await assert.rejects(TradingWriter.send(node.provider, account, 125n, call, { ...fees, evmNonce: 2n }), TradingPayloadError);
    assert.equal(node.accepted.length, 3);
  } finally { await close(node.server); }
}

async function telemetry_outcome_preserves_success_and_original_refusal(): Promise<void> {
  let mutations = 0, refusal = false;
  const requestIds: string[] = [];
  const server = createServer(async (incoming, outgoing) => {
    const chunks: Buffer[] = []; for await (const chunk of incoming) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(Buffer.concat(chunks).toString());
    assert.equal(incoming.url, "/v1/agent/rpc"); assert.equal(body.operation, "session.close");
    assert.equal(incoming.headers.authorization, `LayerX-Key key_1:lxp_live_${id(3)}`);
    assert.equal(body.credential.token_id, id(2));
    requestIds.push(body.request_id);
    if (!refusal) mutations++;
    outgoing.writeHead(refusal ? 403 : 200, { "content-type": "application/json" });
    outgoing.end(JSON.stringify(refusal ? { request_id: body.request_id, class: "PolicyRefusal", protocol_result_code: 17, retriability: "Terminal", reason: "policy.denied" } : { request_id: body.request_id, value: { closed: true }, verification_status: { state: "achieved", rung: "submitted" } }));
  });
  const endpoint = await listen(server);
  const events: SdkTelemetryEvent[] = [], diagnostics: unknown[] = [];
  const transport = new AgentEnvelopeTransport({ endpoint, session: new AgentSessionCredential("repair", id(1), new SecretBytes(Uint8Array.from(Buffer.alloc(32, 2))), 1n), gatewayCredential: new LayerXKeyCredential("key_1", new SecretBytes(Uint8Array.from(Buffer.from(`lxp_live_${id(3)}`)))) });
  const client = new ProductionClient(transport, event => { events.push(event); throw new Error("observer failed"); }, failure => { diagnostics.push(failure); throw new Error("diagnostic failed"); });
  try {
    const response = await client.agent<{}, { value: { closed: boolean }; verification_status: { state: string; rung: Rung } }>("session.close", {}, { idempotencyKey: idempotencyKey(id(4)) });
    assert.deepEqual(response.value, { closed: true }); assert.equal(mutations, 1);
    assert.deepEqual(response.verification_status, { state: "achieved", rung: Rung.Submitted });
    assert.deepEqual(events.map(event => event.outcome), ["completed"]); assert.equal(diagnostics.length, 1);
    refusal = true;
    await assert.rejects(client.agent("session.close", {}, { idempotencyKey: idempotencyKey(id(5)) }), (error: unknown) => {
      assert(error instanceof PlatformSdkError); assert.equal(error.code, "policy-refusal"); assert.equal(error.retry, "never");
      assert.equal(error.requestId, requestIds[1]); assert.equal(error.protocolResultCode, 17); return true;
    });
    assert.equal(requestIds.length, 2);
    assert.deepEqual(events.map(event => event.outcome), ["completed", "refused"]); assert.equal(diagnostics.length, 2); assert.equal(mutations, 1);
  } finally { await close(server); }
}

function lifecycle_size_exact_metadata_deploy_upgrade_boundaries(): void {
  const maximum = NATIVE_PROGRAM_REGISTRATION_MAX_BYTES;
  for (const metadataSize of [0, 1, 952]) {
    const metadata = metadataSize === 0 ? {} : { interface: Buffer.alloc(metadataSize, 1) };
    for (const extra of [-1, 0, 1]) {
      for (const kind of ["deploy", "upgrade"] as const) {
        const hook = Buffer.alloc(kind === "upgrade" ? 17 : 0, 1);
        const framing = (kind === "deploy" ? 104 : 106) + hook.length + (metadataSize === 0 ? 0 : 4 + metadataSize);
        const wasm = Buffer.alloc(maximum + extra - framing);
        wasm.set([0, 97, 115, 109, 1, 0, 0, 0]);
        const common = { programId: Buffer.alloc(32, 1), guestAbi: 5 as const, wasm, newHash: createHash("sha256").update(wasm).digest(), ...metadata };
        const deploy = { ...common, policy: 0 as const, authority: Buffer.alloc(32) };
        const upgrade = { ...common, oldHash: Buffer.alloc(32, 2), migrationHook: hook, clearInterface: false };
        const encode = () => kind === "deploy" ? encodeNativeProgramDeploy(deploy) : encodeNativeProgramUpgrade(upgrade);
        if (extra > 0) assert.throws(encode, /payload exceeds network limit/u);
        else {
          const encoded = encode(); assert.equal(encoded.length, maximum + extra);
          const decoded = kind === "deploy" ? decodeNativeProgramDeploy(encoded) : decodeNativeProgramUpgrade(encoded);
          assert.deepEqual(decoded.wasm, wasm); assert.deepEqual(decoded.interface, metadata.interface);
        }
      }
    }
  }
}

async function subscription_backlog_reconciles_inclusively_without_lost_logs(): Promise<void> {
  const address = `0x${"11".repeat(20)}`;
  const logs = Array.from({ length: SUBSCRIPTION_MAX_BACKLOG + 4 }, (_, index) => ({ address, blockNumber: `0x${(1 + Math.floor(index / 2)).toString(16)}`, blockHash: `0x${id(2 + Math.floor(index / 2))}`, transactionHash: `0x${id(index + 3)}`, transactionIndex: "0x0", logIndex: `0x${(index % 2).toString(16)}`, data: "0x", topics: [], removed: false }));
  const server = createServer(async (incoming, outgoing) => {
    const chunks: Buffer[] = []; for await (const chunk of incoming) chunks.push(Buffer.from(chunk));
    const rpc = JSON.parse(Buffer.concat(chunks).toString());
    assert.equal(incoming.url, "/"); assert.equal(rpc.method, "eth_getLogs"); assert.equal(rpc.params[0].fromBlock, "0x1");
    outgoing.writeHead(200, { "content-type": "application/json" });
    outgoing.end(JSON.stringify({ jsonrpc: "2.0", id: rpc.id, result: logs }));
  });
  const wss = new WebSocketServer({ server });
  let peer!: WebSocket;
  let connected!: () => void;
  const connection = new Promise<void>(resolve => { connected = resolve; });
  wss.on("connection", socket => {
    peer = socket;
    socket.once("message", data => {
      const request = JSON.parse(data.toString()); assert.equal(request.method, "eth_subscribe");
      socket.send(JSON.stringify({ jsonrpc: "2.0", id: request.id, result: "repair" }));
      socket.send(JSON.stringify({ jsonrpc: "2.0", method: "eth_subscription", params: { subscription: "repair", result: logs[0] } }));
      connected();
    });
  });
  const endpoint = await listen(server);
  const stream = subscribeRpc(new URL(endpoint), undefined, "repair", "account", address);
  try {
    const next = stream.next(); await connection;
    const first = await next; assert(!first.done); assert.equal(first.value.cursor, 1n);
    const closed = once(peer, "close");
    for (const log of logs.slice(1)) peer.send(JSON.stringify({ jsonrpc: "2.0", method: "eth_subscription", params: { subscription: "repair", result: log } }));
    await closed;
    let cursor: bigint | undefined;
    await assert.rejects(stream.next(), (error: unknown) => {
      assert(error instanceof SubscriptionReconciliationRequiredError);
      assert.equal(error.code, "reconciliation-required"); cursor = error.lastDeliveredCursor; return true;
    });
    assert.equal(cursor, 1n);
    const response = await fetch(endpoint, { method: "POST", body: JSON.stringify({ jsonrpc: "2.0", id: "read", method: "eth_getLogs", params: [{ address, fromBlock: `0x${cursor!.toString(16)}`, toBlock: "latest" }] }) });
    const recovered = await response.json() as { result: typeof logs };
    const key = (log: typeof logs[number]) => `${log.blockHash}:${log.transactionHash}:${log.logIndex}`;
    const seen = new Map([[key(first.value.result as typeof logs[number]), first.value.result]]);
    for (const log of recovered.result) seen.set(key(log), log);
    assert.deepEqual([...seen.values()], logs);
  } finally { await stream.return(undefined); for (const socket of wss.clients) socket.terminate(); await new Promise<void>(resolve => wss.close(() => resolve())); await close(server); }
}

test(daemon_surface_hosted_authentication_and_node_routing);
test(journal_concurrent_writers_restart_preserves_terminal_revision);
test(gas_response_bound_quote_submission_status_streaming);
test(mirror_stdin_early_exit_is_typed_and_parent_survives);
test(trading_pending_nonce_admits_second_trade_and_preserves_refusals);
test(telemetry_outcome_preserves_success_and_original_refusal);
test(lifecycle_size_exact_metadata_deploy_upgrade_boundaries);
test(subscription_backlog_reconciles_inclusively_without_lost_logs);
