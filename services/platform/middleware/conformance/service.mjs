import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { LayerXBudgetLedger, LayerXReceiptResolver, LayerXRemoteSigner, LayerXServiceEndpoint } from "@sidiora/layerx-agent-integrations";
import { createServer } from "node:http";
import { mkdtemp, open, readFile, rename, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  encodePaymentPayloadHeader,
  verifyPaymentReceipt,
} from "@sidiora/layerx-seller-middleware";
import {
  BuyerMiddleware,
  LayerXPaymentHttpTransport,
  accountIdentifiers,
} from "@sidiora/layerx-buyer-middleware";
import { ProductionClient, SecretBytes, decodeRawEip1559, eip1559SigningHash } from "@sidiora/layerx-sdk";
import {
  ConformanceSequencer,
  buildSignedReceipt,
  buyerPayload,
  fixedBytes,
  toHex,
} from "./dist/receipts.js";
import { FixedReceiptKeys, assert } from "./dist/support.js";

const EXAMPLE_ENTRY = fileURLToPath(new URL("../../examples/paid-api/index.mjs", import.meta.url));
const MERCHANT_ENTRY = fileURLToPath(new URL("../../examples/merchant-shop/index.mjs", import.meta.url));
const AGENT_ENTRY = fileURLToPath(new URL("../../examples/agent-spend/index.mjs", import.meta.url));

export const SERVICE_PAYEE_ACCOUNT = "agent:did:layerx:conformance-seller:main";

/**
 * Derives the payee account identifier the advertised `extra.layerx.account`
 * resolves to, using the same derivation the buyer middleware applies in
 * `accountNamesPayTo` and the interop adapter applies in
 * `services/interop/crates/layerx-x402/src/model.rs`. The native (`LX:ACCOUNT:v1`)
 * identifier is the current one; the legacy digest is the protocol-2 form the
 * buyer still accepts.
 */
export async function servicePayee(account) {
  const identifiers = await accountIdentifiers(account);
  const native = identifiers[identifiers.length - 1];
  if (native === undefined || native.length !== 32) throw new Error("invalid_service_payee_account");
  return native;
}

export async function assertServiceAccountPayTo(account, payTo) {
  for (const candidate of await accountIdentifiers(account)) {
    if (toHex(candidate) === payTo) return;
  }
  throw new Error(`LAYERX_PAY_TO ${payTo} does not derive from LAYERX_ACCOUNT ${account}`);
}

