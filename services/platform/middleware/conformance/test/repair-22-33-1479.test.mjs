import "../pay6/source-loader.mjs";
import assert from "node:assert/strict";
import test from "node:test";

await import("../pay6/committed-rung.test.mjs");
await import("../pay6/ledger-draw.test.mjs");
await import("../pay6/rpc.test.mjs");
const { mergedHttpEnvelopesSurviveReopen, submittedTracksWithoutPaymentAuthorization } = await import("../pay6/walkthrough.mjs");
const { recordedCommittedFixture } = await import("../pay6/committed-fixture.mjs");
const { verifyCommittedReceipt, verifySequencerReceipt } = await import("@sidiora/layerx-sdk");
const { PaymentRpc, encodePaymentCommitmentEvidence, decodePaymentCommitmentEvidence,
  verifyPaymentCommitmentEvidence } = await import("../../seller/src/index.ts");
const { committedVector } = await import("../../seller/test/committed-fixture.mjs");
const { loopback } = await import("../pay6/peers.mjs");

async function recordedNodeCommitNeedsAuthenticatedInclusion(context) {
  const { evidence, resolver } = await recordedCommittedFixture();
  const receiptNode = await loopback(context, (body, request) => {
    assert.equal(request.url, "/");
    assert.equal(request.method, "POST");
    assert.equal(body.jsonrpc, "2.0");
    assert.equal(body.method, "pax_sequencerReceipt");
    assert.deepEqual(body.params, [committedVector.receiptJson.txHash]);
    return { result: committedVector.receiptJson };
  });
  const rpcResult = await new PaymentRpc(receiptNode.endpoint).receipt(committedVector.receiptJson.txHash);
  assert.deepEqual(rpcResult, committedVector.receiptJson);
  const bytes = (value) => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
  const pin = committedVector.pinnedKey;
  const verified = await verifySequencerReceipt(bytes(rpcResult.receipt), bytes(committedVector.rawTransaction), [{
    keyId: bytes(pin.keyId), publicKey: bytes(pin.publicKey),
    first_height: BigInt(pin.firstHeight), last_height: BigInt(pin.lastHeight),
  }]);
  const wire = encodePaymentCommitmentEvidence(evidence);
  const commitmentNode = await loopback(context, (body, request) => {
    assert.equal(request.method, "GET");
    assert.equal(request.url, "/commitment");
    return { text: JSON.stringify(wire) };
  });
  const response = await fetch(`${commitmentNode.endpoint}commitment`);
  assert.equal(response.status, 200);
  const transported = decodePaymentCommitmentEvidence(await response.json());
  assert.deepEqual(transported, evidence);
  await verifyPaymentCommitmentEvidence(verified, "committed", transported, "eip155:125", resolver);
  const nodeEvidence = { chainId: evidence.blockHeader.chainId, landingHeight: evidence.landingBlockHeight,
    header: evidence.blockHeader, inclusion: evidence.receiptInclusion, blockId: evidence.commit.blockId,
    commit: evidence.commit, validators: evidence.validatorSet };
  const committed = await verifyCommittedReceipt(verified, nodeEvidence);
  assert.ok(committed.signedPower * 3n > committed.totalPower * 2n);
  assert.equal(committed.landingHeight, verified.height + 1n);
  for (const mutate of [
    (value) => { delete value.inclusion; },
    (value) => { value.inclusion.batchTransaction[40] ^= 1; },
    (value) => { value.header.dataHash[0] ^= 1; },
    (value) => { value.inclusion.activityIds[0][0] ^= 1; },
    (value) => { value.blockId.hash[0] ^= 1; },
    (value) => { value.inclusion.transaction[40] ^= 1; },
    (value) => { value.inclusion.result.data[40] ^= 1; },
    (value) => { value.inclusion.resultProof.aunts[0][0] ^= 1; },
    (value) => { value.inclusion.executionCommit.signatures[0].signature[0] ^= 1; },
  ]) {
    const invalid = structuredClone(nodeEvidence);
    mutate(invalid);
    await assert.rejects(verifyCommittedReceipt(verified, invalid), (error) => error.code === "verification-failure");
  }
}

test(mergedHttpEnvelopesSurviveReopen);
test(submittedTracksWithoutPaymentAuthorization);
test(recordedNodeCommitNeedsAuthenticatedInclusion);
