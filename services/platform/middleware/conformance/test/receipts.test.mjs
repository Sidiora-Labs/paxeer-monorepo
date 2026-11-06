import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import test from "node:test";
import { decodeRawEip1559, keccak256, recoverEip1559Sender, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import {
  LAYERX_LEDGER_ADDRESS,
  MiddlewareError,
  ReceiptPayloadAuthority,
  decodePaymentPayloadHeader,
  decodePaymentRequiredHeader,
  encodePaymentPayloadHeader,
  encodePaymentRequiredHeader,
  verifyPaymentReceipt,
} from "@sidiora/layerx-seller-middleware";
import {
  ConformanceSequencer,
  buildSignedReceipt,
  buyerPayload,
  fixedBytes,
  offerFixture,
} from "../dist/receipts.js";
import { FixedReceiptKeys } from "../dist/support.js";

const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const bytes = (value) => Buffer.from(value.slice(2), "hex");
const sha256 = (value) => createHash("sha256").update(value).digest();

async function fixture(amount = 25n) {
  const sequencer = await ConformanceSequencer.generate();
  const facts = {
    asset: fixedBytes(0x11),
    payTo: fixedBytes(0x22),
    amount,
    from: fixedBytes(0x33),
    batchId: fixedBytes(0x44),
    previousStateRoot: fixedBytes(0x55),
    resultingStateRoot: fixedBytes(0x66),
  };
  const receipt = await buildSignedReceipt(sequencer, facts);
  const offer = offerFixture(facts.payTo, facts.asset, facts.amount);
  const keys = await new FixedReceiptKeys([sequencer.receiptKey()]).resolve();
  return { sequencer, facts, receipt, offer, keys };
}

async function conformanceReceiptVerifiesUnderSellerVerifier() {
  for (const amount of [25n, (1n << 128n) - 1n]) {
    const { facts, receipt, offer, keys, sequencer } = await fixture(amount);
    assert.equal(receipt.format, "layerx-receipt-proof-v2");
    assert.equal(receipt.verificationLevel, "receipted");
    const raw = bytes(receipt.rawTransaction);
    const transaction = decodeRawEip1559(raw);
    assert.equal(raw[0], 2);
    assert.equal(transaction.chainId, 125n);
    assert.equal(hex(transaction.to), LAYERX_LEDGER_ADDRESS);
    assert.equal(hex(transaction.data.slice(0, 4)), hex(keccak256(Buffer.from(
      "payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)",
    )).slice(0, 4)));
    assert.equal(hex(transaction.data.slice(4, 36)), hex(facts.from));
    assert.equal(hex(transaction.data.slice(36, 68)), hex(facts.payTo));
    assert.equal(hex(transaction.data.slice(68, 100)), hex(facts.asset));
    assert.equal(BigInt(hex(transaction.data.slice(100, 132))), amount);
    assert.equal(receipt.txHash, hex(keccak256(raw)));
    const signed = await verifySequencerReceipt(bytes(receipt.signedReceipt), raw, keys);
    assert.equal(receipt.receiptDigest, hex(signed.receiptDigest));
    assert.notEqual(receipt.receiptDigest, hex(keccak256(bytes(receipt.signedReceipt))));

    const header = encodePaymentPayloadHeader(buyerPayload(offer, receipt, "conformance-payment"));
    const payload = decodePaymentPayloadHeader(header);
    const settled = await new ReceiptPayloadAuthority(keys).settle({
      principal: "conformance-seller",
      payload,
      requirements: offer.requirements,
      idempotencyKey: "conformance-payment",
      requestDigest: "conformance-request",
    });
    assert.equal(settled.kind, "settled");
    assert.equal(settled.signedReceipt, receipt.signedReceipt);
    const verified = await verifyPaymentReceipt(payload.payload, offer.requirements, keys);
    assert.equal(verified.rung, "receipted");
    assert.equal(verified.height, 7n);
    assert.equal(verified.txHash, receipt.txHash);
    assert.equal(verified.payment.amount, facts.amount);
    assert.deepEqual(verified.payment.assetId, facts.asset);
    assert.deepEqual(verified.payment.to, facts.payTo);
    assert.deepEqual(verified.payment.from, facts.from);
    assert.equal(hex(verified.payment.paymentReference).slice(2), offer.requirements.extra.layerx.purposeHash);
    assert.equal(hex(verified.receipt.from.slice(12)).toLowerCase(), recoverEip1559Sender(raw).toLowerCase());

    const restored = JSON.parse(JSON.stringify(receipt));
    const reloadedKeys = await new FixedReceiptKeys([sequencer.receiptKey()]).resolve();
    assert.equal((await verifyPaymentReceipt(restored, offer.requirements, reloadedKeys)).txHash, receipt.txHash);
    const other = await ConformanceSequencer.generate();
    await assert.rejects(
      verifyPaymentReceipt(receipt, offer.requirements, await new FixedReceiptKeys([other.receiptKey()]).resolve()),
      (error) => error instanceof MiddlewareError && error.code === "verification-failure",
    );
  }
}

async function conformanceReceiptRefusesTamperedLog() {
  const { facts, receipt, offer, keys } = await fixture();
  assert.equal((await verifyPaymentReceipt(receipt, offer.requirements, keys)).payment.amount, facts.amount);
  const logData = Buffer.concat([Buffer.from(facts.asset), Buffer.from(facts.amount.toString(16).padStart(64, "0"), "hex")]);
  const originalDigest = sha256(logData);
  const changedReceipt = bytes(receipt.signedReceipt);
  const offset = changedReceipt.indexOf(originalDigest);
  assert.ok(offset >= 0, "the signed PaymentExact effect must contain its data hash");
  assert.equal(changedReceipt.indexOf(originalDigest, offset + 1), -1);
  const tamperedData = Buffer.from(logData);
  tamperedData[tamperedData.length - 1] ^= 1;
  assert.equal(logData.reduce((count, byte, index) => count + Number(byte !== tamperedData[index]), 0), 1);
  sha256(tamperedData).copy(changedReceipt, offset);
  const tampered = {
    ...receipt,
    signedReceipt: hex(changedReceipt),
    receiptDigest: hex(keccak256(changedReceipt)),
  };
  const payload = decodePaymentPayloadHeader(encodePaymentPayloadHeader(buyerPayload(offer, tampered, "tampered-payment")));
  await assert.rejects(
    verifyPaymentReceipt(payload.payload, offer.requirements, keys),
    (error) => error instanceof MiddlewareError && error.code === "verification-failure",
  );
}

function offerFixtureUsesEip155Network() {
  const payTo = fixedBytes(0x22);
  const asset = fixedBytes(0x11);
  const offer = offerFixture(payTo, asset, 25n);
  assert.equal(offer.requirements.network, "eip155:125");
  assert.equal(offer.requirements.extra.layerx.commitment, "receipted");
  assert.equal(offer.requirements.amount, "25");
  assert.equal(offer.requirements.asset, hex(asset).slice(2));
  assert.equal(offer.requirements.payTo, hex(payTo).slice(2));
  const transported = decodePaymentRequiredHeader(encodePaymentRequiredHeader(offer.paymentRequired));
  assert.deepEqual(transported.accepts, [offer.requirements]);
}

test(conformanceReceiptVerifiesUnderSellerVerifier);
test(conformanceReceiptRefusesTamperedLog);
test(offerFixtureUsesEip155Network);