export async function runServiceScenarios(suite) {
  const sequencer = await ConformanceSequencer.generate();
  const payTo = await servicePayee(SERVICE_PAYEE_ACCOUNT);
  const asset = fixedBytes(0xbb);
  const amount = 250_000n;
  const facts = {
    asset,
    payTo,
    amount,
    from: fixedBytes(0xcc),
    batchId: fixedBytes(0xd1),
    previousStateRoot: fixedBytes(0xd2),
    resultingStateRoot: fixedBytes(0xd3),
  };
  const receipt = await buildSignedReceipt(sequencer, facts);
  const receiptKeys = await new FixedReceiptKeys([sequencer.receiptKey()]).resolve();
  const rpc = await listenJsonService((request, body) => receiptRpc(request, body, receipt));

  const workDir = await mkdtemp(join(tmpdir(), "layerx-mw-conformance-"));
  const resourceFile = join(workDir, "resource.json");
  const resourceBody = JSON.stringify({ secret: "paid-resource-body", price: amount.toString() });
  await writeFile(resourceFile, resourceBody, "utf8");

  const port = 8080 + Math.floor(Math.random() * 2000);
  const environment = {
    ...process.env,
    PORT: port.toString(),
    LAYERX_RESOURCE_FILE: resourceFile,
    LAYERX_EXAMPLE_STATE_ROOT: workDir,
    LAYERX_FULFILLMENT_DIR: join(workDir, "fulfillments"),
    LAYERX_RESOURCE_URL: `http://127.0.0.1:${port}/paid`,
    LAYERX_RESOURCE_DESCRIPTION: "conformance paid resource",
    LAYERX_X402_SCHEME: "exact",
    LAYERX_X402_NETWORK: "eip155:125",
    LAYERX_PRICE: amount.toString(),
    LAYERX_ASSET: toHex(asset),
    LAYERX_PAY_TO: toHex(payTo),
    LAYERX_ACCOUNT: SERVICE_PAYEE_ACCOUNT,
    LAYERX_CURRENCY: "LXP",
    LAYERX_PAYMENT_TIMEOUT_SECONDS: "120",
    LAYERX_RECEIPT_KEYS_JSON: JSON.stringify(receiptKeysJson(receiptKeys)),
  };

  const configFile = join(workDir, "example.json");
  await assertServiceAccountPayTo(environment.LAYERX_ACCOUNT, environment.LAYERX_PAY_TO);
  await writeFile(configFile, JSON.stringify({ version: 1, application: "paid-api", environments: { emulator: {
    port, resourceFile, fulfillmentDirectory: "fulfillments",
    resourceUrl: environment.LAYERX_RESOURCE_URL, scheme: "exact", network: "eip155:125",
    priceEnvironment: "LAYERX_PRICE", assetEnvironment: "LAYERX_ASSET", payToEnvironment: "LAYERX_PAY_TO",
    accountEnvironment: "LAYERX_ACCOUNT", currencyEnvironment: "LAYERX_CURRENCY",
    receiptKeysEnvironment: "LAYERX_RECEIPT_KEYS_JSON", rpcUrl: rpc.url,
  } } }));
  environment.LAYERX_EXAMPLE_CONFIG = configFile;
  const config = JSON.parse(await readFile(configFile, "utf8"));
  let child;
  try {
    child = spawn(process.execPath, [EXAMPLE_ENTRY, "--environment", "emulator"], { env: environment, stdio: ["ignore", "pipe", "pipe"] });
    await waitForListening(child, 15_000);
    const base = `http://127.0.0.1:${port}/paid`;
    const buyer = new BuyerMiddleware({
      client: new ProductionClient(new LayerXPaymentHttpTransport({
        baseUrl: `http://127.0.0.1:${port}/`,
        bearerToken: new SecretBytes(new TextEncoder().encode("unused-conformance-token")),
      })),
      source: "acct:conformance-buyer",
      supported: [{ scheme: "exact", network: "eip155:125" }],
      receiptKeys,
    });

    await suite.check("service: an unpaid request is answered with a 402 offer the buyer can parse", async () => {
      const response = await fetch(base, { method: "GET", headers: { accept: "application/json" } });
      assert(response.status === 402, `expected a 402, received ${response.status}`);
      const header = response.headers.get("PAYMENT-REQUIRED");
      assert(header !== null, "the 402 must carry a PAYMENT-REQUIRED offer");
      await response.text();
      const parsed = buyer.parseOffer(header ?? "");
      assert(parsed.accepted.amount === amount.toString(), "the parsed offer must quote the server price");
      assert(parsed.accepted.payTo === toHex(payTo), "the parsed offer must name the server payee");
    });

    let capturedVerified = false;
    await suite.check("service: a valid receipt unlocks the resource and settles over the wire", async () => {
      const offerResponse = await fetch(base, { method: "GET", headers: { accept: "application/json" } });
      const header = offerResponse.headers.get("PAYMENT-REQUIRED") ?? "";
      await offerResponse.text();
      const parsed = buyer.parseOffer(header);
      const payload = buyerPayload({ requirements: parsed.accepted, paymentRequired: parsed.required }, receipt, "service-happy");
      const paid = await fetch(base, {
        method: "GET",
        headers: { accept: "application/json", "PAYMENT-SIGNATURE": encodePaymentPayloadHeader(payload) },
      });
      const body = await paid.text();
      assert(paid.status === 200, `expected the resource to be released, received ${paid.status}: ${body}`);
      assert(body === resourceBody, "the released body must match the protected resource");
      const settlementHeader = paid.headers.get("PAYMENT-RESPONSE");
      assert(settlementHeader !== null, "a released resource must carry a PAYMENT-RESPONSE settlement");
      const prepared = await preparedPayment(parsed, receipt, receiptKeys);
      const captured = await captureServiceSettlement(buyer, settlementHeader ?? "", prepared);
      capturedVerified = captured.verification.rung === "receipted";
      assert(capturedVerified, "the buyer must verify the settlement receipt");
    });

    await suite.check("service: a tampered payment cannot unlock the resource", async () => {
      const offerResponse = await fetch(base, { method: "GET", headers: { accept: "application/json" } });
      const header = offerResponse.headers.get("PAYMENT-REQUIRED") ?? "";
      await offerResponse.text();
      const parsed = buyer.parseOffer(header);
      const tamperedBytes = Buffer.from(receipt.rawTransaction.slice(2), "hex");
      tamperedBytes[tamperedBytes.length - 1] ^= 0x01;
      const tampered = { ...receipt, rawTransaction: `0x${toHex(tamperedBytes)}` };
      const payload = buyerPayload({ requirements: parsed.accepted, paymentRequired: parsed.required }, tampered, "service-tamper");
      const response = await fetch(base, {
        method: "GET",
        headers: { accept: "application/json", "PAYMENT-SIGNATURE": encodePaymentPayloadHeader(payload) },
      });
      await response.text();
      assert(response.status !== 200, "a tampered payment must never release the resource");
    });

    await suite.check("service: replaying the same payment is idempotent", async () => {
      const offerResponse = await fetch(base, { method: "GET", headers: { accept: "application/json" } });
      const header = offerResponse.headers.get("PAYMENT-REQUIRED") ?? "";
      await offerResponse.text();
      const parsed = buyer.parseOffer(header);
      const payload = buyerPayload({ requirements: parsed.accepted, paymentRequired: parsed.required }, receipt, "service-happy");
      const encoded = encodePaymentPayloadHeader(payload);
      const first = await fetch(base, { method: "GET", headers: { accept: "application/json", "PAYMENT-SIGNATURE": encoded } });
      const second = await fetch(base, { method: "GET", headers: { accept: "application/json", "PAYMENT-SIGNATURE": encoded } });
      const firstBody = await first.text();
      const secondBody = await second.text();
      assert(first.status === 200 && second.status === 200, "both replays must release the resource");
      assert(firstBody === secondBody, "a replayed payment must return the same resource");
      assert(capturedVerified, "the earlier settlement capture must have verified");
    });
  } finally {
    if (child !== undefined) {
      child.kill("SIGKILL");
    }
    await rpc.close();
    await rm(workDir, { recursive: true, force: true });
  }
  await runMerchantServiceScenarios(suite, sequencer, facts, receipt, receiptKeys);
  await runAgentServiceScenarios(suite, receipt, receiptKeys, amount, asset, payTo);
  return { config, receiptKeysEnvironment: environment.LAYERX_RECEIPT_KEYS_JSON, receiptKey: sequencer.receiptKey() };
}

