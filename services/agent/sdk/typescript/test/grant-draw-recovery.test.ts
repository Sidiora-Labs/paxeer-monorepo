import assert from "node:assert/strict";
import { fork, type ChildProcess } from "node:child_process";
import { once } from "node:events";
import { mkdtempSync, rmSync } from "node:fs";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { DatabaseSync } from "node:sqlite";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { secp256k1 } from "@noble/curves/secp256k1.js";

import { decodeRawEip1559, eip1559SigningHash, encodeUnsignedEip1559, recoverEip1559Sender } from "../src/evm-transaction.js";
import { precompileWriteCall } from "../src/exchange.js";
import { JsonRpcClient } from "../src/rpc.js";
import { bindDrawTransaction, type ExpectedDrawTransaction } from "../src/x402/activity.js";
import { PreparedGrantDraws, type GrantDrawRpc } from "../src/x402/draw.js";
import { validateGrantDraw, type GrantOffer } from "../src/x402/grant.js";
import { LAYERX_LEDGER_PRECOMPILE } from "../src/x402/receive.js";

const id = (value: number): string => value.toString(16).padStart(2, "0").repeat(32);
const hex = (value: Uint8Array): string => Buffer.from(value).toString("hex");
const bytes = (value: string): Uint8Array => Uint8Array.from(Buffer.from(value.replace(/^0x/u, ""), "hex"));
const expected: ExpectedDrawTransaction = {
  grantId: id(1), recipient: id(3), amount: 40n, idempotencyKey: id(11),
  sequence: 7n, purposeHash: id(5), contextHash: id(12), chainId: 125n,
};
const requirements: GrantOffer = {
  scheme: "subscription", asset: id(4), amount: "40", payTo: expected.recipient,
  extra: { layerx: {
    payer: id(2), grantId: expected.grantId, receiverSequence: expected.sequence.toString(),
    purposeHash: expected.purposeHash, contextHash: expected.contextHash, windowSeconds: "3600",
  } },
};
const periodKey = "buyer:subscription:period-7";

function drawCalldata(draw: ExpectedDrawTransaction): Uint8Array {
  const call = precompileWriteCall(LAYERX_LEDGER_PRECOMPILE,
    "draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)",
    { types: ["bytes32", "bytes32", "uint128", "bytes32", "uint64", "bytes32", "bytes32"], payable: false },
    [`0x${draw.grantId}`, `0x${draw.recipient}`, draw.amount, `0x${draw.idempotencyKey}`,
      draw.sequence, `0x${draw.purposeHash}`, `0x${draw.contextHash}`]);
  return bytes(call.data);
}

const request = {
  principal: "buyer", requestDigest: id(9), draw: hex(drawCalldata(expected)),
  idempotencyKey: expected.idempotencyKey, requirements,
};

function signedDraw(draw: ExpectedDrawTransaction = expected): Uint8Array {
  const transaction = {
    chainId: draw.chainId, nonce: 4n, maxPriorityFeePerGas: 1n, maxFeePerGas: 2n,
    gasLimit: 100000n, to: bytes(LAYERX_LEDGER_PRECOMPILE), value: 0n,
    data: drawCalldata(draw), accessList: [],
  };
  const unsigned = encodeUnsignedEip1559(transaction);
  const signature = secp256k1.sign(eip1559SigningHash(transaction), new Uint8Array(32).fill(1),
    { prehash: false, format: "recovered" });
  const parsed = secp256k1.Signature.fromBytes(signature, "recovered");
  const scalar = (value: bigint): Buffer => {
    if (value === 0n) return Buffer.of(0x80);
    const text = value.toString(16);
    const encoded = Buffer.from(text.padStart(text.length + text.length % 2, "0"), "hex");
    return encoded.length === 1 && encoded[0]! < 0x80 ? encoded : Buffer.concat([Buffer.of(0x80 + encoded.length), encoded]);
  };
  const lengthWidth = unsigned[1]! - 0xf7;
  const payload = Buffer.concat([unsigned.subarray(2 + lengthWidth),
    scalar(BigInt(signature[0]!)), scalar(parsed.r), scalar(parsed.s)]);
  const length = Buffer.alloc(2);
  length.writeUInt16BE(payload.length);
  const raw = Uint8Array.from(Buffer.concat([Buffer.of(2, 0xf9), length, payload]));
  assert.equal(hex(decodeRawEip1559(raw).data), hex(transaction.data));
  assert.match(recoverEip1559Sender(raw), /^0x[0-9a-fA-F]{40}$/u);
  return raw;
}

function transport(endpoint: string, crashBeforeSend = false): GrantDrawRpc {
  const client = new JsonRpcClient(endpoint);
  return {
    async sendRawTransaction(raw) {
      if (crashBeforeSend) process.kill(process.pid, "SIGKILL");
      return client.sendRawTransaction(bytes(raw));
    },
    sequencerReceipt: (hash) => client.sequencerReceipt(hash),
  };
}

interface CapturedRequest {
  readonly method: string;
  readonly parameter: string;
}

