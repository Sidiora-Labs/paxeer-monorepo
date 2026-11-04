import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { isDeepStrictEqual } from "node:util";
import "./revert.test.js";
import "./rpc-node.test.js";
import "./rpc-subscription-eth.test.js";
import "./rpc.test.js";
import "./sequencer-receipt-verify.test.js";
import "./web-search-sequencer.test.js";
import "./x402-draw-sequencer.test.js";
import { webSearchClientTests } from "./web-search.test.js";
import { decodePayerGrant, resolveWebLedgerAccount, webSearchPayerOwner, verifyWebSearchPayment, WebSearchError, type WebSearchOffer } from "../src/web-search.js";
import { decodeLedgerTransaction } from "../src/ledger-payment.js";
import { encodeSequencerLogEffect } from "../src/sequencer-log.js";
import { walletAccount } from "../src/rpc.js";
import { verifySequencerReceipt } from "../src/verifier.js";
import { buyer, exactRecording, ledgerRecording, meteredRules, recordedWebRpc, sellerExchange, unhex, webConfig, webReceiptKeys, localWebFixture, type RecordedRpc } from "./recorded-web-search.js";

async function repair_22_33_24_replays_regenerated_metered_evidence(): Promise<void> {
  const root = new URL("../../../../../../", import.meta.url);
  const provenance = localWebFixture<{ readonly sources: Readonly<Record<string, string>> }>("provenance.json");
  for (const [path, digest] of Object.entries(provenance.sources)) {
    assert.equal(createHash("sha256").update(readFileSync(new URL(path, root))).digest("hex"), digest, path);
  }
  const historical = JSON.parse(readFileSync(new URL("services/interop/crates/x-websearch/tests/fixtures/gateway/metered.json", root), "utf8")) as {
    readonly endpoints: { readonly gateway: { readonly "*": readonly RecordedRpc[] } };
  };
  const submissions = meteredRules.filter(rule => rule.method === "eth_sendRawTransaction");
  assert.equal(submissions.length, 4);
  for (const [index, currency] of (["SID", "PAX", "USDC", "USDL"] as const).entries()) {
    const submission = submissions[index]!;
    assert.deepEqual(submission, historical.endpoints.gateway["*"].find(rule => rule.method === submission.method && rule.result === submission.result));
    const result = meteredRules.find(rule => rule.method === "pax_sequencerReceipt" && rule.params[0] === submission.result)!.result as {
      readonly receipt: string; readonly logs: readonly { readonly address: string; readonly topics: readonly string[]; readonly data: string }[];
    };
    const raw = unhex(String(submission.params[0]));
    const calldata = decodeLedgerTransaction(raw).data;
    const word = (index: number): string => Buffer.from(calldata.subarray(4 + index * 32, 36 + index * 32)).toString("hex");
    const grant = decodePayerGrant(unhex(buyer.grants[currency]));
    const offer: WebSearchOffer = { scheme: "metered", network: "eip155:125", amount: webConfig.assets[currency].price,
      asset: webConfig.assets[currency].asset_id, payTo: word(1), currency, account: "",
      payer: grant.from, purposeHash: word(5), maxTimeoutSeconds: 60 };
    const verified = await verifyWebSearchPayment(unhex(result.receipt), raw, offer, webReceiptKeys, word(3), grant);
    assert.equal(Buffer.from(verified.receipt.activityId).toString("hex"), String(submission.result).slice(2));
    assert.deepEqual(verified.receipt.effects.slice(1).map(effect => effect.body), result.logs.map(log =>
      encodeSequencerLogEffect(unhex(log.address), log.topics.map(unhex), unhex(log.data))));
    assert.equal(result.logs.length, 1);
    assert.equal(result.logs[0]!.data.slice(-64), word(6));
    assert.notEqual(word(6), "00".repeat(32));
    const old = historical.endpoints.gateway["*"].find(rule => rule.method === "pax_sequencerReceipt" && rule.params[0] === submission.result)!.result as { readonly receipt: string };
    await verifySequencerReceipt(unhex(old.receipt), raw, webReceiptKeys);
    await assert.rejects(verifyWebSearchPayment(unhex(old.receipt), raw, offer, webReceiptKeys, word(3), grant),
      error => error instanceof WebSearchError && error.code === "receipt-offer-mismatch");
  }
}