async function runMerchantServiceScenarios(suite, sequencer, facts, receipt, receiptKeys) {
  const { amount, asset, payTo } = facts;
  const workDir = await mkdtemp(join(tmpdir(), "layerx-merchant-conformance-"));
  const settlement = await listenJsonService(async (request, body) => {
    if (request.url === "/rpc") return receiptRpc(request, body, receipt);
    if (request.method !== "POST" || request.url !== "/settle") {
      return jsonReply(404, { code: "not-found" });
    }
    if (request.headers.authorization !== "Bearer merchant-conformance-token") {
      return jsonReply(401, { code: "capability-refusal" });
    }
    const settlementRequest = object(body);
    const requirements = object(settlementRequest.requirements);
    const payment = object(settlementRequest.payload);
    const evidence = object(object(payment.payload));
    if (
      requirements.amount !== amount.toString()
      || requirements.asset !== toHex(asset)
      || requirements.payTo !== toHex(payTo)
      || evidence.signedReceipt !== receipt.signedReceipt
      || evidence.receiptDigest !== receipt.receiptDigest
    ) {
      return jsonReply(402, { code: "verification-failure", retry: "never" });
    }
    return jsonReply(200, {
      state: "settled",
      transaction: receipt.txHash,
    });
  });
  const catalog = join(workDir, "catalog.json");
  await writeFile(catalog, JSON.stringify([{
    sku: "metered-report",
    title: "Conformance item",
    unitAmount: amount.toString(),
    asset: toHex(asset),
    payTo: toHex(payTo),
    scheme: "exact",
    network: "eip155:125",
    maxTimeoutSeconds: 120,
  }]), "utf8");
  const port = 10_100 + Math.floor(Math.random() * 1_000);
  const environment = {
    ...process.env,
    PORT: port.toString(),
    LAYERX_CATALOG_FILE: catalog,
    LAYERX_ORDER_DIR: join(workDir, "orders"),
    LAYERX_FULFILLMENT_DIR: join(workDir, "fulfillments"),
    LAYERX_SETTLEMENT_URL: `${settlement.url}/settle`,
    LAYERX_SETTLEMENT_TOKEN: "merchant-conformance-token",
    LAYERX_PUBLIC_URL: `http://127.0.0.1:${port}`,
  };
  const configFile = join(workDir, "example.json");
  await writeFile(configFile, JSON.stringify({ version: 1, application: "merchant-shop", environments: { emulator: {
    port, publicUrl: environment.LAYERX_PUBLIC_URL, settlementUrl: environment.LAYERX_SETTLEMENT_URL,
    rpcUrl: `${settlement.url}/rpc`, stateDirectory: ".", scheme: "exact", network: "eip155:125",
    receiptKeysEnvironment: "LAYERX_RECEIPT_KEYS_JSON",
    tokenEnvironment: "LAYERX_SETTLEMENT_TOKEN", priceEnvironment: "LAYERX_PRICE",
    assetEnvironment: "LAYERX_ASSET", payToEnvironment: "LAYERX_PAY_TO",
    accountEnvironment: "LAYERX_ACCOUNT", currencyEnvironment: "LAYERX_CURRENCY",
    webhookKeysEnvironment: "LAYERX_WEBHOOK_KEYS",
  } } }));
  Object.assign(environment, { LAYERX_EXAMPLE_CONFIG: configFile, LAYERX_PRICE: amount.toString(),
    LAYERX_ASSET: toHex(asset), LAYERX_PAY_TO: toHex(payTo),
    LAYERX_ACCOUNT: SERVICE_PAYEE_ACCOUNT, LAYERX_CURRENCY: "LXP",
    LAYERX_EXAMPLE_STATE_ROOT: workDir,
    LAYERX_RECEIPT_KEYS_JSON: JSON.stringify(receiptKeysJson(receiptKeys)),
    LAYERX_WEBHOOK_KEYS: JSON.stringify({ conformance: toHex(receiptKeys[0].publicKey) }) });
  await assertServiceAccountPayTo(environment.LAYERX_ACCOUNT, environment.LAYERX_PAY_TO);
  let child;
  try {
    child = spawn(process.execPath, [MERCHANT_ENTRY, "--environment", "emulator"], { env: environment, stdio: ["ignore", "pipe", "pipe"] });
    await waitForListening(child, 15_000);
    const checkout = `http://127.0.0.1:${port}/checkout`;
    const requestBody = {
      principal: "acct:merchant-conformance",
      checkout_key: "checkout-conformance",
      lines: [{ sku: "metered-report", quantity: 1 }],
    };
    const buyer = new BuyerMiddleware({
      client: new ProductionClient(new LayerXPaymentHttpTransport({
        baseUrl: `http://127.0.0.1:${port}/`,
        bearerToken: new SecretBytes(new TextEncoder().encode("unused-merchant-token")),
      })),
      source: "acct:merchant-conformance",
      supported: [{ scheme: "exact", network: "eip155:125" }],
      receiptKeys,
    });
    let offerHeader = "";
    await suite.check("merchant service: catalog checkout opens a receipt-gated order", async () => {
      const response = await postJson(checkout, requestBody);
      offerHeader = response.headers.get("PAYMENT-REQUIRED") ?? "";
      const body = object(await response.json());
      assert(response.status === 402 && body.state === "payment-required", "checkout must remain payment-required");
      const order = object(body.order);
      assert(order.state === "awaiting-payment", "an unpaid order must remain awaiting payment");
      const parsed = buyer.parseOffer(offerHeader);
      assert(parsed.accepted.amount === amount.toString(), "the merchant quote must preserve the exact catalog total");
    });
    await suite.check("merchant service: a real verified receipt pays and idempotently replays the order", async () => {
      const parsed = buyer.parseOffer(offerHeader);
      receipt = await buildSignedReceipt(sequencer, {
        ...facts,
        purposeHash: parsed.accepted.extra?.layerx?.purposeHash,
      });
      const payload = buyerPayload(
        { requirements: parsed.accepted, paymentRequired: parsed.required },
        receipt,
        "merchant-conformance-payment",
      );
      const headers = { "PAYMENT-SIGNATURE": encodePaymentPayloadHeader(payload) };
      const first = await postJson(checkout, requestBody, headers);
      const firstBody = object(await first.json());
      assert(first.status === 200 && firstBody.state === "paid", `a verified receipt must pay the order: ${JSON.stringify(firstBody)}`);
      const firstOrder = object(firstBody.order);
      assert(firstOrder.state === "paid-verified", "paid state must be explicitly receipt verified");
      assert(firstOrder.receiptDigest === receipt.receiptDigest.slice(2), "the stored order must bind the verified receipt digest");
      const settlementHeader = first.headers.get("PAYMENT-RESPONSE") ?? "";
      const prepared = await preparedPayment(parsed, receipt, receiptKeys);
      const captured = await buyer.captureSettlement(settlementHeader, prepared);
      assert(captured.verification.rung === "receipted", "the merchant settlement must verify locally");
      const second = await postJson(checkout, requestBody, headers);
      const secondBody = object(await second.json());
      const secondOrder = object(secondBody.order);
      assert(second.status === 200 && secondOrder.receiptDigest === receipt.receiptDigest.slice(2), "a replay must return the same paid order");
    });
    await suite.check("merchant service: tampered receipt evidence never renders a paid order", async () => {
      const tamperRequest = { ...requestBody, checkout_key: "checkout-tamper" };
      const offerResponse = await postJson(checkout, tamperRequest);
      const tamperOffer = offerResponse.headers.get("PAYMENT-REQUIRED") ?? "";
      await offerResponse.text();
      const parsed = buyer.parseOffer(tamperOffer);
      const tamperedBytes = Buffer.from(receipt.signedReceipt.slice(2), "hex");
      tamperedBytes[tamperedBytes.length - 1] ^= 0x01;
      const tampered = {
        ...receipt,
        signedReceipt: `0x${toHex(tamperedBytes)}`,
      };
      const payload = buyerPayload(
        { requirements: parsed.accepted, paymentRequired: parsed.required },
        tampered,
        "merchant-conformance-tamper",
      );
      const response = await postJson(checkout, tamperRequest, {
        "PAYMENT-SIGNATURE": encodePaymentPayloadHeader(payload),
      });
      const body = object(await response.json());
      assert(response.status !== 200 && body.state !== "paid", "tampered receipt evidence must never pay an order");
    });
  } finally {
    if (child !== undefined) child.kill("SIGKILL");
    await settlement.close();
    await rm(workDir, { recursive: true, force: true });
  }
}

