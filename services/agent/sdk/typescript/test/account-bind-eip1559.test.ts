import assert from "node:assert/strict";
import { test } from "node:test";

import { ed25519 } from "@noble/curves/ed25519.js";
import { secp256k1 } from "@noble/curves/secp256k1.js";

import {
  ADDR_PRECOMPILE_ADDRESS,
  AccountDerivationError,
  bindTransaction,
  bindTransactionRequest,
  deriveFromSeed,
  deriveFromWalletSignature,
  keyDerivationHash,
  layerXBindMessage,
  planLayerXBind,
  type AccountDerivationErrorCode,
} from "../src/account-derivation.js";
import {
  decodeRawEip1559,
  eip1559SigningHash,
  recoverEip1559Sender,
} from "../src/evm-transaction.js";

const seed = Uint8Array.from(Buffer.from("000102030405060708090a0b0c0d0e0f", "hex"));
const fees = { maxPriorityFeePerGas: 1n, maxFeePerGas: 3n, gasLimit: 150_000n };

function bytes(hex: string): Uint8Array {
  return Uint8Array.from(Buffer.from(hex.replace(/^0x/u, ""), "hex"));
}

function refuses(code: AccountDerivationErrorCode, run: () => unknown): void {
  assert.throws(run, (error: unknown) => error instanceof AccountDerivationError && error.code === code);
}

function bind_transaction_request_is_a_type_2_transaction_on_chain_125(): void {
  const derived = deriveFromSeed(seed);
  const plan = planLayerXBind(derived, 125n, { boundDidPublicKey: null, nonce: 4n });
  assert.equal(plan.action, "bind");
  assert.ok(plan.action === "bind");

  for (const nonce of [0n, 9n, 0xffffffffffffffffn]) {
    assert.deepEqual(bindTransactionRequest(derived, plan.call, nonce), {
      method: "eth_sendTransaction",
      params: [
        {
          from: derived.evmAddress,
          to: ADDR_PRECOMPILE_ADDRESS,
          data: plan.call.data,
          value: "0x0",
          type: "0x2",
          chainId: "0x7d",
          nonce: `0x${nonce.toString(16)}`,
        },
      ],
    });
  }
  assert.equal(plan.call.nonce, 4n);
  assert.equal(plan.call.data.slice(0, 10), "0xdd9aa628");
  refuses("invalid_bind_fees", () => bindTransactionRequest(derived, plan.call));
  for (const nonce of [-1n, 1n << 64n]) {
    refuses("invalid_bind_fees", () => bindTransactionRequest(derived, plan.call, nonce));
  }
  refuses("malformed_precompile_answer", () =>
    bindTransactionRequest(derived, { ...plan.call, to: derived.evmAddress }, 9n),
  );
  const wrongChain = planLayerXBind(derived, 1n, { boundDidPublicKey: null, nonce: 4n });
  assert.ok(wrongChain.action === "bind");
  refuses("malformed_precompile_answer", () => bindTransactionRequest(derived, wrongChain.call, 9n));
}

