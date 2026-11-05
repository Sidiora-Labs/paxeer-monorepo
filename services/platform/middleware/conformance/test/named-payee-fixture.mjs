import { readFileSync } from "node:fs";
import { verifySequencerReceipt } from "@sidiora/layerx-sdk";

const hexBytes = (value) => Uint8Array.from(Buffer.from(value.replace(/^0x/u, ""), "hex"));
const base64 = (value) => Uint8Array.from(Buffer.from(value, "base64"));

function timestamp(value) {
  const match = /^(.*?)(?:\.([0-9]{1,9}))?Z$/u.exec(value);
  return { seconds: BigInt(Date.parse(`${match[1]}Z`)) / 1000n,
    nanos: Number((match[2] ?? "").padEnd(9, "0")) };
}

function blockId(value) {
  return { hash: hexBytes(value.hash),
    partSetHeader: { total: value.parts.total, hash: hexBytes(value.parts.hash) } };
}

function header(value) {
  return { versionBlock: BigInt(value.version.block), versionApp: BigInt(value.version.app),
    chainId: value.chain_id, height: BigInt(value.height), time: timestamp(value.time),
    lastBlockId: blockId(value.last_block_id),
    ...Object.fromEntries(["last_commit_hash", "data_hash", "validators_hash", "next_validators_hash",
      "consensus_hash", "app_hash", "last_results_hash", "evidence_hash", "proposer_address"]
      .map((field) => [field.replace(/_([a-z])/gu, (_, letter) => letter.toUpperCase()), hexBytes(value[field])])) };
}

function commit(value) {
  return { height: BigInt(value.height), round: value.round, blockId: blockId(value.block_id),
    signatures: value.signatures.map((signature) => ({ blockIdFlag: signature.block_id_flag,
      validatorAddress: hexBytes(signature.validator_address), timestamp: timestamp(signature.timestamp),
      signature: signature.signature === null ? new Uint8Array() : base64(signature.signature) })) };
}

function proof(value) {
  return { total: BigInt(value.total), index: BigInt(value.index), aunts: value.aunts.map(base64) };
}

export async function recordedNamedPayeeFixture() {
  const recording = JSON.parse(readFileSync(new URL(
    "../../../../../sdk/platform/conformance/fixtures/vector-v7-named-payee.json", import.meta.url,
  ), "utf8"));
  const { settlement, authorization, parties } = recording;
  const json = settlement.receipt;
  const pin = { keyId: hexBytes(json.keyId), publicKey: Uint8Array.from(authorization.PublicKey),
    first_height: BigInt(authorization.FirstBatchNumber), last_height: BigInt(authorization.LastBatchNumber) };
  const verified = await verifySequencerReceipt(hexBytes(json.receipt), hexBytes(settlement.raw_transaction), [pin]);
  const validators = settlement.validators.map((validator) => ({
    publicKey: base64(validator.pub_key.value), address: hexBytes(validator.address), votingPower: BigInt(validator.voting_power),
  }));
  const nextCommit = commit(settlement.commit);
  const evidence = {
    landingBlockHeight: nextCommit.height,
    blockHeader: { ...header(settlement.header), blockId: nextCommit.blockId },
    commit: nextCommit, validatorSet: validators,
    receiptInclusion: {
      transactionHash: verified.receipt.activityId, receiptDigest: verified.receiptDigest, sequence: verified.receipt.globalSequence,
      executionHeader: header(settlement.execution_header), successorHeader: header(settlement.header),
      executionCommit: commit(settlement.execution_commit), executionValidators: validators,
      transaction: base64(settlement.transaction_proof.data), transactionProof: proof(settlement.transaction_proof.proof),
      batchTransaction: base64(settlement.sequence_batch_proof.data), batchProof: proof(settlement.sequence_batch_proof.proof),
      result: { code: settlement.exec_tx_result.code ?? 0, data: base64(settlement.exec_tx_result.data),
        gasWanted: BigInt(settlement.exec_tx_result.gas_wanted), gasUsed: BigInt(settlement.exec_tx_result.gas_used) },
      resultProof: proof(settlement.exec_tx_result_proof), activityIds: [verified.receipt.activityId],
    },
  };
  const resolver = {
    async resolve(transactionHash, network) {
      if (transactionHash !== json.txHash || network !== "eip155:125") throw new Error("untrusted-transaction");
      return structuredClone(evidence);
    },
    async resolveValidators(network, height) {
      if (network !== "eip155:125" || height !== evidence.landingBlockHeight) throw new Error("untrusted-chain-height");
      return { chainId: evidence.blockHeader.chainId, validatorsHash: evidence.blockHeader.validatorsHash.slice() };
    },
  };
  return { parties, pin, verified, evidence, resolver, vector: {
    rawTransaction: settlement.raw_transaction, signedReceipt: json.receipt, receiptJson: json,
    pinnedKey: { keyId: json.keyId, publicKey: json.signingKey },
  } };
}
