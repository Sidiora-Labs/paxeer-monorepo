import assert from "node:assert/strict";
import test from "node:test";
import { keccak256 } from "@sidiora/layerx-sdk";
import { MiddlewareError } from "../dist/index.js";
import {
  LAYERX_LEDGER_ADDRESS,
  LEDGER_DRAW_SELECTOR,
  encodeLedgerDraw,
  decodePaymentExactLog,
  decodePaymentDrawnLog,
  decodeLedgerTransferLog,
} from "../dist/ledger.js";

const DRAW_SIGNATURE = "draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)";
const EXACT_SIGNATURE = "PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)";
const DRAWN_SIGNATURE = "PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)";
const TRANSFER_SIGNATURE = "Transfer(bytes32,bytes32,bytes32,uint128,uint8)";

function hash(signature) {
  return keccak256(new TextEncoder().encode(signature));
}

function id(seed) {
  return Uint8Array.from({ length: 32 }, (_, index) => (seed + index) & 0xff);
}

function uintWord(value) {
  const word = new Uint8Array(32);
  const hex = value.toString(16).padStart(64, "0");
  for (let index = 0; index < 32; index += 1) {
    word[index] = Number.parseInt(hex.slice(index * 2, index * 2 + 2), 16);
  }
  return word;
}

function eventLog(signature, indexed, words) {
  const data = new Uint8Array(words.length * 32);
  for (const [index, word] of words.entries()) data.set(word, index * 32);
  return { address: LAYERX_LEDGER_ADDRESS, topics: [hash(signature), ...indexed], data };
}

function middlewareError(code) {
  return (error) => error instanceof MiddlewareError && error.code === code;
}

function encodesLedgerDrawCalldataWithDrawSelector() {
  assert.equal(LAYERX_LEDGER_ADDRESS, "0x000000000000000000000000000000000000101a");
  const draw = {
    from: id(1), assetId: id(33), amount: 1_000_000n, grantId: id(65),
    receiverSequence: 0x0102030405060708n, idempotencyKey: id(97), contextHash: id(129),
  };
  const calldata = encodeLedgerDraw(draw);
  assert.ok(calldata instanceof Uint8Array);
  assert.equal(calldata.length, 4 + 7 * 32);
  assert.deepEqual(calldata.slice(0, 4), hash(DRAW_SIGNATURE).slice(0, 4));
  assert.deepEqual(LEDGER_DRAW_SELECTOR, calldata.slice(0, 4));
  assert.equal(Buffer.from(calldata.slice(0, 4)).toString("hex"), "2e89490f");
  const expectedWords = [draw.from, draw.assetId, uintWord(draw.amount), draw.grantId,
    uintWord(draw.receiverSequence), draw.idempotencyKey, draw.contextHash];
  for (const [index, word] of expectedWords.entries()) {
    assert.deepEqual(calldata.slice(4 + index * 32, 4 + (index + 1) * 32), word);
  }
  assert.deepEqual(calldata.slice(4 + 2 * 32, 4 + 3 * 32 - 3), new Uint8Array(29));
  assert.deepEqual(calldata.slice(4 + 3 * 32 - 3, 4 + 3 * 32), Uint8Array.of(0x0f, 0x42, 0x40));
  for (const amount of [0n, (1n << 128n) - 1n]) {
    assert.deepEqual(encodeLedgerDraw({ ...draw, amount }).slice(68, 100), uintWord(amount));
  }
  for (const amount of [-1n, 1n << 128n, 25]) {
    assert.throws(() => encodeLedgerDraw({ ...draw, amount }), middlewareError("invalid-payment-payload"));
  }
  for (const receiverSequence of [-1n, 1n << 64n, 1]) {
    assert.throws(() => encodeLedgerDraw({ ...draw, receiverSequence }), middlewareError("invalid-payment-payload"));
  }
  assert.deepEqual(encodeLedgerDraw({ ...draw, receiverSequence: (1n << 64n) - 1n }).slice(132, 164), uintWord((1n << 64n) - 1n));
  for (const field of ["from", "assetId", "grantId", "idempotencyKey", "contextHash"]) {
    for (const length of [31, 33]) {
      assert.throws(() => encodeLedgerDraw({ ...draw, [field]: new Uint8Array(length) }), middlewareError("invalid-payment-payload"));
    }
  }
}

