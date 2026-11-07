import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer } from "node:http";
import test from "node:test";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import {
  checksumAddress,
  decodeGrantDraw,
  decodeRawEip1559,
  eip1559SigningHash,
  encodeUnsignedEip1559,
  keccak256,
  recoverEip1559Sender,
  validateGrantDraw,
} from "@sidiora/layerx-sdk";
import {
  encodeLedgerDraw,
  LAYERX_LEDGER_ADDRESS,
  MiddlewareError,
  PaymentRpc,
  PaymentRpcError,
} from "../dist/index.js";

const word = (value) => new Uint8Array(32).fill(value);
const hex = (value) => Buffer.from(value).toString("hex");
const bytes = (value) => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
const payer = word(0x11);
const asset = word(0x22);
const draw = {
  grantId: word(0x33),
  recipient: word(0x44),
  amount: 25n,
  idempotencyKey: word(0x55),
  sequence: 0x0102030405060708n,
  purposeHash: word(0x66),
  contextHash: word(0x77),
};

function offer(scheme) {
  return {
    scheme,
    asset: hex(asset),
    amount: draw.amount.toString(),
    payTo: hex(draw.recipient),
    extra: { layerx: {
      payer: hex(payer),
      grantId: hex(draw.grantId),
      receiverSequence: draw.sequence.toString(),
      purposeHash: hex(draw.purposeHash),
      contextHash: hex(draw.contextHash),
      ...(scheme === "subscription" ? { windowSeconds: "60" } : {}),
    } },
  };
}

function integer(value) {
  if (value === 0n) return new Uint8Array();
  const digits = value.toString(16);
  return Buffer.from(digits.padStart(digits.length + digits.length % 2, "0"), "hex");
}

function rlp(value) {
  const list = Array.isArray(value);
  const payload = list ? Buffer.concat(value.map(rlp)) : value;
  if (!list && payload.length === 1 && payload[0] < 128) return payload;
  const prefix = list ? 192 : 128;
  if (payload.length < 56) return Buffer.concat([Buffer.of(prefix + payload.length), payload]);
  const length = integer(BigInt(payload.length));
  return Buffer.concat([Buffer.of(prefix + 55 + length.length), length, payload]);
}

function signedDraw(value) {
  const transaction = {
    chainId: 125n, nonce: 7n, maxPriorityFeePerGas: 1n, maxFeePerGas: 2n,
    gasLimit: 200_000n, to: bytes(LAYERX_LEDGER_ADDRESS), value: 0n,
    data: encodeLedgerDraw(value), accessList: [],
  };
  const fields = [
    ...[transaction.chainId, transaction.nonce, transaction.maxPriorityFeePerGas,
      transaction.maxFeePerGas, transaction.gasLimit].map(integer),
    transaction.to, integer(transaction.value), transaction.data, transaction.accessList,
  ];
  assert.deepEqual(encodeUnsignedEip1559(transaction),
    Uint8Array.from(Buffer.concat([Buffer.of(2), rlp(fields)])));
  const signature = secp256k1.sign(eip1559SigningHash(transaction), word(1),
    { prehash: false, format: "recovered" });
  const parsed = secp256k1.Signature.fromBytes(signature, "recovered");
  return `0x${hex(Buffer.concat([Buffer.of(2), rlp([
    ...fields, integer(BigInt(signature[0])), integer(parsed.r), integer(parsed.s),
  ])]))}`;
}

