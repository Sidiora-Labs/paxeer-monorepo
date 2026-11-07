import assert from "node:assert/strict";
import { mkdtemp, readdir, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { verifySequencerReceipt } from "@sidiora/layerx-sdk";
import { MerchantError, MerchantMiddleware } from "../dist/index.js";
import {
  ReceiptPayloadAuthority,
  SellerMiddleware,
  encodePaymentPayloadHeader,
} from "@sidiora/layerx-seller-middleware";
import {
  DeclaredCatalog,
  FileFulfillments,
  FileOrders,
  MerchantCheckout,
} from "../../../examples/support/merchant-app.mjs";
import {
  ConformanceSequencer,
  buildSignedReceipt,
  buyerPayload,
  fixedBytes,
  toHex,
} from "../../conformance/src/receipts.ts";
import { FixedReceiptKeys } from "../../conformance/src/support.ts";

async function checkoutFixture(context) {
  const directory = await mkdtemp(join(tmpdir(), "merchant-repair-22-31-525-"));
  context.after(() => rm(directory, { recursive: true, force: true }));
  const sequencer = await ConformanceSequencer.generate();
  const keys = await new FixedReceiptKeys([sequencer.receiptKey()]).resolve();
  const asset = fixedBytes(0xbb);
  const payTo = fixedBytes(0xaa);
  const item = {
    sku: "report",
    title: "Paid report",
    unitAmount: "250000",
    asset: toHex(asset),
    payTo: toHex(payTo),
    scheme: "exact",
    network: "eip155:125",
    maxTimeoutSeconds: 120,
    extra: { layerx: { commitment: "receipted" } },
  };
  const ordersDirectory = join(directory, "orders");
  const fulfillmentsDirectory = join(directory, "fulfillments");
  const create = () => new MerchantCheckout({
    catalog: new DeclaredCatalog(item),
    orders: new FileOrders(ordersDirectory),
    sellers: {
      create: (paymentRequired) => new SellerMiddleware({
        paymentRequired,
        receiptKeys: keys,
        authority: new ReceiptPayloadAuthority(keys),
        fulfillments: new FileFulfillments(fulfillmentsDirectory),
      }),
    },
    resourceUrl: (checkoutKey) => `https://merchant.example/orders/${checkoutKey}`,
  });
  const merchant = create();
  assert.ok(merchant instanceof MerchantMiddleware);
  return { merchant, create, sequencer, keys, asset, payTo, ordersDirectory, fulfillmentsDirectory };
}

async function paidCheckoutFixture(context) {
  const fixture = await checkoutFixture(context);
  const lines = [{ sku: "report", quantity: 1 }];
  const opened = await fixture.merchant.checkout("merchant", "proof", lines);
  assert.equal(opened.kind, "payment-required");
  assert.equal(opened.order.state, "awaiting-payment");
  const paymentRequired = opened.order.quote.paymentRequired;
  const requirements = paymentRequired.accepts[0];
  const signed = await buildSignedReceipt(fixture.sequencer, {
    asset: fixture.asset,
    payTo: fixture.payTo,
    amount: 250_000n,
    from: fixedBytes(0xcc),
    batchId: fixedBytes(0xd1),
    previousStateRoot: fixedBytes(0xd2),
    resultingStateRoot: fixedBytes(0xd3),
    purposeHash: requirements.extra.layerx.purposeHash,
  });
  const verified = await verifySequencerReceipt(
    Uint8Array.from(Buffer.from(signed.signedReceipt.slice(2), "hex")),
    Uint8Array.from(Buffer.from(signed.rawTransaction.slice(2), "hex")),
    fixture.keys,
  );
  const receipt = { ...signed, receiptDigest: `0x${toHex(verified.receiptDigest)}` };
  const payload = buyerPayload({ paymentRequired, requirements }, receipt, "merchant-proof");
  const header = encodePaymentPayloadHeader(payload);
  const paid = await fixture.merchant.checkout("merchant", "proof", lines, header);
  assert.equal(paid.kind, "paid");
  assert.equal(paid.order.state, "paid-verified");
  assert.equal(paid.order.receiptDigest, receipt.receiptDigest.slice(2));
  assert.equal(paid.order.transaction, receipt.txHash);
  assert.equal(paid.settlement.transaction, receipt.txHash);
  assert.deepEqual(await new FileOrders(fixture.ordersDirectory).get("proof"), paid.order);
  const files = await readdir(fixture.fulfillmentsDirectory);
  assert.equal(files.length, 1);
  const fulfillment = JSON.parse(await readFile(join(fixture.fulfillmentsDirectory, files[0]), "utf8"));
  assert.equal(fulfillment.txHash, receipt.txHash);
  assert.equal(fulfillment.receiptDigest, receipt.receiptDigest);
  return { ...fixture, lines, receipt, payload, header, paid, files, fulfillment };
}

async function repair2231525PaysWithSignedReceipt(context) {
  await paidCheckoutFixture(context);
}

async function repair2231525ReplaysPaidOrderAfterReopening(context) {
  const fixture = await paidCheckoutFixture(context);
  const reopened = fixture.create();
  const replay = await reopened.checkout("merchant", "proof", fixture.lines, fixture.header);
  assert.equal(replay.kind, "paid");
  assert.deepEqual(replay, fixture.paid);
  assert.deepEqual(await new FileOrders(fixture.ordersDirectory).get("proof"), fixture.paid.order);
  assert.deepEqual(await readdir(fixture.fulfillmentsDirectory), fixture.files);
}

async function repair2231525RefusesTamperedAttemptWithoutChangingPaidOrder(context) {
  const fixture = await paidCheckoutFixture(context);
  const reopened = fixture.create();
  const orderFiles = await readdir(fixture.ordersDirectory);
  const orderContents = await Promise.all(orderFiles.map((file) =>
    readFile(join(fixture.ordersDirectory, file), "utf8")));
  const tampered = structuredClone(fixture.payload);
  tampered.payload.receiptDigest = `0x${"00".repeat(32)}`;
  assert.notEqual(tampered.payload.receiptDigest, fixture.receipt.receiptDigest);
  const result = await reopened.checkout(
    "merchant", "proof", fixture.lines, encodePaymentPayloadHeader(tampered),
  );
  assert.equal(result.kind, "refused");
  assert.deepEqual(result.order, fixture.paid.order);
  assert.equal(result.order.state, "paid-verified");
  assert.deepEqual(await new FileOrders(fixture.ordersDirectory).get("proof"), fixture.paid.order);
  assert.deepEqual(await readdir(fixture.ordersDirectory), orderFiles);
  assert.deepEqual(await Promise.all(orderFiles.map((file) =>
    readFile(join(fixture.ordersDirectory, file), "utf8"))), orderContents);
  assert.deepEqual(await readdir(fixture.fulfillmentsDirectory), fixture.files);
  assert.deepEqual(JSON.parse(await readFile(
    join(fixture.fulfillmentsDirectory, fixture.files[0]), "utf8",
  )), fixture.fulfillment);
}

async function repair2231525RejectsMissingSkuOnFreshCheckout(context) {
  const fixture = await checkoutFixture(context);
  await assert.rejects(
    fixture.merchant.checkout("merchant", "unknown", [{ sku: "missing", quantity: 1 }]),
    (error) => error instanceof MerchantError && error.code === "catalog-item-missing",
  );
  assert.equal(await new FileOrders(fixture.ordersDirectory).get("unknown"), undefined);
}

test(repair2231525PaysWithSignedReceipt);
test(repair2231525ReplaysPaidOrderAfterReopening);
test(repair2231525RefusesTamperedAttemptWithoutChangingPaidOrder);
test(repair2231525RejectsMissingSkuOnFreshCheckout);
