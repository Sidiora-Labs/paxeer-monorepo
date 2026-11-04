import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { createRequire, registerHooks } from "node:module";
import { createInterface } from "node:readline";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = new URL("../../../../", import.meta.url);
const require = createRequire(new URL("package.json", root));
const packages = new Map([
  ["@sidiora/layerx-sdk", "services/agent/sdk/typescript/src/index.ts"],
  ["@sidiora/layerx-seller-middleware", "services/platform/middleware/seller/src/index.ts"],
  ["@sidiora/layerx-buyer-middleware", "services/platform/middleware/buyer/src/index.ts"],
]);
let resolvingDependency = false;
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (packages.has(specifier)) {
      return { url: new URL(packages.get(specifier), root).href, shortCircuit: true };
    }
    if (specifier.startsWith(".") && specifier.endsWith(".js") && context.parentURL?.endsWith(".ts")) {
      const source = new URL(specifier.slice(0, -3) + ".ts", context.parentURL);
      if (existsSync(fileURLToPath(source))) return { url: source.href, shortCircuit: true };
    }
    if (!resolvingDependency && (specifier.startsWith("@noble/") || specifier.startsWith("@scure/") || specifier === "ws")) {
      resolvingDependency = true;
      try {
        return { url: pathToFileURL(require.resolve(specifier)).href, shortCircuit: true };
      } finally {
        resolvingDependency = false;
      }
    }
    return nextResolve(specifier, context);
  },
});
const { ProductionClient, SecretBytes } = await import("@sidiora/layerx-sdk");
const { BuyerMiddleware, LayerXPaymentHttpTransport } = await import("@sidiora/layerx-buyer-middleware");
const { PaymentRpc, decodePaymentRequiredHeader, verifyRpcPayment } = await import("@sidiora/layerx-seller-middleware");
const vectors = JSON.parse(readFileSync(new URL("sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", root)));
assert.equal(vectors.provenance.generator, "rpc/sequencer_receipt_vectors_test.go");
const vector = vectors.cases.find((row) => row.id === "V1-layerxledger-pay-exact");
const bytes = (hex) => new Uint8Array(Buffer.from(hex.slice(2), "hex"));
const pin = { keyId: bytes(vector.pinnedKey.keyId), publicKey: bytes(vector.pinnedKey.publicKey),
  first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight) };
let buyer;
let payment;
let proof;
for await (const line of createInterface({ input: process.stdin })) {
  try {
    const request = JSON.parse(line);
    if (request.action === "prepare") {
      const challenge = await fetch(`${request.url}/recorded-paid`);
      assert.equal(challenge.status, 402);
      const header = challenge.headers.get("PAYMENT-REQUIRED");
      const offer = decodePaymentRequiredHeader(header);
      assert.equal(offer.accepts[0].network, "eip155:125");
      const rpc = new PaymentRpc(`${request.url}/rpc`);
      const recorded = await rpc.receipt(vector.receiptJson.txHash);
      const produced = await verifyRpcPayment(recorded, vector.rawTransaction,
        vector.receiptJson.from.slice(2).padStart(64, "0"), offer.accepts[0], [pin]);
      assert.equal(produced.kind, "verified");
      proof = Object.fromEntries(["format", "verificationLevel", "rawTransaction", "signedReceipt", "receiptDigest", "receiptKey"]
        .map((key) => [key, produced[key]]));
      const staged = await fetch(`${request.url}/recorded-evidence`, {
        method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(proof),
      });
      assert.equal(staged.status, 204);
      buyer = new BuyerMiddleware({
        client: new ProductionClient(new LayerXPaymentHttpTransport({ baseUrl: request.url,
          bearerToken: new SecretBytes(new TextEncoder().encode("recorded-session")) })),
        source: "acct:buyer-portable-receipts", supported: [{ scheme: "exact", network: "eip155:125" }],
        receiptKeys: [pin], now: () => Date.parse("2026-08-18T09:32:00Z"),
        retry: { maximumQuoteAttempts: 1, maximumCommitAttempts: 1, maximumJourneyPolls: 1, baseDelayMs: 0 },
      });
      const prepared = await buyer.prepare(header, "repair-22-31-287");
      assert.equal(prepared.kind, "ready");
      payment = prepared.payment;
      assert.deepEqual(payment.payload.payload, proof);
      process.stdout.write(JSON.stringify({ header: payment.paymentHeader, proof }) + "\n");
    } else if (request.action === "pay") {
      const response = await fetch(`${request.url}/recorded-paid`, {
        headers: { "PAYMENT-SIGNATURE": payment.paymentHeader },
      });
      assert.equal(response.status, 200);
      const header = response.headers.get("PAYMENT-RESPONSE");
      const captured = await buyer.captureSettlement(header, payment);
      assert.deepEqual(captured.response.extensions.layerx, proof);
      assert.equal(captured.response.transaction, payment.txHash);
      assert.equal(response.headers.get("layerx-transaction"), payment.txHash);
      assert.equal(response.headers.get("layerx-receipt-digest"), proof.receiptDigest.slice(2));
      process.stdout.write(JSON.stringify({ header, settlement: captured.response, body: await response.text() }) + "\n");
    } else {
      assert.equal(request.action, "close");
      break;
    }
  } catch (error) {
    process.stdout.write(JSON.stringify({ error: String(error), stack: error.stack }) + "\n");
  }
}
