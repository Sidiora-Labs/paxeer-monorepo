import { createHash } from "node:crypto";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { readFileSync } from "node:fs";
import { keccak256 } from "@sidiora/layerx-sdk";
import { decodePaymentExactLog } from "../../seller/dist/index.js";

export const bytes = (value) => Uint8Array.from(Buffer.from(value.replace(/^0x/u, ""), "hex"));
export const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
export const fixtureRoot = new URL("../../../../../sdk/platform/conformance/fixtures/", import.meta.url);
const vectors = JSON.parse(readFileSync(new URL("sequencer-receipt-v1.json", fixtureRoot), "utf8"));
export const exact = vectors.cases.find((entry) => entry.id === "V1-layerxledger-pay-exact");
export const pinsFor = (vector) => [{ keyId: bytes(vector.pinnedKey.keyId), publicKey: bytes(vector.pinnedKey.publicKey),
  first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight) }];
export const keys = pinsFor(exact);
export const proof = {
  format: "layerx-receipt-proof-v2", verificationLevel: "receipted",
  rawTransaction: exact.rawTransaction, signedReceipt: exact.signedReceipt,
  receiptDigest: hex(keccak256(bytes(exact.signedReceipt))),
  receiptKey: { keyId: exact.pinnedKey.keyId, publicKey: exact.pinnedKey.publicKey,
    firstHeight: `0x${keys[0].first_height.toString(16)}`, lastHeight: `0x${keys[0].last_height.toString(16)}` },
};
const log = exact.receiptJson.logs[0];
export const payment = decodePaymentExactLog({ address: log.address, topics: log.topics.map(bytes), data: bytes(log.data) });
export const offer = {
  scheme: "exact", network: "eip155:125", amount: payment.amount.toString(),
  asset: hex(payment.assetId), payTo: hex(payment.to), maxTimeoutSeconds: 120,
  extra: { layerx: { commitment: "receipted", payer: hex(payment.from).slice(2), purposeHash: hex(payment.paymentReference).slice(2) } },
};
export const txHash = exact.receiptJson.txHash;
export const payer = `0x${exact.receiptJson.from.slice(2).padStart(64, "0")}`;
export const result = exact.receiptJson;
export const payloadFor = (resource) => ({ x402Version: 2, resource, accepted: offer, payload: proof });

export const agentSpend = {
  tenant: "tenant", networkId: "125", authorityHex: "01",
  signerPublicKey: Buffer.from(secp256k1.getPublicKey(new Uint8Array(32).fill(1))).toString("hex"),
  submitIdempotencyKey: "02".repeat(32), approvalCurrentSequence: "0",
  preparation: {
    activity_type: "786433", actor: "actor", authority: "authority", account_sequence: "0",
    not_before: "0", not_after: "1", idempotency_key: "01".repeat(32), fee_limit: "100000",
    payload: "00", payload_hash: createHash("sha256").update("LXP/v1/payload-hash\0").update(Buffer.of(0)).digest("hex"),
  },
  asset: offer.asset.slice(2), amount: offer.amount, recipient: offer.payTo.slice(2),
  commitment: { network: offer.network, level: "receipted" },
};