function decodesPaymentExactLogFromLedgerPrecompile() {
  const paymentReference = id(1), from = id(33), to = id(65), assetId = id(97);
  const exact = eventLog(EXACT_SIGNATURE, [paymentReference, from, to], [assetId, uintWord(25n)]);
  assert.equal(Buffer.from(exact.topics[0]).toString("hex"), "c4da5d6f757c251cd70947b752917e5671b677517e3d953f362a1df6648ef5b1");
  assert.deepEqual(decodePaymentExactLog(exact), { paymentReference, from, to, assetId, amount: 25n });
  assert.deepEqual(decodePaymentExactLog({ ...exact, address: exact.address.toUpperCase().replace("0X", "0x") }),
    { paymentReference, from, to, assetId, amount: 25n });

  const grantId = id(129), idempotencyKey = id(161), contextHash = id(193);
  const drawn = eventLog(DRAWN_SIGNATURE, [grantId, from, to], [assetId, uintWord(25n), idempotencyKey, contextHash]);
  assert.deepEqual(decodePaymentDrawnLog(drawn), { grantId, from, to, assetId, amount: 25n, idempotencyKey, contextHash });
  const transfer = eventLog(TRANSFER_SIGNATURE, [from, to, assetId], [uintWord(25n), uintWord(2n)]);
  assert.deepEqual(decodeLedgerTransferLog(transfer), { from, to, assetId, amount: 25n, kind: 2 });
  for (const amount of [0n, (1n << 128n) - 1n]) {
    assert.equal(decodePaymentExactLog(eventLog(EXACT_SIGNATURE, [paymentReference, from, to], [assetId, uintWord(amount)])).amount, amount);
  }
}

function refusesLedgerLogFromAnotherAddress() {
  const indexed = [id(1), id(33), id(65)];
  const cases = [
    [decodePaymentExactLog, eventLog(EXACT_SIGNATURE, indexed, [id(97), uintWord(25n)]), 32],
    [decodePaymentDrawnLog, eventLog(DRAWN_SIGNATURE, indexed, [id(97), uintWord(25n), id(129), id(161)]), 32],
    [decodeLedgerTransferLog, eventLog(TRANSFER_SIGNATURE, indexed, [uintWord(25n), uintWord(2n)]), 0],
  ];
  for (const [decode, log, amountOffset] of cases) {
    for (const address of ["0x000000000000000000000000000000000000101b", "0x101a", ""]) {
      assert.throws(() => decode({ ...log, address }), middlewareError("verification-failure"));
    }
    for (const signature of [EXACT_SIGNATURE, DRAWN_SIGNATURE, TRANSFER_SIGNATURE, "Unknown(bytes32)"]) {
      const topic = hash(signature);
      if (Buffer.from(topic).equals(Buffer.from(log.topics[0]))) continue;
      assert.throws(() => decode({ ...log, topics: [topic, ...indexed] }), middlewareError("verification-failure"));
    }
    for (const topics of [[], log.topics.slice(0, 3), [...log.topics, id(193)]]) {
      assert.throws(() => decode({ ...log, topics }), middlewareError("verification-failure"));
    }
    for (let index = 0; index < 4; index += 1) {
      for (const length of [31, 33]) {
        const topics = log.topics.slice();
        topics[index] = new Uint8Array(length);
        assert.throws(() => decode({ ...log, topics }), middlewareError("verification-failure"));
      }
    }
    for (const data of [log.data.slice(0, -1), new Uint8Array(log.data.length + 32)]) {
      assert.throws(() => decode({ ...log, data }), middlewareError("verification-failure"));
    }
    const overflow = log.data.slice();
    overflow[amountOffset + 15] = 1;
    assert.throws(() => decode({ ...log, data: overflow }), middlewareError("verification-failure"));
  }
  const transfer = cases[2][1];
  const invalidKind = transfer.data.slice();
  invalidKind[62] = 1;
  assert.throws(() => decodeLedgerTransferLog({ ...transfer, data: invalidKind }), middlewareError("verification-failure"));
}

test(encodesLedgerDrawCalldataWithDrawSelector);
test(decodesPaymentExactLogFromLedgerPrecompile);
test(refusesLedgerLogFromAnotherAddress);