async function runAgentServiceScenarios(suite, receipt, receiptKeys, amount, asset, payTo) {
  const transaction = decodeRawEip1559(Buffer.from(receipt.rawTransaction.slice(2), "hex"));
  const signingHash = toHex(eip1559SigningHash(transaction));
  const signature = new secp256k1.Signature(transaction.r, transaction.s, transaction.yParity);
  const signerPublicKey = toHex(signature.recoverPublicKey(eip1559SigningHash(transaction)).toBytes(false));
  const signatureHex = `${transaction.r.toString(16).padStart(64, "0")}${transaction.s.toString(16).padStart(64, "0")}${transaction.yParity.toString(16).padStart(2, "0")}`;
  const requests = new Map(["verified", "approval", "rate", "unknown", "pending", "failed"].map((name) => {
    const request = agentSpendRequest(`agent-${name}`, amount, asset, payTo, signerPublicKey, toHex(transaction.data));
    return [request.submitIdempotencyKey, { name, request }];
  }));
  let latestPreparation;
  const workDir = await mkdtemp(join(tmpdir(), "layerx-agent-conformance-"));
  const budgetPath = join(workDir, "budget.json");
  let budgetQueue = Promise.resolve();
  const boundary = await listenJsonService(async (request, body) => {
    const path = new URL(request.url ?? "/", "http://127.0.0.1").pathname;
    if (path === "/v1/agent/rpc") {
      const call = object(body);
      if (request.headers.authorization !== `LayerX-Key conformance:lxp_live_${"11".repeat(32)}`) {
        return jsonReply(401, { class: "CapabilityRefusal", protocol_result_code: null,
          retriability: "Terminal", reason: "invalid_credential", request_id: call.request_id });
      }
      const operation = String(call.operation ?? "");
      const payload = object(call.request);
      const success = (value) => jsonReply(200, {
        request_id: call.request_id, value, verification_status: { state: "achieved", rung: "receipted" },
      });
      if (operation === "prepare") {
        latestPreparation = payload;
        return success({
          ...payload, preparation_ref: signingHash,
          chain_id: transaction.chainId.toString(), nonce: transaction.nonce.toString(),
          max_priority_fee_per_gas: transaction.maxPriorityFeePerGas.toString(),
          max_fee_per_gas: transaction.maxFeePerGas.toString(), gas_limit: transaction.gasLimit.toString(),
          to: toHex(transaction.to), data: toHex(transaction.data), signing_hash: signingHash,
        });
      }
      if (operation === "submit" || operation === "track") {
        const scenario = operation === "submit" ? requests.get(call.idempotency_key)
          : [...requests.values()].find((entry) => payload.submission_ref === `submission:${entry.request.submitIdempotencyKey}`);
        if (scenario === undefined) throw new Error("unknown_conformance_submission");
        const key = scenario.request.submitIdempotencyKey;
        if (operation === "submit") {
          assert(payload.preparation_ref === signingHash && payload.signature === signatureHex
            && payload.signer_public_key === signerPublicKey, "submission must bind the real signed EVM transaction");
        }
        if (scenario.name === "approval") {
          return jsonReply(403, { class: "PolicyRefusal", protocol_result_code: null,
            retriability: "Terminal", reason: "approval_required", request_id: call.request_id });
        }
        if (scenario.name === "unknown") { request.socket.destroy(); return undefined; }
        if (scenario.name === "pending") {
          const observation = executedSubmission(key, receipt);
          observation.submission.receipt_ref = "receipt:pending";
          return success(observation);
        }
        if (scenario.name === "failed") {
          return success({ tx_hash: receipt.txHash.slice(2), receipt: null, submission: {
            submission_ref: `submission:${key}`, state: "Failed", result_code: "1",
            verification_level: "Submitted", evidence: [], transitions: [],
          } });
        }
        return success(executedSubmission(key, receipt));
      }
      if (operation === "approval.list") {
        return success({ approvals: [{
          approval_id: scenarioKey("approval"), canonical_digest: signingHash,
          activity_type: latestPreparation.activity_type, actor: latestPreparation.actor,
          authority: latestPreparation.authority, counterparties: [], amounts: [], asset: toHex(asset),
          fee_limit: latestPreparation.fee_limit, expiry: latestPreparation.not_after,
          idempotency_key: latestPreparation.idempotency_key, canonical_bytes_digest: signingHash,
          hold_reason_code: "approval_required", hold_reason: "owner approval required",
          created_at_sequence: "1", expires_at_sequence: "100", state: "AwaitingApproval",
        }], next_cursor: null });
      }
      throw new Error("unknown_conformance_operation");
    }
    if (request.headers.authorization !== "Bearer agent-conformance-token") {
      return jsonReply(401, { code: "capability-refusal", retry: "never" });
    }
    if (path === "/signer") {
      const signing = object(body);
      assert(signing.signing_hash === signingHash && signing.preparation_ref === signingHash,
        "the signer must receive the generated transaction's signing hash");
      if (signing.disclosure.idempotency_key === scenarioKey("agent-rate:prepare")) {
        return jsonReply(429, { code: "rate-limit", retry: "after", retry_after_ms: 500 });
      }
      return jsonReply(200, { signature: signatureHex });
    }
    if (path === "/receipt") {
      const reference = object(body).receipt_ref;
      if (reference === "receipt:pending") return jsonReply(202, { state: "pending" });
      assert(reference === "receipt:conformance" || reference === receipt.receiptDigest.slice(2),
        "the resolver must request the generated receipt reference or digest");
      return jsonReply(200, { ...receipt, receiptKey: {
        ...receipt.receiptKey,
        firstHeight: BigInt(receipt.receiptKey.firstHeight).toString(),
        lastHeight: BigInt(receipt.receiptKey.lastHeight).toString(),
      } });
    }
    if (path === "/budget") {
      const run = budgetQueue.then(() => budgetCall(budgetPath, object(body)));
      budgetQueue = run.then(() => undefined, () => undefined);
      return run;
    }
    return jsonReply(404, { code: "unavailable-capability", retry: "never" });
  });
  try {
    const baseEnvironment = {
      ...process.env,
      LAYERX_TOKEN: "agent-conformance-token",
      LAYERX_TENANT: "tenant:conformance",
      LAYERX_AGENT_SERVICES_MODULE: fileURLToPath(import.meta.url),
      LAYERX_AGENT_SURFACE: "gateway",
      LAYERX_SESSION_ID: "33".repeat(32),
      LAYERX_SESSION_TOKEN: "22".repeat(32),
      LAYERX_SESSION_GENERATION: "1",
      LAYERX_GATEWAY_KEY_ID: "conformance",
      LAYERX_GATEWAY_KEY: `lxp_live_${"11".repeat(32)}`,
      LAYERX_RECEIPT_KEYS_JSON: JSON.stringify(receiptKeys.map((key) => ({
        keyId: toHex(key.keyId), publicKey: toHex(key.publicKey),
        first_height: key.first_height.toString(), last_height: key.last_height.toString(),
      }))),
      LAYERX_AGENT_RPC_URL: boundary.url,
      LAYERX_BUDGET_SERVICE_URL: `${boundary.url}/budget`,
      LAYERX_SIGNER_SERVICE_URL: `${boundary.url}/signer`,
      LAYERX_RECEIPT_SERVICE_URL: `${boundary.url}/receipt`,
    };
    await suite.check("agent service: verified spend commits exact budget facts and replays idempotently", async () => {
      const request = agentSpendRequest("agent-verified", amount, asset, payTo, signerPublicKey, toHex(transaction.data));
      const first = await runExample(AGENT_ENTRY, {
        ...baseEnvironment,
        LAYERX_SPEND_REQUEST_JSON: JSON.stringify(request),
      });
      const firstResult = lastJsonLine(first.stdout);
      assert(first.code === 0 && firstResult.kind === "verified", `verified agent spend failed: ${first.stdout} ${first.stderr}`);
      assert(firstResult.receiptDigest === receipt.receiptDigest.slice(2), "agent result must carry the locally verified receipt digest");
      const second = await runExample(AGENT_ENTRY, {
        ...baseEnvironment,
        LAYERX_SPEND_REQUEST_JSON: JSON.stringify(request),
      });
      const secondResult = lastJsonLine(second.stdout);
      assert(second.code === 0 && secondResult.kind === "verified", "idempotent replay must return the verified result");
      const state = object(JSON.parse(await readFile(budgetPath, "utf8")));
      const record = object(object(state.records)[request.preparation.idempotency_key]);
      assert(record.state === "committed", "verified spend must durably commit its reservation");
      assert(record.amount === amount.toString() && record.asset === toHex(asset), "budget commit must preserve amount and asset");
      assert(record.receiptDigest === receipt.receiptDigest.slice(2), "budget commit must bind the verified receipt digest");
    });
    await suite.check("agent service: approval refusal becomes a durable digest-bound hold", async () => {
      const request = agentSpendRequest("agent-approval", amount, asset, payTo, signerPublicKey, toHex(transaction.data));
      const run = await runExample(AGENT_ENTRY, {
        ...baseEnvironment,
        LAYERX_SPEND_REQUEST_JSON: JSON.stringify(request),
      });
      const result = lastJsonLine(run.stdout);
      assert(run.code === 0 && result.kind === "approval-hold", `approval hold failed: ${run.stderr}`);
      assert(result.approvalId === scenarioKey("approval"), "approval result must name the daemon hold");
      const state = object(JSON.parse(await readFile(budgetPath, "utf8")));
      const record = object(object(state.records)[request.preparation.idempotency_key]);
      assert(record.state === "held" && record.approvalId === scenarioKey("approval"), "budget ledger must persist the approval hold");
      assert(record.canonicalBytesDigest === signingHash, "hold must remain bound to the prepared digest");
      const replay = await runExample(AGENT_ENTRY, {
        ...baseEnvironment,
        LAYERX_SPEND_REQUEST_JSON: JSON.stringify(request),
      });
      const replayResult = lastJsonLine(replay.stdout);
      assert(
        replay.code === 0
          && replayResult.kind === "approval-hold"
          && replayResult.approvalId === scenarioKey("approval"),
        "a process replay must restore the same durable approval hold",
      );
    });
    await suite.check("agent service: typed retry timing remains a refusal and keeps the exact reservation", async () => {
      const request = agentSpendRequest("agent-rate", amount, asset, payTo, signerPublicKey, toHex(transaction.data));
      const run = await runExample(AGENT_ENTRY, {
        ...baseEnvironment,
        LAYERX_SPEND_REQUEST_JSON: JSON.stringify(request),
      });
      const result = lastJsonLine(run.stdout);
      assert(result.kind === "refused" && result.code === "rate-limit", "rate limit must use the shared SDK code");
      assert(result.retry === "after" && result.retryAfterMs === 500, "rate limit must preserve retry timing");
      assert(result.reservationState === "reserved", "retriable refusal must keep its bound reservation");
    });
    await suite.check("agent service: an unknown submit outcome is never remapped to a safe refusal", async () => {
      const request = agentSpendRequest("agent-unknown", amount, asset, payTo, signerPublicKey, toHex(transaction.data));
      const run = await runExample(AGENT_ENTRY, {
        ...baseEnvironment,
        LAYERX_SPEND_REQUEST_JSON: JSON.stringify(request),
      });
      const result = lastJsonLine(run.stdout);
      assert(result.kind === "unknown", "a disconnected submit must remain unknown");
      assert(result.retry === undefined && result.code === undefined, "unknown outcome must not acquire a safe refusal code");
      assert(result.reservationState === "reserved", "unknown outcome must retain the budget reservation");
    });
    await suite.check("agent service: an executed state without resolved receipt evidence remains pending", async () => {
      const request = agentSpendRequest("agent-pending", amount, asset, payTo, signerPublicKey, toHex(transaction.data));
      const run = await runExample(AGENT_ENTRY, {
        ...baseEnvironment,
        LAYERX_SPEND_REQUEST_JSON: JSON.stringify(request),
      });
      const result = lastJsonLine(run.stdout);
      assert(run.code === 0 && result.kind === "pending", "missing receipt evidence must remain pending");
      assert(result.reservationState === "reserved", "pending execution must retain its exact reservation");
    });
    await suite.check("agent service: a terminal refusal durably releases with the shared taxonomy", async () => {
      const request = agentSpendRequest("agent-failed", amount, asset, payTo, signerPublicKey, toHex(transaction.data));
      const run = await runExample(AGENT_ENTRY, {
        ...baseEnvironment,
        LAYERX_SPEND_REQUEST_JSON: JSON.stringify(request),
      });
      const result = lastJsonLine(run.stdout);
      assert(result.kind === "refused" && result.code === "core-rejection", "terminal failure must use the shared code");
      assert(result.retry === "never" && result.reservationState === "released", "terminal failure must durably release");
      const state = object(JSON.parse(await readFile(budgetPath, "utf8")));
      const record = object(object(state.records)[request.preparation.idempotency_key]);
      const refusal = object(record.refusal);
      assert(
        record.state === "released" && refusal.code === "core-rejection" && refusal.submissionState === "Failed",
        "released state must preserve the terminal reason and submission state",
      );
    });
  } finally {
    await boundary.close();
    await rm(workDir, { recursive: true, force: true });
  }
}