function derived_DID_key_signs_only_the_bind_message(): void {
  for (const index of [0, 7]) {
    const derived = deriveFromSeed(seed, index);
    assert.ok(derived.evmPrivateKey !== null);
    assert.equal(derived.evmPrivateKey.length, 32);
    assert.equal(derived.layerxSeed.length, 32);
    assert.notDeepEqual(derived.evmPrivateKey, derived.layerxSeed);
    assert.deepEqual(ed25519.getPublicKey(derived.layerxSeed), bytes(derived.layerxPublicKey));

    const plan = planLayerXBind(derived, 125n, { boundDidPublicKey: null, nonce: 4n });
    assert.ok(plan.action === "bind");
    const bindMessage = layerXBindMessage(125n, derived.evmAddress, 4n);
    assert.deepEqual(bindMessage.slice(0, 17), new TextEncoder().encode("LX:PAXEER-BIND:v1"));
    assert.ok(ed25519.verify(bytes(plan.call.signature), bindMessage, bytes(derived.layerxPublicKey)));
    assert.equal(
      ed25519.verify(
        bytes(plan.call.signature),
        layerXBindMessage(125n, derived.evmAddress, 5n),
        bytes(derived.layerxPublicKey),
      ),
      false,
    );

    for (const nonce of [0n, 9n]) {
      const raw = bindTransaction(derived, plan.call, nonce, fees);
      assert.ok(raw instanceof Uint8Array);
      assert.equal(raw[0], 2);
      const transaction = decodeRawEip1559(raw);
      assert.equal(transaction.chainId, 125n);
      assert.equal(transaction.nonce, nonce);
      assert.equal(transaction.maxPriorityFeePerGas, fees.maxPriorityFeePerGas);
      assert.equal(transaction.maxFeePerGas, fees.maxFeePerGas);
      assert.equal(transaction.gasLimit, fees.gasLimit);
      assert.equal(transaction.value, 0n);
      assert.deepEqual(transaction.to, bytes(ADDR_PRECOMPILE_ADDRESS));
      assert.deepEqual(transaction.data, bytes(plan.call.data));
      assert.deepEqual(transaction.accessList, []);
      assert.deepEqual(transaction.data.slice(4, 36), bytes(derived.layerxPublicKey));
      assert.deepEqual(transaction.data.slice(100), bytes(plan.call.signature));
      assert.equal(recoverEip1559Sender(raw), derived.evmAddress);

      const signature = new secp256k1.Signature(transaction.r, transaction.s, transaction.yParity);
      const hash = eip1559SigningHash(transaction);
      assert.equal(signature.hasHighS(), false);
      assert.ok(secp256k1.verify(
        signature.toBytes("compact"),
        hash,
        secp256k1.getPublicKey(derived.evmPrivateKey),
        { prehash: false },
      ));
      assert.equal(ed25519.verify(bytes(plan.call.signature), hash, bytes(derived.layerxPublicKey)), false);
      assert.equal(secp256k1.verify(
        signature.toBytes("compact"),
        hash,
        secp256k1.getPublicKey(derived.layerxSeed),
        { prehash: false },
      ), false);
    }

    for (const nonce of [-1n, 1n << 64n]) {
      refuses("invalid_bind_fees", () => bindTransaction(derived, plan.call, nonce, fees));
    }
    refuses("invalid_bind_fees", () => bindTransaction(derived, plan.call, 9n, { ...fees, gasLimit: 0n }));
    refuses("invalid_bind_fees", () => bindTransaction(derived, plan.call, 9n, { ...fees, maxFeePerGas: 0n }));
    for (const call of [
      { ...plan.call, to: derived.evmAddress },
      { ...plan.call, data: `${plan.call.data}00` },
      { ...plan.call, didPublicKey: "00".repeat(32) },
      { ...plan.call, signature: "00".repeat(64) },
      { ...plan.call, nonce: 5n },
    ]) {
      refuses("malformed_precompile_answer", () => bindTransaction(derived, call, 9n, fees));
    }

    const parameters = { chainId: 125n, address: derived.evmAddress };
    const walletSignature = secp256k1.sign(keyDerivationHash(parameters), derived.evmPrivateKey, {
      prehash: false,
      format: "recovered",
    });
    const canonical = Uint8Array.from([...walletSignature.slice(1), 27 + walletSignature[0]!]);
    const walletDerived = deriveFromWalletSignature(parameters, canonical);
    assert.equal(walletDerived.evmPrivateKey, null);
    const walletPlan = planLayerXBind(walletDerived, 125n, { boundDidPublicKey: null, nonce: 4n });
    assert.ok(walletPlan.action === "bind");
    assert.equal(bindTransactionRequest(walletDerived, walletPlan.call, 9n).method, "eth_sendTransaction");
    refuses("evm_key_unavailable", () => bindTransaction(walletDerived, walletPlan.call, 9n, fees));
  }
}

test(bind_transaction_request_is_a_type_2_transaction_on_chain_125);
test(derived_DID_key_signs_only_the_bind_message);
