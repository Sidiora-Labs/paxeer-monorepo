import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import { DatabaseSync } from "node:sqlite";
import { test } from "node:test";

import { secp256k1 } from "@noble/curves/secp256k1.js";

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

type RlpValue = Uint8Array | readonly RlpValue[];

function integer(value: bigint): Uint8Array {
  if (value === 0n) return new Uint8Array();
  const text = value.toString(16);
  return bytes(text.padStart(text.length + text.length % 2, "0"));
}

function rlp(value: RlpValue): Uint8Array {
  const list = !(value instanceof Uint8Array);
  const payload = list ? Buffer.concat(value.map(rlp)) : value;
  if (!list && payload.length === 1 && payload[0]! < 128) return payload;
  const prefix = list ? 192 : 128;
  if (payload.length < 56) return Buffer.concat([Buffer.of(prefix + payload.length), payload]);
  const length = integer(BigInt(payload.length));
  return Buffer.concat([Buffer.of(prefix + 55 + length.length), length, payload]);
}

function signedDraw(nonce = 4n, draw: ExpectedDrawTransaction = expected): Uint8Array {
  const transaction: Eip1559Transaction = {
    chainId: draw.chainId, nonce, maxPriorityFeePerGas: 1n, maxFeePerGas: 2n,
    gasLimit: 100000n, to: bytes(LAYERX_LEDGER_PRECOMPILE), value: 0n,
    data: calldata(draw), accessList: [],
  };
  const signature = secp256k1.sign(eip1559SigningHash(transaction), new Uint8Array(32).fill(1),
    { prehash: false, format: "recovered" });
  const parsed = secp256k1.Signature.fromBytes(signature, "recovered");
  return Uint8Array.from(Buffer.concat([Buffer.of(2), rlp([
    integer(transaction.chainId), integer(transaction.nonce), integer(transaction.maxPriorityFeePerGas),
    integer(transaction.maxFeePerGas), integer(transaction.gasLimit), transaction.to,
    integer(transaction.value), transaction.data, [],
    integer(BigInt(signature[0]!)), integer(parsed.r), integer(parsed.s),
  ])]));
}

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