async function interruptingTransport(holdSend = false) {
  const captured: CapturedRequest[] = [];
  let onSend: (() => void) | undefined;
  const sent = new Promise<void>((resolve) => { onSend = resolve; });
  const failures: unknown[] = [];
  const server = createServer(async (incoming, response) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of incoming) chunks.push(Buffer.from(chunk));
      const message: { readonly jsonrpc: string; readonly method: string; readonly params: readonly string[] } = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      assert.equal(message.jsonrpc, "2.0");
      assert.equal(message.params.length, 1);
      assert.equal(typeof message.params[0], "string");
      captured.push({ method: message.method, parameter: message.params[0]! });
      if (message.method === "eth_sendRawTransaction") {
        const raw = bytes(message.params[0]!);
        bindDrawTransaction(raw, expected);
        validateGrantDraw(decodeRawEip1559(raw).data, requirements, expected.idempotencyKey);
        onSend!();
        if (holdSend) return;
      } else {
        assert.equal(message.method, "pax_sequencerReceipt");
      }
    } catch (error) {
      failures.push(error);
    }
    response.destroy();
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert(address !== null && typeof address !== "string");
  return {
    endpoint: `http://127.0.0.1:${address.port}`, captured, sent,
    async close() {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
      assert.deepEqual(failures, []);
    },
  };
}

function stored(path: string) {
  const db = new DatabaseSync(path);
  try {
    const rows = db.prepare("SELECT * FROM grant_draw_transactions").all();
    assert.equal(rows.length, 1);
    return rows[0]!;
  } finally {
    db.close();
  }
}

function assertUnknown(path: string, raw: Uint8Array): void {
  const row = stored(path);
  assert.equal(row["admission_state"], "unknown");
  assert.equal(row["attempted"], 1);
  assert.equal(row["idempotency_key"], expected.idempotencyKey);
  assert.equal(row["period_key"], periodKey);
  assert.equal(row["principal"], request.principal);
  assert.equal(row["request_digest"], request.requestDigest);
  assert.equal(row["transaction_hash"], bindDrawTransaction(raw, expected));
  assert.deepEqual(row["raw_transaction"], raw);
  assert.equal(hex(row["calldata"] as Uint8Array), request.draw);
}

function initialize(path: string, endpoint: string): Uint8Array {
  const raw = signedDraw();
  const draws = new PreparedGrantDraws(path, expected.recipient, expected.chainId, transport(endpoint), []);
  try {
    draws.register(request.principal, request.requestDigest, raw, expected, periodKey);
  } finally {
    draws.close();
  }
  return raw;
}

function worker(path: string, endpoint: string, crashBeforeSend: boolean): ChildProcess {
  return fork(fileURLToPath(import.meta.url), ["draw-recovery-worker", path, endpoint, String(crashBeforeSend)],
    { stdio: ["ignore", "ignore", "pipe", "ipc"] });
}

async function waitForKilled(child: ChildProcess): Promise<void> {
  const [code, signal] = await once(child, "exit", { signal: AbortSignal.timeout(10000) });
  assert.equal(code, null);
  assert.equal(signal, "SIGKILL");
}

async function redeliver(path: string, endpoint: string, raw: Uint8Array): Promise<void> {
  const draws = new PreparedGrantDraws(path, expected.recipient, expected.chainId, transport(endpoint), []);
  try {
    draws.register(request.principal, request.requestDigest, raw, expected, periodKey);
    assert.throws(() => draws.register(request.principal, id(10), raw, expected, periodKey), /draw-registration-conflict/u);
    assert.throws(() => draws.register(request.principal, request.requestDigest, raw, expected, "another-period"), /draw-registration-conflict/u);
    const another = { ...expected, idempotencyKey: id(13) };
    assert.throws(() => draws.register(request.principal, request.requestDigest, signedDraw(another), another, periodKey), /UNIQUE constraint failed/u);
    await assert.rejects(draws.execute({ ...request, principal: "another-buyer" }), /unregistered-grant-draw/u);
    await assert.rejects(draws.execute({ ...request, requestDigest: id(10) }), /unregistered-grant-draw/u);
    const executions = await Promise.allSettled([draws.execute(request), draws.execute(request)]);
    for (const execution of executions) {
      assert.equal(execution.status, "rejected");
      if (execution.status !== "rejected") throw new Error("transport interruption unexpectedly succeeded");
      assert(execution.reason instanceof AggregateError);
      assert.equal(execution.reason.message, "draw-admission-unresolved");
      assert.equal(execution.reason.errors.length, 2);
      for (const error of execution.reason.errors) {
        assert(error instanceof Error && "code" in error);
        assert.equal(error.code, "ECONNRESET");
      }
    }
  } finally {
    draws.close();
  }
  assertUnknown(path, raw);
}

