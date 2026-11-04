import assert from "node:assert/strict";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import test from "node:test";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { keccak_256 } from "@noble/hashes/sha3.js";
import {
  decodeRawEip1559,
  eip1559SigningHash,
  encodeUnsignedEip1559,
  keccak256,
  recoverEip1559Sender,
} from "@sidiora/layerx-sdk";
import {
  LAYERX_LEDGER_ADDRESS,
  decodePaymentDrawnLog,
  encodeLedgerDraw,
} from "../../seller/dist/ledger.js";
import { PaymentRpc } from "../../seller/dist/rpc.js";

const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const bytes = (value) => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
// precompiles/layerxledger/abi.json: draw defines calldata order;
// PaymentDrawn defines indexed topics and the separate event data order.
const ledgerAbi = JSON.parse(readFileSync(new URL("../../../../../precompiles/layerxledger/abi.json", import.meta.url), "utf8"));
const drawAbi = ledgerAbi.find((entry) => entry.type === "function" && entry.name === "draw");
const paymentDrawnAbi = ledgerAbi.find((entry) => entry.type === "event" && entry.name === "PaymentDrawn");
const abiSignature = (entry) => `${entry.name}(${entry.inputs.map((input) => input.type).join(",")})`;
const signingKey = new Uint8Array(32).fill(1);
const executorAccount = bytes("0x0000000000000000000000001a642f0e3c3af545e7acbd38b07251b3990914f1");
const draw = {
  recipient: new Uint8Array(32).fill(0x11),
  purposeHash: new Uint8Array(32).fill(0x22),
  amount: 40n,
  grantId: new Uint8Array(32).fill(0x33),
  sequence: 0x0102030405060708n,
  idempotencyKey: new Uint8Array(32).fill(0x44),
  contextHash: new Uint8Array(32).fill(0x55),
};

