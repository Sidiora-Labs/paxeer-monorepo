import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { test, type TestContext } from "node:test";
import { ed25519 } from "@noble/curves/ed25519.js";
import { encodeSequencerReceipt } from "../src/sequencer-receipt-codec.js";
import { verifySequencerReceipt, type SequencerReceipt, type SequencerReceiptKey } from "../src/verifier.js";
import { WebSearchClient, WebSearchError, decodePayerGrant, WEB_SEARCH_PAYER_HEADER,
  type WebSearchCurrency, type WebSearchScheme } from "../src/web-search.js";
import { buyer, exactRecording, meteredRules, recordedSeller, recordedWebRpc, sellerExchange,
  unhex, webConfig, webReceiptKeys, type SellerStep } from "./recorded-web-search.js";

const hex = (bytes: Uint8Array): string => Buffer.from(bytes).toString("hex");
type Json = Record<string, unknown>;
const layerx = (paid: SellerStep): Json => ((paid.response.headers["PAYMENT-RESPONSE"] as Json)["extensions"] as Json)["layerx"] as Json;
const digest = (receipt: Uint8Array): string => createHash("sha256").update("LXP/v1/receipt\0")
  .update(receipt.subarray(0, receipt.length - 69)).update(Uint8Array.of(0)).digest("hex");

function recording(scheme: WebSearchScheme, currency: WebSearchCurrency = "SID") {
  const index = scheme === "metered" ? 1 : { SID: 3, PAX: 5, USDC: 7, USDL: 9 }[currency];
  const challenge = structuredClone(sellerExchange[index - 1]!);
  const paid = structuredClone(sellerExchange[index]!);
  const settlement = paid.response.headers["PAYMENT-RESPONSE"] as Json;
  const receipt = new Uint8Array(Buffer.from(String(layerx(paid)["receipt"]), "base64"));
  const payment = paid.request.headers["PAYMENT-SIGNATURE"] as { readonly payload: { readonly idempotencyKey: string } };
  const transactionHash = String(settlement["transaction"]);
  const submission = meteredRules.find(rule => rule.method === "eth_sendRawTransaction" && rule.result === transactionHash);
  const raw = unhex(scheme === "metered" ? String(submission!.params[0]) : exactRecording.payments[currency].rawTransaction);
  return { challenge, paid, receipt, raw, payment, transactionHash, currency, scheme };
}

async function purchase(entry: ReturnType<typeof recording>, keys: readonly SequencerReceiptKey[] = webReceiptKeys) {
  const rpc = await recordedWebRpc();
  const peer = await recordedSeller(entry.challenge, entry.paid);
  const payerAddress = entry.challenge.request.headers[WEB_SEARCH_PAYER_HEADER];
  const client = new WebSearchClient({
    endpoint: peer.endpoint, network: "eip155:125", nativeAsset: webConfig.assets.PAX.asset_id,
    ledgerRpc: rpc.client, receiptKeys: keys,
    assets: { [entry.currency]: { assetId: webConfig.assets[entry.currency].asset_id, maxAmount: BigInt(webConfig.assets[entry.currency].price) } },
    payer: {
      ...(typeof payerAddress === "string" ? { address: payerAddress } : {}),
      preferences: [{ currency: entry.currency, scheme: entry.scheme }],
      exact: async () => ({ receipt: entry.receipt, txHash: entry.transactionHash, rawTransaction: entry.raw, idempotencyKey: entry.payment.payload.idempotencyKey }),
      metered: async () => ({ grant: unhex(buyer.grants[entry.currency]), idempotencyKey: entry.payment.payload.idempotencyKey }),
    }, now: () => 1_000_000_000n,
  });
  return { peer, rpc, run: () => entry.scheme === "metered" ? client.search("paxeer")
    : client.fetch(new URL(entry.challenge.request.target, peer.endpoint).searchParams.get("url")!),
    close: async () => { await peer.close(); await rpc.close(); } };
}

function refused(code: string): (error: unknown) => boolean {
  return error => { assert(error instanceof WebSearchError); assert.equal(error.code, code); return true; };
}

