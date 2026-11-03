import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import { test, type TestContext } from "node:test";
import { ed25519 } from "@noble/curves/ed25519.js";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { keccak_256 } from "@noble/hashes/sha3.js";
import { decodeRawEip1559, encodeUnsignedEip1559 } from "../src/evm-transaction.js";
import { walletAccount } from "../src/rpc.js";
import { verifySequencerReceipt, type SequencerReceiptKey as ReceiptKey } from "../src/verifier.js";
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
  readonly signedReceipt: string;
  readonly rawTransaction: string;
  readonly receiptJson: { readonly from: string; readonly to: string; readonly txHash: string };
  readonly pinnedKey: { readonly keyId: string; readonly publicKey: string; readonly firstHeight: number; readonly lastHeight: number };
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
const fixtureSecret = new Uint8Array(32).fill(0x5e);
const key: ReceiptKey = {
  keyId: bytes(vector.pinnedKey.keyId), publicKey: bytes(vector.pinnedKey.publicKey),
  first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight),
};
const nativeAsset = "0e237584332fc3e1f1672d80a167a8d027597a64b34f4be18951e60babdd4f73";
const payerDid = "did:layerx:buyer";
const sellerDid = "did:layerx:seller";
const sellerAccount = { name: `agent:${sellerDid}:main`, id: walletAccount(sellerDid, nativeAsset, nativeAsset) };
const offer = {
  scheme: "metered", network: `eip155:${vectors.chainId}`,
  amount: decodeRawEip1559(bytes(vector.rawTransaction)).value.toString(),
  asset: nativeAsset, payTo: sellerAccount.id, maxTimeoutSeconds: 30,
  extra: { layerx: {
    account: sellerAccount.name, commitment: "executed", currency: "PAX",
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
const drawTransaction = Buffer.concat([Buffer.of(2, 0xf9), transactionLength, transactionPayload]);
const rawTransaction = bytes(vector.rawTransaction);
const transactionHash = vector.receiptJson.txHash;
const producerReceipt = bytes(vector.signedReceipt);
const signingKey: ReceiptKey = { ...key, publicKey: ed25519.getPublicKey(secret) };
const summary = Buffer.concat([
  uint(12n, 2), uint(1n, 4), uint(0n, 4), Buffer.of(2), bounded(bytes(offer.asset)),
]);
const summaryOffset = producerReceipt.indexOf(summary);
const amountOffset = summaryOffset + summary.length;
const fromOffset = amountOffset + 16 + 4;
const toOffset = fromOffset + 32 + 16 + 16 + 8 + 4;

function resignReceipt(change: (receipt: Buffer) => void, signingSecret = secret): Buffer {
  const receipt = Buffer.from(producerReceipt);
  bytes(offer.extra.layerx.payer).copy(receipt, fromOffset);
  bytes(offer.payTo).copy(receipt, toOffset);
  change(receipt);
  const fields = receipt.subarray(0, receipt.length - 69);
  const digest = createHash("sha256").update("LXP/v1/receipt\0").update(fields).update(Buffer.of(0)).digest();
  return Buffer.concat([fields, Buffer.of(1), bounded(ed25519.sign(digest, signingSecret))]);
}

function signedReceipt(signingSecret?: Uint8Array, keyId = key.keyId, resultCode = 0): Buffer {
  if (signingSecret === undefined) return resignReceipt(() => {}, fixtureSecret);
  return resignReceipt((receipt) => {
    receipt.writeInt32BE(resultCode, 6 + 36 + 8 + 36 * 3);
    Buffer.from(keyId).copy(receipt, receipt.length - 109);
  }, new Uint8Array(signingSecret));
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
  const decoded = decodeRawEip1559(new Uint8Array(drawTransaction));
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

async function web_search_receipt_shape_matches_sequencer(context: TestContext): Promise<void> {
  const verified = await verifySequencerReceipt(producerReceipt, rawTransaction, [key]);
  assert.equal(verified.receipt.moduleId, 12);
  assert.equal(verified.receipt.operation, 2);
  assert.equal(verified.receipt.amount.toString(), offer.amount);
  assert.equal(hex(verified.receipt.asset), offer.asset);
  assert.equal(hex(verified.receipt.from), vector.receiptJson.from.slice(2).padStart(64, "0"));
  assert.equal(hex(verified.receipt.to), vector.receiptJson.to.slice(2).padStart(64, "0"));
  assert.equal(`0x${hex(keccak_256(rawTransaction))}`, transactionHash);
  assert.equal(hex(ed25519.getPublicKey(fixtureSecret)), hex(key.publicKey));
  assert.ok(summaryOffset >= 0);
  assert.equal(producerReceipt.indexOf(summary, summaryOffset + 1), -1);

  for (const scheme of ["exact", "metered"] as const) {
    const receipt = signedReceipt();
    const matching = await verifySequencerReceipt(receipt, rawTransaction, [key]);
    assert.equal(matching.receipt.moduleId, 12);
    assert.equal(matching.receipt.operation, 2);
    assert.equal(matching.receipt.amount, verified.receipt.amount);
    assert.deepEqual(matching.receipt.asset, verified.receipt.asset);
    assert.equal(hex(matching.receipt.from), offer.extra.layerx.payer);
    assert.equal(hex(matching.receipt.to), offer.payTo);
    const expectedFields = Buffer.from(producerReceipt.subarray(0, producerReceipt.length - 64));
    bytes(offer.extra.layerx.payer).copy(expectedFields, fromOffset);
    bytes(offer.payTo).copy(expectedFields, toOffset);
    assert.deepEqual(receipt.subarray(0, receipt.length - 64), expectedFields);
    await context.test(`${scheme} accepts the producer receipt with matching accounts`, async () => {
      const sidecar = await seller(receipt, scheme);
      try {
        const found = await client(sidecar.endpoint, receipt, [key], scheme).search("paxeer");
        assert.deepEqual(found.settlement, {
          scheme, currency: "PAX", network: offer.network, asset: offer.asset, amount: offer.amount,
          payer: offer.extra.layerx.payer,
          receiptDigest: createHash("sha256").update("LXP/v1/merkle-leaf\0").update(receipt).digest("hex"),
          transaction: transactionHash,
        });
        assert.equal(found.results[0]?.snippet, "Paid search");
        assert.equal(sidecar.sent.length, 1);
        assert.equal(sidecar.sent[0]!["target"], "/search?q=paxeer");
        const sent = sidecar.sent[0]!["payment"] as { readonly payload: Json };
        if (scheme === "metered") assert.deepEqual(sent.payload, payment);
        else {
          assert.equal(sent.payload["receipt"], receipt.toString("base64"));
          assert.equal(sent.payload["rawTransaction"], vector.rawTransaction);
        }
      } finally {
        await sidecar.close();
      }
    });

    for (const [moduleId, operation] of [[1, 6], [1, 2], [13, 2], [12, 6]] as const) {
      await context.test(`${scheme} refuses receipt shape ${moduleId}/${operation}`, async () => {
        const receipt = resignReceipt((changed) => {
          changed.writeUInt16BE(moduleId, summaryOffset);
          changed[summaryOffset + 10] = operation;
        });
        const changed = await verifySequencerReceipt(receipt, rawTransaction, [signingKey]);
        assert.equal(changed.receipt.moduleId, moduleId);
        assert.equal(changed.receipt.operation, operation);
        assert.equal(changed.receipt.amount, verified.receipt.amount);
        assert.deepEqual(changed.receipt.asset, verified.receipt.asset);
        assert.deepEqual(changed.receipt.from, matching.receipt.from);
        assert.deepEqual(changed.receipt.to, matching.receipt.to);
        const sidecar = await seller(receipt, scheme);
        try {
          await assert.rejects(client(sidecar.endpoint, receipt, [signingKey], scheme).search("paxeer"),
            refused("settlement-operation-mismatch"));
        } finally {
          await sidecar.close();
        }
      });
    }

    const failedResult = signedReceipt(secret, key.keyId, 1);
    assert.equal((await verifySequencerReceipt(failedResult, rawTransaction, [signingKey])).receipt.resultCode, 1);
    const changedAmount = resignReceipt((receipt) => uint(BigInt(offer.amount) + 1n, 16).copy(receipt, amountOffset));
    const changedAsset = resignReceipt((receipt) => receipt.fill(0x2b, amountOffset - 32, amountOffset));
    const changedRecipient = resignReceipt((receipt) => receipt.fill(0x2b, toOffset, toOffset + 32));
    const changedPayer = resignReceipt((receipt) => receipt.fill(0x2b, fromOffset, fromOffset + 32));
    const alteredTransaction = Buffer.from(rawTransaction);
    alteredTransaction[alteredTransaction.length - 1]! ^= 1;
    const cases: readonly {
      readonly name: string;
      readonly code: string;
      readonly receipt: Uint8Array;
      readonly pins: readonly ReceiptKey[];
      readonly overrides?: Json;
      readonly layerxOverrides?: Json;
    }[] = [
      { name: "wrong public key", code: "receipt-unverified", receipt, pins: [signingKey] },
      { name: "unknown key ID", code: "receipt-unverified", receipt, pins: [{ ...key, keyId: new Uint8Array(32).fill(11) }] },
      { name: "missing pins", code: "receipt-unverified", receipt, pins: [] },
      { name: "key outside height range", code: "receipt-unverified", receipt, pins: [{ ...key, first_height: 1001n }] },
      { name: "failed result", code: "receipt-unverified", receipt: failedResult, pins: [signingKey] },
      { name: "wrong receipt amount", code: "receipt-offer-mismatch", receipt: changedAmount, pins: [signingKey] },
      { name: "wrong receipt asset", code: "receipt-offer-mismatch", receipt: changedAsset, pins: [signingKey] },
      { name: "wrong receipt recipient", code: "receipt-offer-mismatch", receipt: changedRecipient, pins: [signingKey] },
      { name: "wrong settlement payer", code: "settlement-payer-mismatch", receipt, pins: [key], overrides: { payer: "2b".repeat(32) } },
      { name: "wrong transaction hash", code: "settlement-receipt-mismatch", receipt, pins: [key], overrides: { transaction: `0x${"11".repeat(32)}` } },
      { name: "altered transaction", code: scheme === "exact" ? "settlement-receipt-mismatch" : "receipt-unverified", receipt, pins: [key], layerxOverrides: { rawTransaction: `0x${hex(alteredTransaction)}` } },
      { name: "malformed transaction", code: "settlement-receipt-mismatch", receipt, pins: [key], layerxOverrides: { rawTransaction: "0x0" } },
      { name: "wrong digest", code: "settlement-receipt-mismatch", receipt, pins: [key], layerxOverrides: { receiptDigest: "2b".repeat(32) } },
      ...(scheme === "metered" ? [
        { name: "wrong receipt payer", code: "settlement-payer-mismatch", receipt: changedPayer, pins: [signingKey], overrides: { payer: "2b".repeat(32) } },
        { name: "wrong purpose", code: "settlement-purpose-mismatch", receipt, pins: [key], layerxOverrides: { purposeHash: "2b".repeat(32) } },
      ] : []),
    ];
    for (const entry of cases) {
      await context.test(`${scheme} refuses ${entry.name}`, async () => {
        const sidecar = await seller(entry.receipt, scheme, entry.overrides, entry.layerxOverrides);
        try {
          await assert.rejects(client(sidecar.endpoint, entry.receipt, entry.pins, scheme).search("paxeer"), refused(entry.code));
        } finally {
          await sidecar.close();
        }
      });
    }
  }
}

test(web_settlement_transaction_is_the_draw_transaction_hash);
test(web_search_client_verifies_the_payment_with_pinned_receipt_keys);
test(web_search_receipt_shape_matches_sequencer);