function integer(value) {
  if (value === 0n) return new Uint8Array();
  const digits = value.toString(16);
  return bytes(`0x${digits.padStart(digits.length + digits.length % 2, "0")}`);
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

export function signedDrawFixture() {
  const transaction = {
    chainId: 125n,
    nonce: 7n,
    maxPriorityFeePerGas: 1n,
    maxFeePerGas: 2n,
    gasLimit: 200_000n,
    to: bytes(LAYERX_LEDGER_ADDRESS),
    value: 0n,
    data: encodeLedgerDraw(draw),
    accessList: [],
  };
  const fields = [
    integer(transaction.chainId), integer(transaction.nonce), integer(transaction.maxPriorityFeePerGas),
    integer(transaction.maxFeePerGas), integer(transaction.gasLimit), transaction.to,
    integer(transaction.value), transaction.data, transaction.accessList,
  ];
  assert.deepEqual(encodeUnsignedEip1559(transaction),
    Uint8Array.from(Buffer.concat([Buffer.of(2), rlp(fields)])));
  const signature = secp256k1.sign(eip1559SigningHash(transaction), signingKey,
    { prehash: false, format: "recovered" });
  const parsed = secp256k1.Signature.fromBytes(signature, "recovered");
  const raw = Uint8Array.from(Buffer.concat([Buffer.of(2), rlp([
    ...fields, integer(BigInt(signature[0])), integer(parsed.r), integer(parsed.s),
  ])]));
  return { transaction, raw, rawTransaction: hex(raw), txHash: hex(keccak256(raw)) };
}

function ledgerDrawTransactionBindsGrantTerms() {
  const fixture = signedDrawFixture();
  const decoded = decodeRawEip1559(fixture.raw);
  const { yParity, r, s, ...transaction } = decoded;
  assert.deepEqual(transaction, fixture.transaction);
  assert.equal(hex(decoded.to), "0x000000000000000000000000000000000000101a");
  assert.equal(hex(decoded.data.slice(0, 4)), "0x2e89490f");
  assert.equal(decoded.data.length, 4 + 7 * 32);
  const word = (index) => decoded.data.slice(4 + index * 32, 4 + (index + 1) * 32);
  assert.deepEqual(drawAbi.inputs.map((input) => [input.name, input.type]), [
    ["grantId", "bytes32"], ["recipient", "bytes32"], ["amount", "uint128"],
    ["idempotencyKey", "bytes32"], ["sequence", "uint64"], ["purposeHash", "bytes32"], ["contextHash", "bytes32"],
  ]);
  const values = { grantId: draw.grantId, recipient: draw.recipient, amount: draw.amount,
    idempotencyKey: draw.idempotencyKey, sequence: draw.sequence, purposeHash: draw.purposeHash,
    contextHash: draw.contextHash };
  assert.deepEqual(decoded.data.slice(0, 4), keccak256(new TextEncoder().encode(abiSignature(drawAbi))).slice(0, 4));
  for (const [index, input] of drawAbi.inputs.entries()) {
    if (input.type.startsWith("uint")) assert.equal(BigInt(hex(word(index))), values[input.name]);
    else assert.deepEqual(word(index), values[input.name]);
  }

  const sender = recoverEip1559Sender(fixture.raw);
  assert.equal(sender.toLowerCase(), "0x1a642f0e3c3af545e7acbd38b07251b3990914f1");
  const recipient = new Uint8Array(32);
  recipient.set(bytes(sender), 12);
  assert.deepEqual(recipient, executorAccount);
  assert.notDeepEqual(recipient, draw.recipient);
  assert.ok(secp256k1.verify(new secp256k1.Signature(r, s, yParity).toBytes(),
    eip1559SigningHash(decoded), secp256k1.getPublicKey(signingKey), { prehash: false }));

  const asset = new Uint8Array(32).fill(0x66);
  const grantSource = new Uint8Array(32).fill(0x77);
  // Decoder vector only; native grant execution needs the recorded wiring request.
  const eventValues = { grantId: word(0), from: grantSource, to: word(1), assetId: asset,
    amount: word(2), idempotencyKey: word(3), contextHash: word(6) };
  assert.deepEqual(paymentDrawnAbi.inputs.filter((input) => input.indexed).map((input) => input.name),
    ["grantId", "from", "to"]);
  assert.deepEqual(paymentDrawnAbi.inputs.filter((input) => !input.indexed).map((input) => input.name),
    ["assetId", "amount", "idempotencyKey", "contextHash"]);
  const log = {
    address: hex(decoded.to),
    topics: [keccak256(new TextEncoder().encode(abiSignature(paymentDrawnAbi))),
      ...paymentDrawnAbi.inputs.filter((input) => input.indexed).map((input) => eventValues[input.name])],
    data: Uint8Array.from(Buffer.concat(paymentDrawnAbi.inputs.filter((input) => !input.indexed)
      .map((input) => eventValues[input.name]))),
  };
  const payment = decodePaymentDrawnLog(log);
  assert.deepEqual(payment, {
    grantId: draw.grantId, from: grantSource, to: draw.recipient, assetId: asset,
    amount: draw.amount, idempotencyKey: draw.idempotencyKey, contextHash: draw.contextHash,
  });
  assert.notDeepEqual(payment.assetId, word(5));
  assert.notDeepEqual(payment.from, payment.to);
  assert.notDeepEqual(payment.from, executorAccount);
  assert.notDeepEqual(payment.to, executorAccount);
  const offer = { scheme: "metered", network: "eip155:125", amount: "40", asset: hex(asset), payTo: hex(draw.recipient) };
  assert.equal(hex(payment.to), offer.payTo);
  assert.equal(hex(payment.assetId), offer.asset);
  assert.equal(payment.amount.toString(), offer.amount);
  const invalidAmount = log.data.slice();
  invalidAmount[32] = 1;
  for (const invalid of [{ ...log, data: invalidAmount }, { ...log, data: log.data.slice(0, -1) },
    { ...log, topics: log.topics.slice(0, -1) }]) {
    assert.throws(() => decodePaymentDrawnLog(invalid), (error) => error.code === "verification-failure");
  }
  log.data.fill(0);
  log.topics[1].fill(0);
  assert.deepEqual(payment.grantId, draw.grantId);
  assert.deepEqual(payment.assetId, asset);
  assert.equal(fixture.txHash, hex(keccak_256(bytes(fixture.rawTransaction))));
  assert.notEqual(fixture.txHash, hex(eip1559SigningHash(decoded)));
}

async function paymentRpcSubmitsLedgerDrawOnce(context) {
  const fixture = signedDrawFixture();
  const requests = [];
  const submitted = { rung: "submitted", txHash: fixture.txHash, nonce: "0x7" };
  const server = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      requests.push(body);
      assert.equal(request.method, "POST");
      assert.equal(request.url, "/");
      assert.equal(request.headers["content-type"], "application/json");
      assert.equal(body.jsonrpc, "2.0");
      let result;
      if (requests.length === 1) {
        assert.equal(body.method, "eth_sendRawTransaction");
        assert.deepEqual(body.params, [fixture.rawTransaction]);
        result = hex(keccak_256(bytes(body.params[0])));
      } else {
        assert.equal(requests.length, 2);
        assert.equal(body.method, "pax_sequencerReceipt");
        assert.deepEqual(body.params, [fixture.txHash]);
        result = submitted;
      }
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: "2.0", id: body.id, result }));
    } catch (error) {
      response.writeHead(500);
      response.end(String(error));
    }
  });
  context.after(async () => {
    const closed = new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    server.closeAllConnections();
    await closed;
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const rpc = new PaymentRpc(`http://127.0.0.1:${server.address().port}/`);
  const txHash = await rpc.send(fixture.rawTransaction);
  assert.equal(txHash, fixture.txHash);
  assert.deepEqual(await rpc.receipt(txHash), submitted);
  assert.deepEqual(requests, [
    { jsonrpc: "2.0", id: 1, method: "eth_sendRawTransaction", params: [fixture.rawTransaction] },
    { jsonrpc: "2.0", id: 2, method: "pax_sequencerReceipt", params: [txHash] },
  ]);
  assert.equal(requests.filter((request) => request.method === "eth_sendRawTransaction").length, 1);
}

test(ledgerDrawTransactionBindsGrantTerms);
test(paymentRpcSubmitsLedgerDrawOnce);
