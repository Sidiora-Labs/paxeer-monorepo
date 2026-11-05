import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import test from "node:test";
import { preparedTransactionReference } from "../agent-spend/services.mjs";

const { keccak256, serializeTransaction } = createRequire(new URL("../../../../sdk/wallet/package.json", import.meta.url))("viem");
const unsignedTransaction = {
  type: "eip1559",
  chainId: 125,
  nonce: 7,
  maxPriorityFeePerGas: 1n,
  maxFeePerGas: 2n,
  gas: 100000n,
  to: "0x000000000000000000000000000000000000101c",
  value: 0n,
  data: "0x01020304",
  accessList: [],
};
const prepared = {
  chain_id: "125",
  nonce: "7",
  max_priority_fee_per_gas: "1",
  max_fee_per_gas: "2",
  gas_limit: "100000",
  to: unsignedTransaction.to.slice(2),
  data: unsignedTransaction.data.slice(2),
};

function preparedReferenceMatchesKeccakOfUnsignedTransaction() {
  const unsigned = serializeTransaction(unsignedTransaction);
  assert.equal(unsigned, "0x02e47d070102830186a094000000000000000000000000000000000000101c808401020304c0");
  const reference = keccak256(unsigned).slice(2);
  const preparation = { ...prepared, signing_hash: reference, preparation_ref: reference };
  assert.equal(preparedTransactionReference(preparation), reference);
  for (const changed of [{ nonce: "8" }, { data: "01020305" }, { to: "000000000000000000000000000000000000101d" }, { max_fee_per_gas: "3" }]) {
    assert.throws(() => preparedTransactionReference({ ...preparation, ...changed }), { message: "prepared_bytes_mismatch" });
  }
}

function preparedReferenceRefusesSha256Reference() {
  const unsigned = serializeTransaction(unsignedTransaction);
  const reference = createHash("sha256").update(Buffer.from(unsigned.slice(2), "hex")).digest("hex");
  const signingHash = keccak256(unsigned).slice(2);
  assert.notEqual(reference, signingHash);
  assert.throws(() => preparedTransactionReference({ ...prepared, signing_hash: signingHash, preparation_ref: reference }), {
    message: "prepared_bytes_mismatch",
  });
}

test(preparedReferenceMatchesKeccakOfUnsignedTransaction);
test(preparedReferenceRefusesSha256Reference);
