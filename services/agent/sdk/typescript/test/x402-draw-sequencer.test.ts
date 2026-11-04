import assert from "node:assert/strict";
import { readFileSync, mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { JsonRpcClient } from "../src/rpc.js";
import { meteredRules, unhex, webReceiptKeys, localWebFixture } from "./recorded-web-search.js";
import { decodeLedgerTransaction } from "../src/ledger-payment.js";
import { createServer } from "node:http";
import { DatabaseSync } from "node:sqlite";
import { test } from "node:test";

import { signedDraw as recordedSignedDraw } from "./record-draw.js";

import { decodeRawEip1559, eip1559SigningHash, keccak256, type Eip1559Transaction } from "../src/evm-transaction.js";
import { SequencerReceiptVerificationError, type SequencerReceiptKey } from "../src/verifier.js";
import { bindDrawTransaction, type ExpectedDrawTransaction } from "../src/x402/activity.js";
import { PreparedGrantDraws, type GrantDrawRpc } from "../src/x402/draw.js";
import { validateGrantDraw, type GrantOffer } from "../src/x402/grant.js";
import { LAYERX_LEDGER_PRECOMPILE } from "../src/x402/receive.js";

const id = (value: number): string => value.toString(16).padStart(2, "0").repeat(32);
const hex = (value: Uint8Array): string => Buffer.from(value).toString("hex");
const bytes = (value: string): Uint8Array => Uint8Array.from(Buffer.from(value.replace(/^0x/u, ""), "hex"));
const word = (value: bigint): string => value.toString(16).padStart(64, "0");
const recovery = localWebFixture<{ readonly rawTransaction: string; readonly calldata: string; readonly txHash: string; readonly receipt: string }>("draw-recovery.json");
const expected: ExpectedDrawTransaction = {
  grantId: id(1), recipient: id(3), amount: 40n, idempotencyKey: id(11),
  sequence: 7n, purposeHash: id(5), contextHash: id(12), chainId: 125n,
};
const offer: GrantOffer = {
  scheme: "metered", asset: id(4), amount: "40", payTo: expected.recipient,
  extra: { layerx: {
    payer: id(2), grantId: expected.grantId, receiverSequence: expected.sequence.toString(),
    purposeHash: expected.purposeHash, contextHash: expected.contextHash,
  } },
};

function calldata(draw: ExpectedDrawTransaction = expected): Uint8Array {
  return bytes("2e89490f" + draw.grantId + draw.recipient + word(draw.amount)
    + draw.idempotencyKey + word(draw.sequence) + draw.purposeHash + draw.contextHash);
}

function signedDraw(nonce = 4n, draw: ExpectedDrawTransaction = expected): Uint8Array { return recordedSignedDraw(nonce, draw); }

async function prepared_draw_submits_a_raw_draw_transaction(): Promise<void> {
  const vectors: { readonly cases: readonly {
    readonly signedReceipt: string;
    readonly pinnedKey: { readonly keyId: string; readonly publicKey: string; readonly firstHeight: number; readonly lastHeight: number };
  }[] } = JSON.parse(readFileSync(new URL(
    "../../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url,
  ), "utf8"));
  const vector = vectors.cases[0]!;
  const receiptKey: SequencerReceiptKey = {
    keyId: bytes(vector.pinnedKey.keyId), publicKey: bytes(vector.pinnedKey.publicKey),
    first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight),
  };
  const ingress = new DatabaseSync(":memory:");
  ingress.exec("CREATE TABLE submissions (ordinal INTEGER PRIMARY KEY, transaction_hash TEXT NOT NULL, raw BLOB NOT NULL); CREATE TABLE receipts (transaction_hash TEXT PRIMARY KEY, receipt TEXT NOT NULL)");
  const server = createServer(async (request, response) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const message: { readonly jsonrpc: string; readonly id: number; readonly method: string; readonly params: readonly string[] } = JSON.parse(Buffer.concat(chunks).toString());
      assert.equal(message.jsonrpc, "2.0");
      assert.equal(message.params.length, 1);
      const parameter = message.params[0]!;
      let result: unknown;
      if (message.method === "eth_sendRawTransaction") {
        assert.match(parameter, /^0x(?:[0-9a-f]{2})+$/u);
        const raw = bytes(parameter);
        const transaction = decodeRawEip1559(raw);
        assert.equal(transaction.chainId, 125n);
        const transactionHash = `0x${hex(keccak256(raw))}`;
        ingress.prepare("INSERT INTO submissions (transaction_hash, raw) VALUES (?, ?)").run(transactionHash, raw);
        result = transactionHash;
      } else {
        assert.equal(message.method, "pax_sequencerReceipt");
        const stored = ingress.prepare("SELECT receipt FROM receipts WHERE transaction_hash = ?").get(parameter);
        result = stored ? { txHash: parameter, rung: "receipted", receipt: stored["receipt"] }
          : { txHash: parameter, rung: "submitted" };
      }
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: "2.0", id: message.id, result }));
    } catch (error) {
      response.writeHead(500);
      response.end(String(error));
    }
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  assert(address !== null && typeof address !== "string");
  const call = async (method: string, parameter: string): Promise<unknown> => {
    const response = await fetch(`http://127.0.0.1:${address.port}`, {
      method: "POST", headers: { "content-type": "application/json" },
      body: JSON.stringify({ jsonrpc: "2.0", id: 1, method, params: [parameter] }),
    });
    assert.equal(response.status, 200);
    const message = await response.json() as { readonly result: unknown };
    return message.result;
  };
  const rpc: GrantDrawRpc = {
    async sendRawTransaction(raw) {
      const hash = await call("eth_sendRawTransaction", raw);
      assert.equal(typeof hash, "string");
      return hash as string;
    },
    async sequencerReceipt(hash) {
      const result = await call("pax_sequencerReceipt", hash);
      assert(result !== null && typeof result === "object" && !Array.isArray(result));
      return result as Record<string, unknown>;
    },
  };
  const draws = new PreparedGrantDraws(":memory:", expected.recipient, 125n, rpc, [receiptKey]);
  try {
    const raw = signedDraw();
    const hash = bindDrawTransaction(raw, expected);
    const request = { principal: "buyer", requestDigest: id(9), draw: hex(calldata()),
      idempotencyKey: expected.idempotencyKey, requirements: offer };
    draws.register(request.principal, request.requestDigest, raw, expected);
    draws.register(request.principal, request.requestDigest, raw, expected);
    assert.throws(() => draws.register(request.principal, request.requestDigest, signedDraw(5n), expected), /draw-registration-conflict/u);
    await assert.rejects(draws.execute({ ...request, principal: "other-buyer" }), /unregistered-grant-draw/u);
    await assert.rejects(draws.execute({ ...request, requestDigest: id(10) }), /unregistered-grant-draw/u);
    await assert.rejects(draws.execute({ ...request, draw: hex(calldata({ ...expected, amount: 41n })) }), /unregistered-grant-draw/u);
    await assert.rejects(draws.execute({ ...request, requirements: { ...offer, amount: "39" } }), /invalid-grant-draw/u);
    assert.equal(ingress.prepare("SELECT count(*) AS count FROM submissions").get()!["count"], 0);
    assert.deepEqual(await Promise.all([draws.execute(request), draws.execute(request)]), [{ kind: "pending" }, { kind: "pending" }]);
    assert.deepEqual(await draws.execute(request), { kind: "pending" });
    const submissions = ingress.prepare("SELECT transaction_hash, raw FROM submissions").all();
    assert.equal(submissions.length, 1);
    assert.equal(submissions[0]!["transaction_hash"], hash);
    assert.deepEqual(submissions[0]!["raw"], raw);
    assert.equal(hex(decodeRawEip1559(submissions[0]!["raw"] as Uint8Array).data), request.draw);
    ingress.prepare("INSERT INTO receipts (transaction_hash, receipt) VALUES (?, ?)").run(hash, vector.signedReceipt);
    await assert.rejects(draws.execute(request), (error: unknown) => {
      assert(error instanceof SequencerReceiptVerificationError);
      assert.equal(error.check, "hash_mismatch");
      return true;
    });
    const corrupted = bytes(vector.signedReceipt);
    corrupted[corrupted.length - 1]! ^= 1;
    ingress.prepare("UPDATE receipts SET receipt = ? WHERE transaction_hash = ?").run(`0x${hex(corrupted)}`, hash);
    await assert.rejects(draws.execute(request), (error: unknown) => {
      assert(error instanceof SequencerReceiptVerificationError);
      assert.equal(error.check, "bad_signature");
      return true;
    });
    assert.equal(ingress.prepare("SELECT count(*) AS count FROM submissions").get()!["count"], 1);
  } finally {
    draws.close();
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    ingress.close();
  }
}