// Re-signing is confined to explicit refusal vectors; no positive receipt is reissued.
async function negativeMutation(entry: ReturnType<typeof recording>, change: (receipt: SequencerReceipt) => SequencerReceipt): Promise<ReturnType<typeof recording>> {
  const decoded = await verifySequencerReceipt(entry.receipt, entry.raw, webReceiptKeys);
  const changed = change(structuredClone(decoded.receipt));
  const seed = new Uint8Array(createHash("sha256").update("x-websearch-test/key/sequencer").digest());
  assert.deepEqual(ed25519.getPublicKey(seed), webReceiptKeys[0]!.publicKey);
  const receipt = encodeSequencerReceipt({ ...changed,
    sequencerSignature: ed25519.sign(new Uint8Array(createHash("sha256").update("LXP/v1/receipt\0").update(encodeSequencerReceipt(changed, false)).digest()), seed) });
  layerx(entry.paid)["receipt"] = Buffer.from(receipt).toString("base64");
  layerx(entry.paid)["receiptDigest"] = digest(receipt);
  return { ...entry, receipt: new Uint8Array(receipt) };
}

async function web_settlement_transaction_is_the_draw_transaction_hash(): Promise<void> {
  const entry = recording("metered");
  const verified = await verifySequencerReceipt(entry.receipt, entry.raw, webReceiptKeys);
  assert.equal(`0x${hex(verified.receipt.activityId)}`, entry.transactionHash);
  const grant = decodePayerGrant(unhex(buyer.grants.SID));
  assert.equal(grant.grant_id, (entry.paid.request.headers["PAYMENT-SIGNATURE"] as { payload: { grantId: string } }).payload.grantId);
  const peer = await purchase(entry);
  try {
    const found = await peer.run();
    assert.equal(found.settlement?.transaction, entry.transactionHash);
    assert.equal(found.settlement?.scheme, "metered");
    assert.equal(peer.peer.sent.length, 1);
    assert.equal(peer.peer.sent[0]?.target, "/search?q=paxeer");
    assert.equal(peer.peer.sent[0]?.headers["layerx-payer-address"], entry.challenge.request.headers[WEB_SEARCH_PAYER_HEADER]);
  } finally { await peer.close(); }
}

async function web_search_client_verifies_the_payment_with_pinned_receipt_keys(context: TestContext): Promise<void> {
  const original = recording("exact");
  const key = webReceiptKeys[0]!;
  for (const [name, keys] of [
    ["wrong public key", [{ ...key, publicKey: ed25519.getPublicKey(new Uint8Array(32).fill(10)) }]],
    ["unknown key id", [{ ...key, keyId: new Uint8Array(32).fill(11) }]],
    ["height outside range", [{ ...key, first_height: 10n, last_height: 11n }]],
  ] as const) await context.test(name, async () => {
    const peer = await purchase(recording("exact"), keys);
    try { await assert.rejects(peer.run(), refused("receipt-unverified")); assert.equal(peer.peer.sent.length, 0); }
    finally { await peer.close(); }
  });
  const rpc = await recordedWebRpc();
  try {
    assert.throws(() => new WebSearchClient({ endpoint: rpc.endpoint, network: "eip155:125", nativeAsset: webConfig.assets.PAX.asset_id,
      assets: {}, payer: { preferences: [] }, receiptKeys: [] }), refused("receipt-keys-required"));
  } finally { await rpc.close(); }
  for (const name of ["failed result", "corrupted signature", "unknown signing key"] as const) await context.test(name, async () => {
    let entry = recording("exact");
    if (name === "failed result") entry = await negativeMutation(entry, receipt => ({ ...receipt, resultCode: 1 }));
    else if (name === "unknown signing key") {
      const verified = await verifySequencerReceipt(entry.receipt, entry.raw, webReceiptKeys);
      const hash = new Uint8Array(createHash("sha256").update("LXP/v1/receipt\0").update(encodeSequencerReceipt(verified.receipt, false)).digest());
      entry = { ...entry, receipt: new Uint8Array(encodeSequencerReceipt({ ...verified.receipt,
        sequencerSignature: ed25519.sign(hash, new Uint8Array(32).fill(10)) })) };
    } else {
      const receipt = entry.receipt.slice(); receipt[receipt.length - 1]! ^= 1;
      entry = { ...entry, receipt };
    }
    const peer = await purchase(entry);
    try { await assert.rejects(peer.run(), refused("receipt-unverified")); }
    finally { await peer.close(); }
  });
  assert.deepEqual(original.receipt, new Uint8Array(Buffer.from(String(layerx(original.paid)["receipt"]), "base64")));
}