async function legacyDrawFieldNamesAcrossTransport(t) {
  const accepted = new Map();
  const requests = [];
  let terms;
  const server = createServer(async (request, response) => {
    let body;
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      requests.push(body);
      assert.equal(request.method, "POST");
      assert.equal(request.url, "/");
      assert.equal(body.jsonrpc, "2.0");
      assert.equal(body.method, "eth_sendRawTransaction");
      assert.equal(body.params.length, 1);
      const raw = bytes(body.params[0]);
      const transaction = decodeRawEip1559(raw);
      assert.equal(transaction.chainId, 125n);
      assert.equal(transaction.value, 0n);
      assert.equal(`0x${hex(transaction.to)}`, LAYERX_LEDGER_ADDRESS);
      assert.equal(recoverEip1559Sender(raw),
        checksumAddress(bytes("0x1a642f0e3c3af545e7acbd38b07251b3990914f1")));
      const decoded = validateGrantDraw(transaction.data, terms, hex(draw.idempotencyKey));
      const txHash = `0x${hex(keccak256(raw))}`;
      accepted.set(txHash, decoded);
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: "2.0", id: body.id, result: txHash }));
    } catch (error) {
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: "2.0", id: body?.id,
        error: { code: -32602, message: String(error) } }));
    }
  });
  t.after(async () => {
    const closed = new Promise((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
    });
    server.closeAllConnections();
    await closed;
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const rpc = new PaymentRpc(`http://127.0.0.1:${server.address().port}/`);
  assert.equal(new Set([payer, draw.recipient, asset, draw.purposeHash].map(hex)).size, 4);
  for (const scheme of ["metered", "subscription"]) {
    terms = offer(scheme);
    const raw = signedDraw(draw);
    const txHash = await rpc.send(raw);
    assert.equal(txHash, `0x${hex(keccak256(bytes(raw)))}`);
    assert.deepEqual(accepted.get(txHash), {
      grantId: hex(draw.grantId), recipient: hex(draw.recipient), amount: draw.amount,
      idempotencyKey: hex(draw.idempotencyKey), sequence: draw.sequence,
      purposeHash: hex(draw.purposeHash), contextHash: hex(draw.contextHash),
    });
    for (const changes of [
      { recipient: payer }, { purposeHash: asset }, { sequence: draw.sequence + 1n },
      { grantId: word(0x88) }, { idempotencyKey: word(0x88) },
      { contextHash: word(0x88) }, { amount: draw.amount + 1n },
    ]) {
      await assert.rejects(rpc.send(signedDraw({ ...draw, ...changes })),
        (error) => error instanceof PaymentRpcError && error.code === -32602
          && error.message === "JSON-RPC error -32602");
    }
    assert.equal(accepted.size, 1);
  }
  assert.equal(requests.length, 16);
}

function canonicalLedgerDrawRejectsLegacyAliasesAndInvalidWords() {
  const invalid = (error) => error instanceof MiddlewareError && error.code === "invalid-payment-payload";
  assert.throws(() => encodeLedgerDraw({
    grantId: draw.grantId, from: draw.recipient, assetId: draw.purposeHash,
    amount: draw.amount, receiverSequence: draw.sequence,
    idempotencyKey: draw.idempotencyKey, contextHash: draw.contextHash,
  }), invalid);
  for (const field of ["grantId", "recipient", "idempotencyKey", "purposeHash", "contextHash"]) {
    for (const value of [undefined, "11".repeat(32), new Uint8Array(31), new Uint8Array(33)]) {
      assert.throws(() => encodeLedgerDraw({ ...draw, [field]: value }), invalid);
    }
  }
  for (const [field, bits] of [["amount", 128n], ["sequence", 64n]]) {
    for (const value of [undefined, -1n, 1n << bits, 1, "1"]) {
      assert.throws(() => encodeLedgerDraw({ ...draw, [field]: value }), invalid);
    }
  }
  for (const value of [null, undefined, 1]) assert.throws(() => encodeLedgerDraw(value), invalid);
  const maximum = { ...draw, amount: (1n << 128n) - 1n, sequence: (1n << 64n) - 1n };
  const decoded = decodeGrantDraw(encodeLedgerDraw(maximum));
  assert.equal(decoded.amount, maximum.amount);
  assert.equal(decoded.sequence, maximum.sequence);
  const zero = decodeGrantDraw(encodeLedgerDraw({ ...draw, sequence: 0n }));
  assert.equal(zero.sequence, 0n);
  assert.deepEqual(encodeLedgerDraw({
    ...draw, from: payer, assetId: asset, receiverSequence: 0n,
  }), encodeLedgerDraw(draw));
}

test(legacyDrawFieldNamesAcrossTransport);
test(canonicalLedgerDrawRejectsLegacyAliasesAndInvalidWords);