function executedSubmission(key, receipt) {
  return {
    tx_hash: receipt.txHash.slice(2),
    submission: {
      submission_ref: `submission:${key}`, state: "Executed", receipt_ref: "receipt:conformance",
      verification_level: "Receipted", evidence: [], transitions: [],
    },
    receipt: {
      signed_receipt: receipt.signedReceipt.slice(2), raw_transaction: receipt.rawTransaction.slice(2),
      verification_level: "Receipted", receipt_key: {
        key_id: receipt.receiptKey.keyId.slice(2), public_key: receipt.receiptKey.publicKey.slice(2),
        first_height: BigInt(receipt.receiptKey.firstHeight).toString(),
        last_height: BigInt(receipt.receiptKey.lastHeight).toString(),
      },
    },
  };
}

function scenarioKey(value) {
  return createHash("sha256").update(value).digest("hex");
}

function agentSpendRequest(name, amount, asset, payTo, signerPublicKey, payload) {
  return {
    tenant: "tenant:conformance",
    preparation: {
      activity_type: "1", actor: "did:layerx:agent-conformance", authority: "did:layerx:authority-conformance",
      account_sequence: "1", not_before: "0", not_after: "4102444800000", fee_limit: "1000",
      idempotency_key: scenarioKey(`${name}:prepare`), payload,
      payload_hash: createHash("sha256").update("LXP/v1/payload-hash\0").update(Buffer.from(payload, "hex")).digest("hex"),
    },
    networkId: "125", authorityHex: "00", signerPublicKey, submitIdempotencyKey: scenarioKey(`${name}:submit`),
    approvalCurrentSequence: "1", asset: toHex(asset), amount: amount.toString(), recipient: toHex(payTo),
  };
}

