import assert from "node:assert/strict";
import { stat } from "node:fs/promises";
import test from "node:test";
import { keccak256 } from "@sidiora/layerx-sdk";
import {
  LAYERX_LEDGER_ADDRESS,
  decodePaymentDrawnLog,
  verifyPaymentReceipt,
} from "@sidiora/layerx-seller-middleware";
import {
  CommittedRungRequiresExactPaymentError,
  committedRungOffer,
  runScenarios,
} from "../dist/scenarios.js";
import {
  ConformanceSequencer,
  buildSignedReceipt,
  fixedBytes,
  offerFixture,
  toHex,
} from "../dist/receipts.js";
import { FixedReceiptKeys } from "../dist/support.js";

async function conformancePackageBuildOutputExists() {
  for (const name of ["scenarios", "receipts", "support"]) {
    for (const extension of ["js", "d.ts"]) {
      const output = new URL(`../dist/${name}.${extension}`, import.meta.url);
      const info = await stat(output);
      assert.ok(info.isFile(), `${output.pathname} must be built`);
      assert.ok(info.size > 0, `${output.pathname} must contain build output`);
    }
  }
}

async function committedRungScenarioAcceptsExactPayment() {
  const sequencer = await ConformanceSequencer.generate();
  const facts = {
    asset: fixedBytes(0xbb),
    payTo: fixedBytes(0xaa),
    amount: 250_000n,
    from: fixedBytes(0xcc),
    batchId: fixedBytes(0xd1),
    previousStateRoot: fixedBytes(0xd2),
    resultingStateRoot: fixedBytes(0xd3),
  };
  const offer = offerFixture(facts.payTo, facts.asset, facts.amount);
  const receipt = await buildSignedReceipt(sequencer, facts);
  const keys = await new FixedReceiptKeys([sequencer.receiptKey()]).resolve();
  const verification = await verifyPaymentReceipt(receipt, offer.requirements, keys);
  assert.ok("paymentReference" in verification.payment);
  const committed = committedRungOffer(offer, verification.payment);
  assert.deepEqual(committed.requirements, {
    ...offer.requirements,
    extra: {
      layerx: {
        commitment: "committed",
        purposeHash: toHex(verification.payment.paymentReference),
      },
    },
  });
  assert.deepEqual(committed.paymentRequired, {
    ...offer.paymentRequired,
    accepts: [committed.requirements],
  });
  assert.equal(offer.requirements.extra.layerx.commitment, "receipted");
  await assert.rejects(
    verifyPaymentReceipt(receipt, committed.requirements, keys),
    (error) => error.code === "verification-failure",
  );

  const suite = await runScenarios();
  const checks = suite.results().filter(
    (result) => result.name === "x402: a committed requirement refuses receipted-only evidence",
  );
  assert.equal(checks.length, 1);
  assert.equal(checks[0].ok, true, checks[0].detail);
}

function committedRungScenarioRefusesDrawnPaymentWithNamedError() {
  const asset = fixedBytes(0xbb);
  const payTo = fixedBytes(0xaa);
  const amount = new Uint8Array(32);
  amount[31] = 25;
  const payment = decodePaymentDrawnLog({
    address: LAYERX_LEDGER_ADDRESS,
    topics: [
      keccak256(new TextEncoder().encode(
        "PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)",
      )),
      fixedBytes(0x11),
      fixedBytes(0xcc),
      payTo,
    ],
    data: Uint8Array.from(Buffer.concat([asset, amount, fixedBytes(0x22), fixedBytes(0x33)])),
  });
  assert.equal(payment.amount, 25n);
  assert.ok("grantId" in payment);
  assert.equal("paymentReference" in payment, false);
  const offer = offerFixture(payTo, asset, payment.amount);
  assert.throws(() => committedRungOffer(offer, payment), (error) => {
    assert.ok(error instanceof CommittedRungRequiresExactPaymentError);
    assert.equal(error.name, "CommittedRungRequiresExactPaymentError");
    assert.equal(error.message, "the committed rung requires an exact payment");
    return true;
  });
  assert.equal(offer.requirements.extra.layerx.commitment, "receipted");
}

test(conformancePackageBuildOutputExists);
test(committedRungScenarioAcceptsExactPayment);
test(committedRungScenarioRefusesDrawnPaymentWithNamedError);