async function web_search_receipt_shape_matches_sequencer(context: TestContext): Promise<void> {
  for (const currency of ["SID", "PAX", "USDC", "USDL"] as const) await context.test(`${currency} unmodified recorded exact payment`, async () => {
    const entry = recording("exact", currency);
    const verified = await verifySequencerReceipt(entry.receipt, entry.raw, webReceiptKeys);
    assert.deepEqual(encodeSequencerReceipt(verified.receipt), entry.receipt);
    assert.equal(verified.receipt.moduleId, 12); assert.equal(verified.receipt.operation, 2);
    const peer = await purchase(entry);
    try {
      const found = await peer.run();
      assert.equal(found.settlement?.transaction, entry.transactionHash);
      assert.equal(found.settlement?.currency, currency);
      assert.equal(found.settlement?.receiptDigest, digest(entry.receipt));
      assert.equal(peer.peer.sent.length, 1);
      assert.deepEqual(peer.peer.sent[0]?.payment, entry.paid.request.headers["PAYMENT-SIGNATURE"]);
    } finally { await peer.close(); }
  });
  for (const [moduleId, operation] of [[1, 6], [1, 2], [13, 2], [12, 6]] as const) await context.test(`refuse receipt shape ${moduleId}/${operation}`, async () => {
    const entry = await negativeMutation(recording("exact"), receipt => ({ ...receipt, moduleId, operation }));
    const peer = await purchase(entry);
    try { await assert.rejects(peer.run(), refused("settlement-operation-mismatch")); }
    finally { await peer.close(); }
  });
  for (const field of ["amount", "asset", "recipient", "payer"] as const) await context.test(`refuse wrong receipt ${field}`, async () => {
    const entry = await negativeMutation(recording("exact"), receipt => ({ ...receipt, effects: receipt.effects.map((effect, index) => {
      if (index === 0) return effect;
      const body = effect.body.slice();
      const offset = { amount: 21 + 4 * 32, asset: 22 + 4 * 32, recipient: 21 + 3 * 32, payer: 21 + 2 * 32 }[field];
      body[offset]! ^= 1;
      return { ...effect, body };
    }) }));
    const peer = await purchase(entry);
    try { await assert.rejects(peer.run(), refused("receipt-offer-mismatch")); }
    finally { await peer.close(); }
  });
  for (const [name, mutate, code] of [
    ["wrong settlement payer", (entry: ReturnType<typeof recording>) => { (entry.paid.response.headers["PAYMENT-RESPONSE"] as Json)["payer"] = "2b".repeat(32); }, "settlement-payer-mismatch"],
    ["wrong transaction hash", (entry: ReturnType<typeof recording>) => { (entry.paid.response.headers["PAYMENT-RESPONSE"] as Json)["transaction"] = `0x${"11".repeat(32)}`; }, "settlement-receipt-mismatch"],
    ["malformed transaction", (entry: ReturnType<typeof recording>) => { layerx(entry.paid)["rawTransaction"] = "0x0"; }, "settlement-receipt-mismatch"],
    ["altered transaction", (entry: ReturnType<typeof recording>) => { const raw = entry.raw.slice(); raw[raw.length - 1]! ^= 1; layerx(entry.paid)["rawTransaction"] = `0x${hex(raw)}`; }, "settlement-receipt-mismatch"],
    ["wrong digest", (entry: ReturnType<typeof recording>) => { layerx(entry.paid)["receiptDigest"] = "2b".repeat(32); }, "settlement-receipt-mismatch"],
  ] as const) await context.test(name, async () => {
    const entry = recording("exact"); mutate(entry);
    const peer = await purchase(entry);
    try { await assert.rejects(peer.run(), refused(code)); assert.equal(peer.peer.sent.length, 1); }
    finally { await peer.close(); }
  });
}

test(web_settlement_transaction_is_the_draw_transaction_hash);
test(web_search_client_verifies_the_payment_with_pinned_receipt_keys);
test(web_search_receipt_shape_matches_sequencer);