export function createAgentServices(environment) {
  const token = new SecretBytes(new TextEncoder().encode(environment.LAYERX_TOKEN));
  const endpoint = (name) => new LayerXServiceEndpoint({ url: environment[name], token, timeoutMs: 30000 });
  const receipts = new LayerXReceiptResolver(endpoint("LAYERX_RECEIPT_SERVICE_URL"));
  return {
    budgets: new LayerXBudgetLedger(endpoint("LAYERX_BUDGET_SERVICE_URL")),
    signer: new LayerXRemoteSigner(endpoint("LAYERX_SIGNER_SERVICE_URL")),
    receipts: {
      async resolve(receiptRef) {
        const proof = await receipts.resolve(receiptRef);
        return { ...proof, receiptKey: {
          ...proof.receiptKey,
          firstHeight: `0x${BigInt(proof.receiptKey.firstHeight).toString(16)}`,
          lastHeight: `0x${BigInt(proof.receiptKey.lastHeight).toString(16)}`,
        } };
      },
    },
    destroy: () => token.destroy(),
  };
}

async function budgetCall(path, body) {
  const state = await readBudgetState(path);
  if (body.action === "reserve") {
    const request = object(body.request);
    const idempotencyKey = boundedString(request.idempotency_key, 255);
    const requestDigest = hexDigest(request.request_digest);
    const amount = canonicalAmount(request.amount);
    const asset = hexDigest(request.asset);
    const existing = state.records[idempotencyKey];
    if (existing !== undefined) {
      if (
        existing.requestDigest !== requestDigest
        || existing.amount !== amount
        || existing.asset !== asset
      ) {
        return jsonReply(200, { kind: "conflict" });
      }
      return jsonReply(200, { kind: "reserved", reservation: reservationWire(existing) });
    }
    const reservation = {
      reservationId: `reservation:${requestDigest.slice(0, 32)}`,
      requestDigest,
      amount,
      asset,
      state: "reserved",
    };
    state.records[idempotencyKey] = reservation;
    await writeBudgetState(path, state);
    return jsonReply(200, { kind: "reserved", reservation: reservationWire(reservation) });
  }
  const transition = object(body.transition);
  const requestDigest = hexDigest(transition.request_digest);
  const amount = canonicalAmount(transition.amount);
  const asset = hexDigest(transition.asset);
  const reservationId = boundedString(transition.reservation_id, 512);
  const entry = Object.entries(state.records).find(([, candidate]) => candidate.reservationId === reservationId);
  if (entry === undefined) {
    return jsonReply(409, { code: "idempotency-conflict", retry: "never" });
  }
  const [idempotencyKey, current] = entry;
  if (
    current.requestDigest !== requestDigest
    || current.amount !== amount
    || current.asset !== asset
  ) {
    return jsonReply(409, { code: "idempotency-conflict", retry: "never" });
  }
  let next;
  if (body.action === "hold") {
    const approvalId = boundedString(transition.approval_id, 512);
    const canonicalBytesDigest = hexDigest(transition.canonical_bytes_digest);
    if (current.state === "held") {
      if (current.approvalId !== approvalId || current.canonicalBytesDigest !== canonicalBytesDigest) {
        return jsonReply(409, { code: "idempotency-conflict", retry: "never" });
      }
      return jsonReply(200, reservationWire(current));
    }
    if (current.state !== "reserved") {
      return jsonReply(409, { code: "idempotency-conflict", retry: "never" });
    }
    next = { ...current, state: "held", approvalId, canonicalBytesDigest };
  } else if (body.action === "commit") {
    const receiptDigest = hexDigest(transition.receipt_digest);
    if (current.state === "committed") {
      if (current.receiptDigest !== receiptDigest) {
        return jsonReply(409, { code: "idempotency-conflict", retry: "never" });
      }
      return jsonReply(200, reservationWire(current));
    }
    if (current.state !== "reserved" && current.state !== "held") {
      return jsonReply(409, { code: "idempotency-conflict", retry: "never" });
    }
    next = { ...current, state: "committed", receiptDigest };
  } else if (body.action === "release") {
    const refusal = refusalFromWire(transition.refusal);
    stableRefusal(refusal);
    if (current.state === "released") {
      if (JSON.stringify(current.refusal) !== JSON.stringify(refusal)) {
        return jsonReply(409, { code: "idempotency-conflict", retry: "never" });
      }
      return jsonReply(200, reservationWire(current));
    }
    if (current.state === "committed") {
      return jsonReply(409, { code: "idempotency-conflict", retry: "never" });
    }
    next = { ...current, state: "released", refusal };
  } else {
    return jsonReply(400, { code: "invalid-argument", retry: "never" });
  }
  state.records[idempotencyKey] = next;
  await writeBudgetState(path, state);
  return jsonReply(200, reservationWire(next));
}

