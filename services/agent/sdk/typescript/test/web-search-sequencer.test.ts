import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import { test } from "node:test";
import { ed25519 } from "@noble/curves/ed25519.js";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { keccak_256 } from "@noble/hashes/sha3.js";
import { decodeRawEip1559, encodeUnsignedEip1559 } from "../src/evm-transaction.js";
import { type SequencerReceiptKey as ReceiptKey } from "../src/verifier.js";
import { walletAccount } from "../src/rpc.js";
import { decodePayerGrant, WebSearchClient, WebSearchError, type WebSearchScheme } from "../src/web-search.js";
import { encodeGrant, grantAuthorizationMessage, type PayerGrant } from "../src/x402/receive.js";

type Json = Record<string, unknown>;
interface Offer {
  readonly scheme: string;
  readonly network: string;
  readonly amount: string;
  readonly asset: string;
  readonly payTo: string;
  readonly maxTimeoutSeconds: number;
  readonly extra: { readonly layerx: { readonly account: string; readonly commitment: string; readonly currency: string; readonly payer: string; readonly purposeHash: string } };
}
interface ReceiptVector {
  readonly rawTransaction: string;
  readonly pinnedKey: { readonly firstHeight: number; readonly lastHeight: number };
}
const vectors = JSON.parse(readFileSync(new URL(
  "../../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url,
), "utf8")) as { readonly chainId: number; readonly cases: readonly ReceiptVector[] };
const vector = vectors.cases[0]!;
const bytes = (hex: string): Buffer => Buffer.from(hex.replace(/^0x/u, ""), "hex");
const hex = (value: Uint8Array): string => Buffer.from(value).toString("hex");
const header = (value: unknown): string => Buffer.from(JSON.stringify(value)).toString("base64");
const uint = (value: bigint, size: number): Buffer => bytes(value.toString(16).padStart(size * 2, "0"));
const bounded = (value: Uint8Array): Buffer => Buffer.concat([uint(BigInt(value.length), 4), value]);
const zero = Buffer.alloc(32);
const secret = new Uint8Array(32).fill(7);
const key: ReceiptKey = {
  keyId: new Uint8Array(32).fill(8), publicKey: ed25519.getPublicKey(secret),
  first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight),
};
const nativeAsset = "0e237584332fc3e1f1672d80a167a8d027597a64b34f4be18951e60babdd4f73";
const payerDid = "did:layerx:buyer";
const sellerDid = "did:layerx:seller";
const offer = {
  scheme: "metered", network: `eip155:${vectors.chainId}`,
  amount: decodeRawEip1559(bytes(vector.rawTransaction)).value.toString(),
  asset: nativeAsset, payTo: walletAccount(sellerDid, nativeAsset, nativeAsset), maxTimeoutSeconds: 30,
  extra: { layerx: {
    account: `agent:${sellerDid}:main`, commitment: "executed", currency: "PAX",
    payer: walletAccount(payerDid, nativeAsset, nativeAsset),
    purposeHash: createHash("sha256").update("/search?q=paxeer").digest("hex"),
  } },
} satisfies Offer;
const payerSecret = new Uint8Array(32).fill(6);
const unsignedGrant: PayerGrant = {
  grant_id: hex(new Uint8Array(32).fill(1)), from: offer.extra.layerx.payer, recipient: offer.payTo,
  asset: offer.asset, per_draw_maximum: offer.amount, allowance: offer.amount,
  recurring: false, window_length: "0", expiration: "2000000000", purpose_hash: offer.extra.layerx.purposeHash,
  has_reference: false, reference_hash: hex(zero), revocation_sequence: "0",
  public_key: hex(ed25519.getPublicKey(payerSecret)), signature: "00".repeat(64),
};
const canonicalGrant = encodeGrant({
  ...unsignedGrant, signature: hex(ed25519.sign(grantAuthorizationMessage(unsignedGrant), payerSecret)),
});
const payment = { grant: hex(canonicalGrant), idempotencyKey: hex(new Uint8Array(32).fill(2)) };
const grant = decodePayerGrant(canonicalGrant);
const calldata = Buffer.concat([
  bytes("2e89490f"), bytes(grant.grant_id), bytes(offer.payTo), uint(BigInt(offer.amount), 32),
  bytes(payment.idempotencyKey), uint(1n, 32), bytes(offer.extra.layerx.purposeHash), zero,
]);
const unsignedTransaction = encodeUnsignedEip1559({
  chainId: BigInt(vectors.chainId), nonce: 0n, maxPriorityFeePerGas: 1n, maxFeePerGas: 2n, gasLimit: 100000n,
  to: bytes("000000000000000000000000000000000000101a"), value: 0n, data: calldata, accessList: [],
});
const signature = secp256k1.sign(keccak_256(unsignedTransaction), new Uint8Array(32).fill(9), { prehash: false, format: "recovered" });
const scalar = (value: Uint8Array): Buffer => {
  const canonical = bytes(BigInt(`0x${hex(value)}`).toString(16).padStart(64, "0").replace(/^(00)+/u, ""));
  return canonical.length === 1 && canonical[0]! < 0x80 ? canonical : Buffer.concat([Buffer.of(0x80 + canonical.length), canonical]);
};
const lengthWidth = unsignedTransaction[1]! - 0xf7;
const transactionPayload = Buffer.concat([
  unsignedTransaction.subarray(2 + lengthWidth),
  signature[0] === 0 ? Buffer.of(0x80) : Buffer.of(1), scalar(signature.subarray(1, 33)), scalar(signature.subarray(33)),
]);
const transactionLength = uint(BigInt(transactionPayload.length), 2);
const rawTransaction = Buffer.concat([Buffer.of(2, 0xf9), transactionLength, transactionPayload]);
const transactionHash = `0x${hex(keccak_256(rawTransaction))}`;