async function draw_restart_recovers_a_crash_before_send(): Promise<void> {
  const directory = mkdtempSync(join(tmpdir(), "draw-before-send-"));
  const endpoint = await interruptingTransport();
  let child: ChildProcess | undefined;
  try {
    const path = join(directory, "draws.sqlite");
    const raw = initialize(path, endpoint.endpoint);
    child = worker(path, endpoint.endpoint, true);
    await waitForKilled(child);
    assert.deepEqual(endpoint.captured, []);
    assertUnknown(path, raw);
    await redeliver(path, endpoint.endpoint, raw);
    await redeliver(path, endpoint.endpoint, raw);
    const pair = [
      { method: "pax_sequencerReceipt", parameter: bindDrawTransaction(raw, expected) },
      { method: "eth_sendRawTransaction", parameter: `0x${hex(raw)}` },
    ];
    assert.deepEqual(endpoint.captured, [...pair, ...pair]);
  } finally {
    if (child && child.exitCode === null && child.signalCode === null) child.kill("SIGKILL");
    await endpoint.close();
    rmSync(directory, { recursive: true, force: true });
  }
}

async function draw_restart_redelivers_after_an_uncertain_send_acknowledgement(): Promise<void> {
  const directory = mkdtempSync(join(tmpdir(), "draw-uncertain-ack-"));
  const interrupted = await interruptingTransport(true);
  const recovery = await interruptingTransport();
  let child: ChildProcess | undefined;
  try {
    const path = join(directory, "draws.sqlite");
    const raw = initialize(path, interrupted.endpoint);
    child = worker(path, interrupted.endpoint, false);
    const exited = waitForKilled(child);
    await Promise.race([interrupted.sent, exited.then(() => { throw new Error("worker exited before sending"); })]);
    assertUnknown(path, raw);
    child.kill("SIGKILL");
    await exited;
    await redeliver(path, recovery.endpoint, raw);
    assert.deepEqual(interrupted.captured, [{ method: "eth_sendRawTransaction", parameter: `0x${hex(raw)}` }]);
    assert.deepEqual(recovery.captured, [
      { method: "pax_sequencerReceipt", parameter: bindDrawTransaction(raw, expected) },
      { method: "eth_sendRawTransaction", parameter: interrupted.captured[0]!.parameter },
    ]);
  } finally {
    if (child && child.exitCode === null && child.signalCode === null) child.kill("SIGKILL");
    await interrupted.close();
    await recovery.close();
    rmSync(directory, { recursive: true, force: true });
  }
}

async function draw_restart_recovers_a_send_failure_before_admission(): Promise<void> {
  const directory = mkdtempSync(join(tmpdir(), "draw-send-failure-"));
  const endpoint = await interruptingTransport();
  try {
    const path = join(directory, "draws.sqlite");
    const raw = initialize(path, endpoint.endpoint);
    const draws = new PreparedGrantDraws(path, expected.recipient, expected.chainId, transport(endpoint.endpoint), []);
    try {
      await assert.rejects(draws.execute(request), (error: unknown) => {
        assert(error instanceof Error && "code" in error);
        assert.equal(error.code, "ECONNRESET");
        return true;
      });
    } finally {
      draws.close();
    }
    assertUnknown(path, raw);
    await redeliver(path, endpoint.endpoint, raw);
    assert.deepEqual(endpoint.captured, [
      { method: "eth_sendRawTransaction", parameter: `0x${hex(raw)}` },
      { method: "pax_sequencerReceipt", parameter: bindDrawTransaction(raw, expected) },
      { method: "eth_sendRawTransaction", parameter: `0x${hex(raw)}` },
    ]);
  } finally {
    await endpoint.close();
    rmSync(directory, { recursive: true, force: true });
  }
}

async function draw_restart_migrates_legacy_attempts_without_losing_the_payment(): Promise<void> {
  const directory = mkdtempSync(join(tmpdir(), "draw-legacy-"));
  const endpoint = await interruptingTransport();
  try {
    const path = join(directory, "draws.sqlite");
    const raw = initialize(path, endpoint.endpoint);
    const db = new DatabaseSync(path);
    try {
      db.exec("ALTER TABLE grant_draw_transactions DROP COLUMN admission_state; UPDATE grant_draw_transactions SET attempted = 1");
    } finally {
      db.close();
    }
    await redeliver(path, endpoint.endpoint, raw);
    assert.deepEqual(endpoint.captured, [
      { method: "pax_sequencerReceipt", parameter: bindDrawTransaction(raw, expected) },
      { method: "eth_sendRawTransaction", parameter: `0x${hex(raw)}` },
    ]);
  } finally {
    await endpoint.close();
    rmSync(directory, { recursive: true, force: true });
  }
}

if (process.argv[2] === "draw-recovery-worker") {
  const path = process.argv[3]!;
  const endpoint = process.argv[4]!;
  const draws = new PreparedGrantDraws(path, expected.recipient, expected.chainId,
    transport(endpoint, process.argv[5] === "true"), []);
  try {
    await draws.execute(request);
    throw new Error("worker unexpectedly completed before interruption");
  } finally {
    draws.close();
  }
} else {
  test(draw_restart_recovers_a_crash_before_send);
  test(draw_restart_redelivers_after_an_uncertain_send_acknowledgement);
  test(draw_restart_recovers_a_send_failure_before_admission);
  test(draw_restart_migrates_legacy_attempts_without_losing_the_payment);
}