function reservationWire(reservation) {
  const base = {
    reservation_id: reservation.reservationId,
    request_digest: reservation.requestDigest,
    amount: reservation.amount,
    asset: reservation.asset,
    state: reservation.state,
  };
  if (reservation.state === "held") {
    return {
      ...base,
      approval_id: reservation.approvalId,
      canonical_bytes_digest: reservation.canonicalBytesDigest,
    };
  }
  if (reservation.state === "committed") {
    return { ...base, receipt_digest: reservation.receiptDigest };
  }
  if (reservation.state === "released") {
    return { ...base, refusal: refusalWire(reservation.refusal) };
  }
  return base;
}

function refusalWire(refusal) {
  return {
    code: refusal.code,
    retry: refusal.retry,
    ...(refusal.retryAfterMs === undefined ? {} : { retry_after_ms: refusal.retryAfterMs }),
    ...(refusal.protocolResultCode === undefined ? {} : { protocol_result_code: refusal.protocolResultCode }),
    ...(refusal.submissionState === undefined ? {} : { submission_state: refusal.submissionState }),
  };
}

function refusalFromWire(value) {
  const refusal = object(value);
  return {
    code: refusal.code,
    retry: refusal.retry,
    ...(refusal.retry_after_ms === undefined ? {} : { retryAfterMs: refusal.retry_after_ms }),
    ...(refusal.protocol_result_code === undefined ? {} : { protocolResultCode: refusal.protocol_result_code }),
    ...(refusal.submission_state === undefined ? {} : { submissionState: refusal.submission_state }),
  };
}

async function readBudgetState(path) {
  try {
    const parsed = object(JSON.parse(await readFile(path, "utf8")));
    const records = object(parsed.records);
    return { records: { ...records } };
  } catch (error) {
    if (error?.code === "ENOENT") return { records: {} };
    throw error;
  }
}

async function writeBudgetState(path, state) {
  const temporary = `${path}.${process.pid}.${crypto.randomUUID()}.tmp`;
  const file = await open(temporary, "wx", 0o600);
  try {
    await file.writeFile(JSON.stringify(state), "utf8");
    await file.sync();
  } finally {
    await file.close();
  }
  await rename(temporary, path);
  const directory = await open(dirname(path), "r");
  try {
    await directory.sync();
  } finally {
    await directory.close();
  }
}

function stableRefusal(value) {
  const codes = new Set([
    "invalid-argument", "idempotency-required", "transport-failure", "deadline",
    "protocol-incompatibility", "unavailable-capability", "core-rejection",
    "verification-failure", "policy-refusal", "capability-refusal", "budget-refusal",
    "rate-limit", "idempotency-conflict", "decode-failure", "unknown-outcome", "internal-fault",
  ]);
  const retries = new Set(["never", "safe", "after"]);
  if (!codes.has(value.code) || value.code === "unknown-outcome" || !retries.has(value.retry)) {
    throw new Error("invalid_refusal");
  }
  if (
    (value.retry === "after" && value.retryAfterMs === undefined)
    || (value.retry !== "after" && value.retryAfterMs !== undefined)
    || (value.retryAfterMs !== undefined && (!Number.isSafeInteger(value.retryAfterMs) || value.retryAfterMs < 0))
  ) {
    throw new Error("invalid_refusal");
  }
  if (
    value.protocolResultCode !== undefined
    && (!Number.isSafeInteger(value.protocolResultCode)
      || value.protocolResultCode < -2_147_483_648
      || value.protocolResultCode > 2_147_483_647)
  ) throw new Error("invalid_refusal");
  if (value.submissionState !== undefined && value.submissionState !== "Failed" && value.submissionState !== "Expired") {
    throw new Error("invalid_refusal");
  }
}

function canonicalAmount(value) {
  const amount = boundedString(value, 39);
  if (!/^(0|[1-9][0-9]*)$/.test(amount)) throw new Error("invalid_amount");
  return amount;
}

function hexDigest(value) {
  const digest = boundedString(value, 64);
  if (!/^[0-9a-f]{64}$/.test(digest)) throw new Error("invalid_digest");
  return digest;
}

function boundedString(value, maximum) {
  if (typeof value !== "string" || value.length === 0 || value.length > maximum || value.includes("\0")) {
    throw new Error("invalid_string");
  }
  return value;
}