function grant_draw_validation_checks_the_draw_calldata_against_the_offer(): void {
  assert.deepEqual(validateGrantDraw(calldata(), offer, expected.idempotencyKey), {
    grantId: expected.grantId, recipient: expected.recipient, amount: expected.amount,
    idempotencyKey: expected.idempotencyKey, sequence: expected.sequence,
    purposeHash: expected.purposeHash, contextHash: expected.contextHash,
  });
  assert.throws(() => validateGrantDraw(calldata({ ...expected, amount: 41n }), offer, expected.idempotencyKey), /invalid-grant-draw/u);
  for (const mismatch of [
    { grantId: id(20) }, { recipient: id(20) }, { amount: 39n }, { amount: 0n },
    { idempotencyKey: id(20) }, { sequence: 8n }, { purposeHash: id(20) }, { contextHash: id(20) },
    { amount: 1n << 128n }, { sequence: 1n << 64n },
  ]) assert.throws(() => validateGrantDraw(calldata({ ...expected, ...mismatch }), offer, expected.idempotencyKey), /invalid-grant-draw/u);
  assert.throws(() => validateGrantDraw(calldata(), offer, id(20)), /invalid-grant-draw/u);
  for (const malformed of [calldata().slice(0, -1), Buffer.concat([calldata(), Buffer.of(0)]),
    Buffer.concat([Buffer.alloc(4), calldata().slice(4)]), new Uint8Array(733)]) {
    assert.throws(() => validateGrantDraw(malformed, offer, expected.idempotencyKey), /invalid-grant-draw/u);
  }
  for (const amount of ["0", "040", "-1", (1n << 128n).toString()]) {
    assert.throws(() => validateGrantDraw(calldata(), { ...offer, amount }, expected.idempotencyKey), /invalid-grant-draw/u);
  }
  for (const asset of [id(0), "ab".repeat(31)]) {
    assert.throws(() => validateGrantDraw(calldata(), { ...offer, asset }, expected.idempotencyKey), /invalid-grant-draw/u);
  }
  const subscription: GrantOffer = { ...offer, scheme: "subscription",
    extra: { layerx: { ...offer.extra.layerx, windowSeconds: "3600" } } };
  assert.deepEqual(validateGrantDraw(calldata(), subscription, expected.idempotencyKey), validateGrantDraw(calldata(), offer, expected.idempotencyKey));
  assert.throws(() => validateGrantDraw(calldata(), { ...subscription, extra: offer.extra }, expected.idempotencyKey), /invalid-subscription-grant/u);
  assert.throws(() => validateGrantDraw(calldata(), { ...offer, extra: subscription.extra }, expected.idempotencyKey), /invalid-metered-grant/u);
}

