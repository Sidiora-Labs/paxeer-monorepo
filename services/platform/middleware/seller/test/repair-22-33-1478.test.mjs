import "./source-loader.mjs";
import assert from "node:assert/strict";
import { once } from "node:events";
import { mkdtemp, readdir, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
const {
  ReceiptPayloadAuthority, SellerMiddleware, decodePaymentRequiredHeader,
  decodeSettlementHeader, encodePaymentPayloadHeader, verifyPaymentReceipt,
} = await import("../src/index.ts");
const { FileFulfillmentRepository, createPaidApiServer } = await import("../../../examples/paid-api/index.mjs");
const { committedPaymentFixture, refusesUnboundCommitments } = await import("./repair-22-33-1478-support.mjs");
const { keccak256, SequencerReceiptVerificationError, verifySequencerReceipt } = await import("@sidiora/layerx-sdk");
await import("./commitment-rungs.test.mjs");
await import("./grant-draw.test.mjs");
await import("./payment-rpc.test.mjs");
await import("./proof_middleware_seller_ts_exports.test.mjs");
await import("./receipt-replay.test.mjs");
await import("./settlement-response.test.mjs");

async function recordedNodePaymentCrossesExampleAndRecovers(t) {
  const { proof, requirements, keys, evidence, resolver, vector } = await committedPaymentFixture();
  const recordedVotes = vector.receiptInclusion.NextCommit.Signatures;
  assert.ok(recordedVotes.some((vote) => vote.BlockIDFlag === 1 && vote.Signature === null));
  for (const [index, vote] of recordedVotes.entries()) {
    const decoded = evidence.commit.signatures[index];
    assert.equal(decoded.blockIdFlag, vote.BlockIDFlag);
    assert.deepEqual(decoded.validatorAddress, new Uint8Array(Buffer.from(vote.ValidatorAddress, "base64")));
    assert.deepEqual(decoded.signature, vote.Signature === null
      ? new Uint8Array() : new Uint8Array(Buffer.from(vote.Signature, "base64")));
  }
  const directory = await mkdtemp(join(tmpdir(), "seller-repair-1478-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const paymentRequired = { x402Version: 2, resource: { url: "https://merchant.example/paid" }, accepts: [requirements] };
  const header = encodePaymentPayloadHeader({ x402Version: 2, accepted: requirements, payload: proof, resource: paymentRequired.resource });
  const resource = { report: "recorded-node-payment" };
  class RecordedPaymentMiddleware extends SellerMiddleware {
    async handle(...args) {
      try {
        return await super.handle(...args);
      } catch (error) {
        const causes = [];
        for (let inner = error; inner instanceof Error; inner = inner.cause) {
          causes.push({ name: inner.name, code: inner.code, check: inner.check, message: inner.message });
        }
        t.diagnostic(JSON.stringify({ recordedPaymentFailure: causes }));
        throw error;
      }
    }
  }
  let server;
  let original;
  t.after(async () => { if (server?.listening) await stop(); });
  const stop = async () => {
    const closing = new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    server.closeAllConnections(); await closing;
  };
  for (const reopened of [false, true]) {
    const commitments = reopened ? {
      resolveValidators: (...args) => resolver.resolveValidators(...args),
      async resolve() { assert.fail("recovery must consume persisted node inclusion evidence"); },
    } : resolver;
    const middleware = new RecordedPaymentMiddleware({ paymentRequired, receiptKeys: keys, commitments,
      authority: new ReceiptPayloadAuthority(keys), fulfillments: new FileFulfillmentRepository(directory) });
    server = createPaidApiServer(middleware, resource);
    server.listen(0, "127.0.0.1"); await once(server, "listening");
    const endpoint = `http://127.0.0.1:${server.address().port}/paid`;
    const challenge = await fetch(endpoint);
    assert.equal(challenge.status, 402);
    assert.deepEqual(decodePaymentRequiredHeader(challenge.headers.get("PAYMENT-REQUIRED")), paymentRequired);
    assert.deepEqual(await challenge.json(), paymentRequired);
    const response = await fetch(endpoint, { headers: { "PAYMENT-SIGNATURE": header } });
    assert.equal(response.status, 200);
    assert.deepEqual(await response.json(), resource);
    const settlement = decodeSettlementHeader(response.headers.get("PAYMENT-RESPONSE"));
    assert.equal(settlement.success, true);
    assert.equal(settlement.transaction, vector.receiptJson.txHash);
    assert.equal(settlement.amount, requirements.amount);
    assert.equal(settlement.network, "eip155:125");
    assert.equal(settlement.payer, vector.receiptJson.from.slice(2).padStart(64, "0"));
    const portable = settlement.extensions.layerx;
    assert.equal(portable.landingBlockHeight, `0x${evidence.landingBlockHeight.toString(16)}`);
    assert.equal(portable.landingBlockHash, `0x${Buffer.from(evidence.blockHeader.blockId.hash).toString("hex")}`);
    for (const field of ["blockHeader", "receiptInclusion", "commit", "validatorSet"]) {
      assert.equal(Object.hasOwn(portable, field), false);
    }
    const verified = await verifyPaymentReceipt(portable, requirements, keys, resolver);
    assert.equal(verified.rung, "committed");
    assert.equal(verified.txHash, settlement.transaction);
    if (reopened) assert.deepEqual(settlement, original);
    else original = settlement;
    const files = await readdir(directory);
    assert.equal(files.length, 1);
    const disk = JSON.parse(await readFile(join(directory, files[0]), "utf8"));
    const recovered = await new FileFulfillmentRepository(directory).get(files[0].slice(0, -5));
    assert.equal(disk.receiptDigest, proof.receiptDigest);
    assert.deepEqual(recovered.commitmentEvidence, evidence);
    assert.deepEqual(recovered.resource, resource);
    await stop();
  }
  await refusesUnboundCommitments();
}

test(recordedNodePaymentCrossesExampleAndRecovers);

async function receiptAuthorityPreservesSdkRefusal(t) {
  const { proof, requirements, keys, resolver, evidence, vector } = await committedPaymentFixture();
  const signedReceipt = new Uint8Array(Buffer.from(proof.signedReceipt.slice(2), "hex"));
  signedReceipt[signedReceipt.length - 1] ^= 1;
  const raw = new Uint8Array(Buffer.from(proof.rawTransaction.slice(2), "hex"));
  let sdkError;
  await assert.rejects(verifySequencerReceipt(signedReceipt, raw, keys), (error) => {
    assert.ok(error instanceof SequencerReceiptVerificationError);
    assert.equal(error.code, "verification-failure");
    assert.equal(error.check, "bad_signature");
    sdkError = error;
    return true;
  });
  const invalid = { ...proof, signedReceipt: `0x${Buffer.from(signedReceipt).toString("hex")}`,
    receiptDigest: `0x${Buffer.from(keccak256(signedReceipt)).toString("hex")}` };
  const request = { principal: "public-paid-api", requirements, idempotencyKey: "11".repeat(32),
    requestDigest: "22".repeat(32), payload: { x402Version: 2, accepted: requirements, payload: invalid } };
  await assert.rejects(new ReceiptPayloadAuthority(keys).settle(request), (error) => {
    assert.equal(error.code, "verification-failure");
    assert.ok(error.cause instanceof SequencerReceiptVerificationError);
    assert.equal(error.cause.code, sdkError.code);
    assert.equal(error.cause.check, sdkError.check);
    t.diagnostic(JSON.stringify({ outerCode: error.code, innerCode: error.cause.code, innerCheck: error.cause.check }));
    return true;
  });
  assert.deepEqual(await resolver.resolve(vector.receiptJson.txHash, requirements.network), evidence);
}

test(receiptAuthorityPreservesSdkRefusal);