export function receiptKeysJson(keys) {
  return keys.map((key) => ({
    keyId: toHex(key.keyId), publicKey: toHex(key.publicKey),
    firstHeight: key.first_height.toString(), lastHeight: key.last_height.toString(),
  }));
}

function receiptRpc(request, body, receipt) {
  const call = object(body);
  if (request.method !== "POST" || call.jsonrpc !== "2.0" || call.params?.[0] !== receipt.txHash) {
    return jsonReply(400, { jsonrpc: "2.0", id: call.id, error: { code: -32602, message: "invalid receipt request" } });
  }
  if (call.method === "eth_getRawTransactionByHash") {
    return jsonReply(200, { jsonrpc: "2.0", id: call.id, result: receipt.rawTransaction });
  }
  if (call.method !== "pax_sequencerReceipt") {
    return jsonReply(400, { jsonrpc: "2.0", id: call.id, error: { code: -32601, message: "unknown method" } });
  }
  return jsonReply(200, { jsonrpc: "2.0", id: call.id, result: {
    txHash: receipt.txHash, rung: "receipted", receipt: receipt.signedReceipt,
  } });
}

async function listenJsonService(handler) {
  const server = createServer(async (request, response) => {
    try {
      const body = await readJsonBody(request);
      const reply = await handler(request, body);
      if (reply === undefined || response.destroyed) return;
      response.writeHead(reply.status, { "content-type": "application/json", "cache-control": "no-store" });
      response.end(JSON.stringify(reply.body));
    } catch (error) {
      if (!response.destroyed) {
        response.writeHead(500, { "content-type": "application/json", "cache-control": "no-store" });
        response.end(JSON.stringify({ code: "internal-fault", retry: "unknown-outcome" }));
      }
    }
  });
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  const address = server.address();
  if (address === null || typeof address === "string") throw new Error("service did not bind a TCP port");
  return {
    url: `http://127.0.0.1:${address.port}`,
    close: () => new Promise((resolve, reject) => server.close((error) => error === undefined ? resolve() : reject(error))),
  };
}

async function readJsonBody(request) {
  const chunks = [];
  let size = 0;
  for await (const chunk of request) {
    size += chunk.length;
    if (size > 2 * 1024 * 1024) throw new Error("request_too_large");
    chunks.push(chunk);
  }
  if (size === 0) return {};
  return JSON.parse(Buffer.concat(chunks).toString("utf8"));
}

function jsonReply(status, body) {
  return { status, body };
}

function object(value) {
  if (value === null || typeof value !== "object" || Array.isArray(value)) throw new Error("invalid_object");
  return value;
}

function postJson(url, body, headers = {}) {
  return fetch(url, {
    method: "POST",
    headers: { accept: "application/json", "content-type": "application/json", ...headers },
    body: JSON.stringify(body),
  });
}

function runExample(entry, environment) {
  return new Promise((resolve, reject) => {
    const child = spawn(process.execPath, [entry], { env: environment, stdio: ["ignore", "pipe", "pipe"] });
    let stdout = "";
    let stderr = "";
    child.stdout.setEncoding("utf8");
    child.stderr.setEncoding("utf8");
    child.stdout.on("data", (chunk) => { stdout += chunk; });
    child.stderr.on("data", (chunk) => { stderr += chunk; });
    child.once("error", reject);
    child.once("exit", (code) => resolve({ code: code ?? -1, stdout, stderr }));
  });
}

function lastJsonLine(output) {
  const lines = output.trim().split("\n").filter((line) => line.length > 0);
  const last = lines.at(-1);
  if (last === undefined) throw new Error("example produced no JSON output");
  return object(JSON.parse(last));
}

async function preparedPayment(parsed, receipt, receiptKeys) {
  const verification = await verifyPaymentReceipt(
    receipt,
    parsed.accepted,
    receiptKeys,
  );
  const payload = buyerPayload({ requirements: parsed.accepted, paymentRequired: parsed.required }, receipt, "service-happy");
  return {
    offer: parsed,
    quote: {
      quote_id: "quote-service",
      description_copy_key: "move.description",
      mechanism: "transfer",
      money: { amount: parsed.accepted.amount, currency: parsed.accepted.asset },
      fee_estimate: { amount: "0", currency: parsed.accepted.asset },
      fee_ceiling: { amount: "0", currency: parsed.accepted.asset },
      arrival_estimate: "2026-01-01T00:00:00.000Z",
      expires_at: "2999-01-01T00:00:00.000Z",
    },
    journey: {
      journey_id: "journey-service",
      kind: "move",
      state: "done",
      state_copy_key: "journey.done",
      evidence: [],
      stages: [],
      started_at: "2026-01-01T00:00:00.000Z",
      updated_at: "2026-01-01T00:00:00.000Z",
    },
    payload,
    paymentHeader: encodePaymentPayloadHeader(payload),
    idempotencyKey: "service-happy",
    ...receipt,
    verification,
    receiptDigest: receipt.receiptDigest,
  };
}

function waitForListening(child, timeoutMs) {
  return new Promise((resolve, reject) => {
    let settled = false;
    let buffer = "";
    const timer = setTimeout(() => {
      if (!settled) {
        settled = true;
        reject(new Error("paid-api example did not report a listening port in time"));
      }
    }, timeoutMs);
    const finish = (error) => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timer);
      if (error === undefined) {
        resolve();
      } else {
        reject(error);
      }
    };
    child.stdout.setEncoding("utf8");
    child.stdout.on("data", (chunk) => {
      buffer += chunk;
      if (buffer.includes("\"listening\"")) {
        finish();
      }
    });
    child.stderr.setEncoding("utf8");
    child.stderr.on("data", () => undefined);
    child.on("exit", (code) => finish(new Error(`paid-api example exited early with code ${code ?? "unknown"}`)));
    child.on("error", (error) => finish(error instanceof Error ? error : new Error(String(error))));
  });
}

export async function captureServiceSettlement(buyer, header, payment) {
  const captured = await buyer.captureSettlement(header, payment);
  assert(captured.verification.rung === payment.verification.rung, "settlement must preserve the prepared verification rung");
  assert(captured.txHash === payment.txHash, "settlement must bind the prepared transaction");
  return captured;
}