function signedReceipt(signingSecret = secret, keyId = key.keyId, resultCode = 0): Buffer {
  const amount = BigInt(offer.amount);
  const fields = Buffer.concat([
    bytes("000252010002"), bounded(keccak_256(rawTransaction)), uint(1n, 8), bounded(zero),
    bounded(new Uint8Array(32).fill(1)), bounded(keccak_256(rawTransaction)), uint(BigInt(resultCode), 4), uint(0n, 4),
    uint(0n, 16), bounded(uint(key.first_height, 32)), uint(1n, 2), uint(1n, 4), uint(0n, 4), Buffer.of(6),
    bounded(bytes(offer.asset)), uint(amount, 16), bounded(bytes(offer.extra.layerx.payer)),
    uint(amount * 2n, 16), uint(amount, 16), uint(1n, 8), bounded(bytes(offer.payTo)),
    uint(0n, 16), uint(amount, 16), bounded(zero), bounded(zero), bounded(keyId), uint(1000n, 8),
  ]);
  const digest = createHash("sha256").update("LXP/v1/receipt\0").update(fields).update(Buffer.of(0)).digest();
  return Buffer.concat([fields, Buffer.of(1), bounded(ed25519.sign(digest, signingSecret))]);
}

async function seller(receipt: Uint8Array, scheme: WebSearchScheme = "metered", overrides: Json = {}, layerxOverrides: Json = {}) {
  const sent: Json[] = [];
  const accepted = scheme === "metered" ? offer : {
    ...offer, scheme, extra: { layerx: { account: offer.extra.layerx.account, commitment: "executed", currency: "PAX" } },
  };
  const server = createServer((request, response) => {
    const signature = request.headers["payment-signature"];
    response.setHeader("content-type", "application/json");
    if (signature === undefined) {
      response.writeHead(402, { "PAYMENT-REQUIRED": header({ x402Version: 2, accepts: [accepted] }) });
      response.end(JSON.stringify({ error: "payment_required" }));
      return;
    }
    sent.push({ target: request.url, payer: request.headers["layerx-payer-did"], payment: JSON.parse(Buffer.from(String(signature), "base64").toString("utf8")) as Json });
    response.writeHead(200, { "PAYMENT-RESPONSE": header({
      success: true, network: offer.network, amount: offer.amount, payer: offer.extra.layerx.payer, transaction: transactionHash,
      extensions: { layerx: {
        receipt: Buffer.from(receipt).toString("base64"),
        receiptDigest: createHash("sha256").update("LXP/v1/merkle-leaf\0").update(receipt).digest("hex"),
        verificationLevel: "receipted", purposeHash: offer.extra.layerx.purposeHash,
        rawTransaction: `0x${hex(rawTransaction)}`, ...layerxOverrides,
      } }, ...overrides,
    }) });
    response.end(JSON.stringify({ results: [{ url: "https://example.com", title: "Result", snippet: "Paid search" }] }));
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.ok(address !== null && typeof address !== "string");
  return {
    endpoint: `http://127.0.0.1:${address.port}`, sent,
    close: () => new Promise<void>((resolve, reject) => server.close((error) => error === undefined ? resolve() : reject(error))),
  };
}

function client(endpoint: string, receipt: Uint8Array, receiptKeys: readonly ReceiptKey[] = [key], scheme: WebSearchScheme = "metered"): WebSearchClient {
  return new WebSearchClient({
    endpoint, network: offer.network, nativeAsset,
    assets: { PAX: { assetId: offer.asset, maxAmount: BigInt(offer.amount) } }, receiptKeys,
    payer: {
      did: payerDid, preferences: [{ currency: "PAX", scheme }],
      metered: async () => ({ grant: bytes(payment.grant), idempotencyKey: payment.idempotencyKey }),
      exact: async () => ({ receipt, rawTransaction }),
    }, now: () => 1_000_000_000n,
  });
}

function refused(code: string): (error: unknown) => boolean {
  return (error) => {
    assert.ok(error instanceof WebSearchError);
    assert.equal(error.code, code);
    return true;
  };
}

async function web_settlement_transaction_is_the_draw_transaction_hash(): Promise<void> {
  const decoded = decodeRawEip1559(new Uint8Array(rawTransaction));
  assert.equal(hex(decoded.to), "000000000000000000000000000000000000101a");
  assert.deepEqual(decoded.data, new Uint8Array(calldata));
  const receipt = signedReceipt();
  for (const scheme of ["metered", "exact"] as const) {
    const sidecar = await seller(receipt, scheme);
    try {
      const found = await client(sidecar.endpoint, receipt, [key], scheme).search("paxeer");
      assert.ok(found.settlement !== null);
      assert.equal(found.settlement.transaction, transactionHash);
      assert.equal(found.settlement.scheme, scheme);
      assert.equal(found.results[0]?.snippet, "Paid search");
      assert.equal(sidecar.sent.length, 1);
      assert.equal(sidecar.sent[0]!["target"], "/search?q=paxeer");
      const sent = sidecar.sent[0]!["payment"] as { readonly payload: Json };
      if (scheme === "metered") assert.deepEqual(sent.payload, payment);
      else assert.equal(sent.payload["rawTransaction"], `0x${hex(rawTransaction)}`);
    } finally {
      await sidecar.close();
    }
  }
  for (const [overrides, layerxOverrides, code] of [
    [{ transaction: `0x${"11".repeat(32)}` }, {}, "settlement-receipt-mismatch"],
    [{}, { rawTransaction: `0x${hex(unsignedTransaction)}` }, "receipt-unverified"],
    [{}, { rawTransaction: "0x0" }, "settlement-receipt-mismatch"],
  ] as const) {
    const sidecar = await seller(receipt, "metered", overrides, layerxOverrides);
    try {
      await assert.rejects(client(sidecar.endpoint, receipt).search("paxeer"), refused(code));
      assert.equal(sidecar.sent.length, 1);
    } finally {
      await sidecar.close();
    }
  }
}

async function web_search_client_verifies_the_payment_with_pinned_receipt_keys(): Promise<void> {
  const receipt = signedReceipt();
  for (const [signed, pins] of [
    [receipt, [key]],
    [signedReceipt(new Uint8Array(32).fill(10)), [key]],
    [signedReceipt(new Uint8Array(32).fill(10), new Uint8Array(32).fill(11)), [key]],
    [receipt, []],
    [receipt, [{ ...key, first_height: 1001n }]],
    [signedReceipt(secret, key.keyId, 1), [key]],
  ] as const) {
    const sidecar = await seller(signed);
    try {
      const search = client(sidecar.endpoint, signed, pins).search("paxeer");
      if (signed === receipt && pins.length === 1 && pins[0] === key) {
        assert.equal((await search).settlement?.transaction, transactionHash);
      } else {
        await assert.rejects(search, refused("receipt-unverified"));
      }
      assert.equal(sidecar.sent.length, 1);
    } finally {
      await sidecar.close();
    }
  }
}

test(web_settlement_transaction_is_the_draw_transaction_hash);
test(web_search_client_verifies_the_payment_with_pinned_receipt_keys);