async function repair_22_33_24_replays_current_ledger_reads(): Promise<void> {
  const root = new URL("../../../../../../", import.meta.url);
  for (const [path, digest] of Object.entries(ledgerRecording.sources)) {
    assert.equal(createHash("sha256").update(readFileSync(new URL(path, root))).digest("hex"), digest, path);
  }
  assert.equal(ledgerRecording.calls.length, 8);
  const payerAddress = sellerExchange[0]!.request.headers["LAYERX-PAYER-ADDRESS"];
  assert.equal(typeof payerAddress, "string");
  const owner = webSearchPayerOwner(payerAddress as string);
  const rpc = await recordedWebRpc();
  try {
    for (const currency of ["SID", "PAX", "USDC", "USDL"] as const) {
      const asset = webConfig.assets[currency].asset_id;
      const grant = decodePayerGrant(unhex(buyer.grants[currency]));
      const resolved = await resolveWebLedgerAccount(rpc.client, owner, asset, webConfig.assets.PAX.asset_id);
      assert.equal(resolved, grant.from, currency);
      const seller = sellerExchange[2 + 2 * ["SID", "PAX", "USDC", "USDL"].indexOf(currency)]!;
      const required = seller.response.headers["PAYMENT-REQUIRED"] as {
        readonly accepts: readonly { readonly scheme: string; readonly payTo: string;
          readonly extra: { readonly layerx: { readonly currency: string; readonly account: string } } }[];
      };
      const offer = required.accepts.find(entry => entry.scheme === "exact" && entry.extra.layerx.currency === currency);
      assert(offer);
      const did = offer.extra.layerx.account.split(":").slice(1, 4).join(":");
      assert.equal(await resolveWebLedgerAccount(rpc.client, did.slice("did:layerx:".length), asset, webConfig.assets.PAX.asset_id), offer.payTo);
      if (currency === "PAX") {
        const assetAccount = await rpc.client.ledgerAccount(owner, asset);
        assert.notEqual(assetAccount, grant.from);
        assert.equal(resolved, walletAccount(`did:layerx:${owner}`, asset, webConfig.assets.PAX.asset_id));
      }
    }
    for (const submission of meteredRules.filter(rule => rule.method === "eth_sendRawTransaction")) {
      assert.equal(typeof submission.result, "string");
      assert.deepEqual(await rpc.client.getRawTransaction(submission.result as string), unhex(String(submission.params[0])));
    }
    assert(rpc.calls.every(call => call.path === "/"));
    assert(rpc.calls.some(call => call.method === "eth_getRawTransactionByHash"));
    assert(rpc.calls.some(call => call.method === "eth_call" && (call.params[0] as { readonly data: string }).data.includes(owner)));
    const accountReads = new Set(rpc.calls.filter(call => call.method === "eth_call").map(call => (call.params[0] as { readonly data: string }).data));
    assert.equal(accountReads.size, 8);
    for (const recorded of ledgerRecording.calls) {
      assert(rpc.calls.some(call => call.method === recorded.method && isDeepStrictEqual(call.params, recorded.params)));
    }
  } finally { await rpc.close(); }
}

async function repair_22_33_24_replays_recorded_payment_reads(): Promise<void> {
  const payments = [
    ...meteredRules.filter(rule => rule.method === "eth_sendRawTransaction").map(rule => ({
      txHash: String(rule.result), rawTransaction: String(rule.params[0]),
    })),
    ...Object.values(exactRecording.payments),
  ];
  assert.equal(payments.length, 8);
  assert.equal(new Set(payments.map(payment => payment.txHash)).size, 8);
  const rpc = await recordedWebRpc();
  try {
    for (const payment of payments) {
      const raw = await rpc.client.getRawTransaction(payment.txHash);
      assert.deepEqual(raw, unhex(payment.rawTransaction));
      const result = await rpc.client.sequencerReceipt(payment.txHash);
      assert.equal(result["txHash"], payment.txHash);
      assert.equal(result["rung"], "receipted");
      assert.equal(typeof result["receipt"], "string");
      const verified = await verifySequencerReceipt(unhex(String(result["receipt"])), raw, webReceiptKeys);
      assert.equal(`0x${Buffer.from(verified.receipt.activityId).toString("hex")}`, payment.txHash);
      assert.equal(verified.receipt.resultCode, 0);
    }
    assert.equal(rpc.calls.length, 16);
    assert(rpc.calls.every(call => call.path === "/"));
    assert.deepEqual(rpc.calls.map(call => call.method), payments.flatMap(() => ["eth_getRawTransactionByHash", "pax_sequencerReceipt"]));
  } finally { await rpc.close(); }
}

function repair_22_33_24_preserves_recorded_producer_provenance(): void {
  const root = new URL("../../../../../../", import.meta.url);
  const source = (path: string): string => readFileSync(new URL(path, root), "utf8");
  const committed = JSON.parse(source("services/agent/sdk/typescript/test/fixtures/repair-22-33-24/committed.json")) as {
    readonly provenance: { readonly source: string; readonly sha256: string };
  };
  assert.equal(createHash("sha256").update(source(committed.provenance.source)).digest("hex"), committed.provenance.sha256);
  // These source checks pin the transcript's routing boundary; they do not claim to execute Go or Rust handlers.
  assert.match(source("rpc/rpcstack.go"), /if path == "" \{\s*return r.URL.Path == "\/"/u);
  assert.match(source("services/platform/hosted/gateway/src/main.rs"), /if let Some\(result\) = public_reads::route\(config, request\)/u);
  assert.match(
    source("services/platform/hosted/gateway/src/public_reads.rs"),
    /if matches!\(request.path.as_str\(\), "\/" \| "\/openrpc.json"\) \{\s*return Some\(super::rpc::route\(config, request\)\);/u,
  );
  const gatewayRpc = source("services/platform/hosted/gateway/src/rpc.rs");
  assert.match(gatewayRpc, /if request.path != "\/" \{\s*return response\(404, "not_found", None\);/u);
  assert.match(gatewayRpc, /if request.method != "POST" \{\s*return response\(405, "method_not_allowed", None\);/u);
  assert.match(source("services/interop/crates/x-websearch/src/payment.rs"), /PAYER_ADDRESS: &str = "LAYERX-PAYER-ADDRESS"/u);
}

test(repair_22_33_24_preserves_recorded_producer_provenance);
test(repair_22_33_24_replays_current_ledger_reads);
test(repair_22_33_24_replays_recorded_payment_reads);
test(repair_22_33_24_replays_regenerated_metered_evidence);
test(webSearchClientTests);