test(prepared_draw_submits_a_raw_draw_transaction);
test(grant_draw_validation_checks_the_draw_calldata_against_the_offer);


async function reopenDrawCase(interruption: "before-send" | "uncertain-response" | "receipt-after-restart"): Promise<void> {
  const directory = mkdtempSync(join(tmpdir(), "prepared-draw-reopen-"));
  const path = join(directory, "draws.sqlite");
  const ingress = new DatabaseSync(join(directory, "ingress.sqlite"));
  ingress.exec("CREATE TABLE payments (transaction_hash TEXT PRIMARY KEY, raw BLOB NOT NULL); CREATE TABLE deliveries (raw BLOB NOT NULL)");
  const raw = signedDraw();
  const hash = bindDrawTransaction(raw, expected);
  let disconnect = interruption === "uncertain-response";
  let receipt: string | undefined;
  const methods: string[] = [];
  const failures: unknown[] = [];
  const server = createServer(async (incoming, outgoing) => {
    try {
      assert.equal(incoming.url, "/");
      const chunks: Buffer[] = [];
      for await (const chunk of incoming) chunks.push(Buffer.from(chunk));
      const message = JSON.parse(Buffer.concat(chunks).toString()) as { readonly id: string; readonly method: string; readonly params: readonly string[] };
      methods.push(message.method);
      assert.equal(message.params.length, 1);
      let result: unknown;
      if (message.method === "eth_sendRawTransaction") {
        assert.equal(message.params[0], `0x${hex(raw)}`);
        const delivered = bytes(message.params[0]!);
        assert.equal(bindDrawTransaction(delivered, expected), hash);
        assert.deepEqual(validateGrantDraw(decodeRawEip1559(delivered).data, offer, expected.idempotencyKey),
          validateGrantDraw(calldata(), offer, expected.idempotencyKey));
        ingress.prepare("INSERT INTO deliveries (raw) VALUES (?)").run(delivered);
        ingress.prepare("INSERT OR IGNORE INTO payments (transaction_hash, raw) VALUES (?, ?)").run(hash, delivered);
        if (disconnect) { disconnect = false; outgoing.destroy(); return; }
        result = hash;
      } else {
        assert.equal(message.method, "pax_sequencerReceipt"); assert.equal(message.params[0], hash);
        const admitted = ingress.prepare("SELECT raw FROM payments WHERE transaction_hash = ?").get(hash);
        // An uncertain response is reconciled as absent once, forcing exact-byte redelivery.
        result = interruption === "uncertain-response" && methods.filter(method => method === "eth_sendRawTransaction").length === 1
          ? null : admitted === undefined ? null
          : receipt === undefined ? { txHash: hash, rung: "submitted" } : { txHash: hash, rung: "receipted", receipt };
      }
      outgoing.writeHead(200, { "content-type": "application/json" });
      outgoing.end(JSON.stringify({ jsonrpc: "2.0", id: message.id, result }));
    } catch (error) { failures.push(error); outgoing.writeHead(500); outgoing.end(String(error)); }
  });
  await new Promise<void>(resolve => server.listen(0, "127.0.0.1", resolve));
  const address = server.address(); assert(address !== null && typeof address !== "string");
  const client = new JsonRpcClient(`http://127.0.0.1:${address.port}`);
  let draws: PreparedGrantDraws | undefined;
  let interruptedBeforeSend = false;
  const rpc: GrantDrawRpc = {
    sendRawTransaction: raw => {
      // Cut the component at the transport boundary, after persisting unknown admission and before sending any bytes.
      if (interruption === "before-send" && !interruptedBeforeSend) {
        interruptedBeforeSend = true; draws!.close(); draws = undefined;
        return Promise.reject(new Error("interrupted-before-send"));
      }
      return client.sendRawTransaction(bytes(raw));
    }, sequencerReceipt: async hash => {
      const response = await fetch(`http://127.0.0.1:${address.port}`, {
        method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({ jsonrpc: "2.0", id: "receipt", method: "pax_sequencerReceipt", params: [hash] }),
      });
      assert.equal(response.status, 200);
      const message = await response.json() as { readonly jsonrpc: string; readonly id: string; readonly result: Record<string, unknown> | null };
      assert.equal(message.jsonrpc, "2.0"); assert.equal(message.id, "receipt");
      assert(message.result === null || typeof message.result === "object" && !Array.isArray(message.result));
      return message.result;
    },
  };
  const request = { principal: "buyer", requestDigest: id(9), draw: hex(calldata()), idempotencyKey: expected.idempotencyKey, requirements: offer };
  const reopen = (): PreparedGrantDraws => new PreparedGrantDraws(path, expected.recipient, expected.chainId, rpc, webReceiptKeys);
  const stored = () => {
    const db = new DatabaseSync(path);
    try {
      const rows = db.prepare("SELECT * FROM grant_draw_transactions").all(); assert.equal(rows.length, 1);
      const row = rows[0]!;
      assert.equal(row["idempotency_key"], expected.idempotencyKey);
      assert.equal(row["transaction_hash"], hash);
      assert.deepEqual(row["raw_transaction"], raw);
      return row;
    } finally { db.close(); }
  };
  try {
    draws = reopen(); draws.register(request.principal, request.requestDigest, raw, expected);
    if (interruption === "uncertain-response") {
      await assert.rejects(draws.execute(request), (error: unknown) => error instanceof Error && "code" in error && error.code === "ECONNRESET");
      assert.equal(stored()["admission_state"], "unknown");
    } else if (interruption === "receipt-after-restart") {
      assert.deepEqual(await draws.execute(request), { kind: "pending" });
      assert.equal(stored()["admission_state"], "admitted");
    } else {
      await assert.rejects(draws.execute(request), /interrupted-before-send/u);
      assert.equal(stored()["admission_state"], "unknown");
      assert.equal(stored()["attempted"], 1);
      assert.deepEqual(methods, []);
    }
    draws?.close(); draws = undefined;
    draws = reopen(); draws.register(request.principal, request.requestDigest, raw, expected);
    assert.deepEqual(await Promise.all([draws.execute(request), draws.execute(request)]), [{ kind: "pending" }, { kind: "pending" }]);
    assert.equal(stored()["admission_state"], "admitted");
    const delivered = ingress.prepare("SELECT raw FROM deliveries").all();
    assert.equal(delivered.length, interruption === "uncertain-response" ? 2 : 1);
    for (const row of delivered) assert.deepEqual(row["raw"], raw);
    assert.equal(ingress.prepare("SELECT count(*) AS count FROM payments").get()!["count"], 1);
    const sendCount = delivered.length;
    draws.close(); draws = reopen();
    assert.deepEqual(await draws.execute(request), { kind: "pending" });
    assert.equal(ingress.prepare("SELECT count(*) AS count FROM deliveries").get()!["count"], sendCount);
    // A real, independently pinned receipt for another raw transaction never settles the recovered draw.
    const recorded = meteredRules.find(rule => rule.method === "pax_sequencerReceipt" && rule.result !== null);
    assert(recorded);
    receipt = String((recorded.result as { readonly receipt: string }).receipt);
    await assert.rejects(draws.execute(request), (error: unknown) => error instanceof SequencerReceiptVerificationError && error.check === "hash_mismatch");
    assert.equal(stored()["idempotency_key"], expected.idempotencyKey);
    assert.equal(ingress.prepare("SELECT count(*) AS count FROM payments").get()!["count"], 1);
    assert.deepEqual(raw, unhex(recovery.rawTransaction));
    receipt = recovery.receipt;
    draws.close(); draws = reopen();
    const settled = await draws.execute(request);
    assert.equal(settled.kind, "settled");
    if (settled.kind !== "settled") throw new Error("Recovered draw did not settle");
    assert.deepEqual(settled.canonicalReceipt, unhex(recovery.receipt));
    assert.equal(`0x${hex(settled.verifiedReceipt.receipt.activityId)}`, hash);
    assert.equal(stored()["idempotency_key"], expected.idempotencyKey);
    assert.equal((await draws.execute(request)).kind, "settled");
    assert.equal(ingress.prepare("SELECT count(*) AS count FROM payments").get()!["count"], 1);
    assert.equal(ingress.prepare("SELECT count(*) AS count FROM deliveries").get()!["count"], sendCount);
    assert.deepEqual(failures, []);
  } finally {
    draws?.close(); server.closeAllConnections();
    await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    ingress.close(); rmSync(directory, { recursive: true, force: true });
  }
}

