import { keccak256, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import { LayerXApplicationStateError } from "../support/runtime.mjs";

const TRANSACTION_HASH = /^0x[0-9a-fA-F]{64}$/u;
const bytes = (value) => Uint8Array.from(Buffer.from(value.slice(2), "hex"));

function findReceipt(value) {
  if (value === null || typeof value !== "object") return undefined;
  for (const field of ["txHash", "transactionHash", "tx_hash", "transaction_hash"]) {
    if (typeof value[field] !== "string") continue;
    if (!TRANSACTION_HASH.test(value[field])) {
      throw new LayerXApplicationStateError("unknown", "program_response_invalid_transaction_hash");
    }
    return value[field].toLowerCase();
  }
  for (const child of Object.values(value)) {
    const found = findReceipt(child);
    if (found !== undefined) return found;
  }
  return undefined;
}

function findState(value) {
  if (value === null || typeof value !== "object") return undefined;
  if (["pending", "unknown", "refused"].includes(value.state)) return value.state;
  for (const child of Object.values(value)) {
    const found = findState(child);
    if (found !== undefined) return found;
  }
  return undefined;
}

async function rawTransactionForHash(client, txHash) {
  let response;
  let body;
  try {
    response = await fetch(client.rpcUrl, {
      method: "POST",
      headers: { accept: "application/json", "content-type": "application/json" },
      body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "eth_getRawTransactionByHash", params: [txHash] }),
    });
    body = await response.json();
  } catch {
    throw new LayerXApplicationStateError("unknown", "program_transaction_unreachable");
  }
  if (body === null || typeof body !== "object" || Array.isArray(body)
    || body.jsonrpc !== "2.0" || body.id !== 1) {
    throw new LayerXApplicationStateError("unknown", "invalid_program_transaction_response");
  }
  if (body.error !== undefined) throw new LayerXApplicationStateError("refused", "program_transaction_refused");
  if (!response.ok) throw new LayerXApplicationStateError("unknown", `program_transaction_http_${response.status}`);
  if (body.result === null) throw new LayerXApplicationStateError("pending", "program_transaction_unavailable");
  if (typeof body.result !== "string" || !/^0x(?:[0-9a-fA-F]{2})+$/u.test(body.result)
    || body.result.length > 2097154) {
    throw new LayerXApplicationStateError("refused", "invalid_program_raw_transaction");
  }
  const raw = bytes(body.result);
  if (`0x${Buffer.from(keccak256(raw)).toString("hex")}` !== txHash) {
    throw new LayerXApplicationStateError("refused", "program_transaction_hash_mismatch");
  }
  return raw;
}

export async function verifyProgramOutcome(output, client) {
  const txHash = findReceipt(output);
  if (txHash === undefined) {
    const state = findState(output);
    if (state !== undefined) return { state, output };
    throw new LayerXApplicationStateError("unknown", "program_response_omitted_transaction_hash");
  }
  const raw = await rawTransactionForHash(client, txHash);
  const proof = await client.resolve(raw);
  const key = proof.receiptKey;
  const verified = await verifySequencerReceipt(bytes(proof.signedReceipt), raw, [{
    keyId: bytes(key.keyId),
    publicKey: bytes(key.publicKey),
    first_height: BigInt(key.firstHeight),
    last_height: BigInt(key.lastHeight),
  }]);
  const resultCode = verified.receipt.resultCode;
  if (resultCode !== 0 && resultCode !== 1) {
    throw new LayerXApplicationStateError("refused", "program_receipt_invalid_status");
  }
  const status = resultCode === 0 ? 1 : 0;
  return {
    state: status === 1 ? "completed" : "refused",
    receiptDigest: proof.receiptDigest,
    rung: proof.verificationLevel,
    status,
    output,
  };
}
