import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { keccak256 } from "@sidiora/layerx-sdk";
import {
  LayerXApplicationStateError,
  receiptKeysFromEnvironment,
  requiredEnvironment,
  SequencerReceiptClient,
} from "../support/runtime.mjs";

export async function verifyCalledReceipt(rpcUrl, txHash, receiptKeys) {
  if (typeof txHash !== "string" || !/^0x[0-9a-fA-F]{64}$/u.test(txHash)) {
    throw new LayerXApplicationStateError("refused", "invalid_transaction_hash");
  }
  const transactionHash = txHash.toLowerCase();
  const client = new SequencerReceiptClient(rpcUrl, receiptKeys);
  let response;
  let body;
  try {
    response = await fetch(client.rpcUrl, {
      method: "POST",
      headers: { accept: "application/json", "content-type": "application/json" },
      body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "eth_getRawTransactionByHash", params: [transactionHash] }),
    });
    body = await response.json();
  } catch {
    throw new LayerXApplicationStateError("unknown", "transaction_unreachable");
  }
  if (body === null || typeof body !== "object" || Array.isArray(body)
    || body.jsonrpc !== "2.0" || body.id !== 1) {
    throw new LayerXApplicationStateError("unknown", "invalid_transaction_response");
  }
  if (body.error !== undefined) throw new LayerXApplicationStateError("refused", "transaction_refused");
  if (!response.ok) throw new LayerXApplicationStateError("unknown", `transaction_http_${response.status}`);
  if (body.result === null) throw new LayerXApplicationStateError("pending", "transaction_unavailable");
  if (typeof body.result !== "string" || !/^0x(?:[0-9a-fA-F]{2})+$/u.test(body.result)
    || body.result.length > 2097154) {
    throw new LayerXApplicationStateError("refused", "invalid_raw_transaction");
  }
  const raw = Uint8Array.from(Buffer.from(body.result.slice(2), "hex"));
  if (`0x${Buffer.from(keccak256(raw)).toString("hex")}` !== transactionHash) {
    throw new LayerXApplicationStateError("refused", "transaction_hash_mismatch");
  }
  return client.resolve(raw);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    if (process.argv.length !== 3) throw new LayerXApplicationStateError("refused", "usage: verify-receipt.mjs txHash");
    const proof = await verifyCalledReceipt(requiredEnvironment("LAYERX_RPC_URL"), process.argv[2],
      receiptKeysFromEnvironment("LAYERX_RECEIPT_KEYS_JSON"));
    console.log(proof.verificationLevel);
  } catch (error) {
    console.error(error instanceof LayerXApplicationStateError ? error.state : "refused");
    process.exitCode = 1;
  }
}