async function prepared_draw_reopens_before_send(): Promise<void> { await reopenDrawCase("before-send"); }
async function prepared_draw_reopens_after_uncertain_response(): Promise<void> { await reopenDrawCase("uncertain-response"); }
async function prepared_draw_checks_receipt_arrival_after_restart(): Promise<void> { await reopenDrawCase("receipt-after-restart"); }

async function prepared_draw_settles_recorded_payment_after_restart(): Promise<void> {
  const raw = unhex(recovery.rawTransaction);
  const transaction = decodeLedgerTransaction(raw), data = Buffer.from(transaction.data);
  const word = (index: number): string => data.subarray(4 + index * 32, 36 + index * 32).toString("hex");
  const draw: ExpectedDrawTransaction = { grantId: word(0), recipient: word(1), amount: BigInt(`0x${word(2)}`),
    idempotencyKey: word(3), sequence: BigInt(`0x${word(4)}`), purposeHash: word(5), contextHash: word(6), chainId: transaction.chainId };
  const directory = mkdtempSync(join(tmpdir(), "recorded-draw-reopen-"));
  const recorded = { result: recovery.txHash };
  const response = { params: [recovery.txHash], result: { txHash: recovery.txHash, rung: "receipted", receipt: recovery.receipt } };
  const submissions: string[] = [];
  const methods: string[] = [];
  const failures: unknown[] = [];
  // The loopback replays the producer response; the actual SDK transport is used on both calls.
  const server = createServer(async (incoming, outgoing) => {
    try {
      const chunks: Buffer[] = []; for await (const chunk of incoming) chunks.push(Buffer.from(chunk));
      const message = JSON.parse(Buffer.concat(chunks).toString()) as { readonly id: string; readonly method: string; readonly params: readonly unknown[] };
      assert.equal(incoming.url, "/");
      methods.push(message.method);
      let result: unknown;
      if (message.method === "eth_sendRawTransaction") {
        assert.deepEqual(message.params, [recovery.rawTransaction]);
        assert.equal(submissions.length, 0);
        assert.equal(bindDrawTransaction(unhex(String(message.params[0])), draw), recovery.txHash);
        submissions.push(String(message.params[0]));
        result = recovery.txHash;
      } else {
        assert.equal(message.method, "pax_sequencerReceipt"); assert.deepEqual(message.params, response.params);
        result = response.result;
      }
      outgoing.writeHead(200, { "content-type": "application/json" });
      outgoing.end(JSON.stringify({ jsonrpc: "2.0", id: message.id, result }));
    } catch (error) { failures.push(error); outgoing.writeHead(500); outgoing.end(String(error)); }
  });
  await new Promise<void>(resolve => server.listen(0, "127.0.0.1", resolve));
  const address = server.address(); assert(address !== null && typeof address !== "string");
  const client = new JsonRpcClient(`http://127.0.0.1:${address.port}`);
  const rpc: GrantDrawRpc = { sendRawTransaction: raw => client.sendRawTransaction(unhex(raw)), sequencerReceipt: hash => client.sequencerReceipt(hash) };
  const path = join(directory, "draws.sqlite");
  let draws = new PreparedGrantDraws(path, draw.recipient, draw.chainId, rpc, webReceiptKeys);
  try {
    draws.register("buyer", id(9), raw, draw);
    draws.close(); draws = new PreparedGrantDraws(path, draw.recipient, draw.chainId, rpc, webReceiptKeys);
    const requirements: GrantOffer = { scheme: "metered", amount: draw.amount.toString(), payTo: draw.recipient, asset: offer.asset,
      extra: { layerx: { payer: offer.extra.layerx.payer, grantId: draw.grantId, receiverSequence: draw.sequence.toString(), purposeHash: draw.purposeHash, contextHash: draw.contextHash } } };
    const request = { principal: "buyer", requestDigest: id(9), draw: hex(data), idempotencyKey: draw.idempotencyKey, requirements };
    const settled = await draws.execute(request);
    assert.equal(settled.kind, "settled");
    if (settled.kind !== "settled") throw new Error("Recorded payment did not settle after reopening");
    assert.equal(`0x${hex(settled.verifiedReceipt.receipt.activityId)}`, recorded.result);
    assert.deepEqual(settled.canonicalReceipt, unhex(String((response.result as { readonly receipt: string }).receipt)));
    assert.equal((await draws.execute(request)).kind, "settled");
    assert.deepEqual(submissions, [recovery.rawTransaction]);
    assert.deepEqual(methods, ["eth_sendRawTransaction", "pax_sequencerReceipt", "pax_sequencerReceipt"]);
    assert.deepEqual(failures, []);
  } finally {
    draws.close(); server.closeAllConnections();
    await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    rmSync(directory, { recursive: true, force: true });
  }
}

test(prepared_draw_reopens_before_send);
test(prepared_draw_reopens_after_uncertain_response);
test(prepared_draw_checks_receipt_arrival_after_restart);
test(prepared_draw_settles_recorded_payment_after_restart);
