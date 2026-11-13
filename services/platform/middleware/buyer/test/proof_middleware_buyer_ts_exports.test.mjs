/*
Proves: buyer/src/index.ts BuyerMiddleware.prepare, captureSettlement and grantPaymentHeader -> @sidiora/layerx-buyer-middleware exports -> express/next mounts, buyer-agent.runBuyerAgent, run-reference-apps.paidMerchantOutcome, conformance.captureServiceSettlement/captureConformanceSettlement.
Repairs: buyer/src/index.ts retained mutable supported-payment configuration and
left paid response bodies open on invalid settlements; Express/Next entrypoints
omitted grantPaymentHeader and the buyer evidence/configuration/result types.
The module boundary has no durable state: every repeated observation below runs
production parsing, signing, transport and verification again. Conformance's
real receipt producer signs fresh transactions and receipts; no export, verifier,
consumer or HTTP transport is substituted. Upstream human records are test inputs.
Source-only consumer packages are loaded directly from TypeScript, never stale
dist entries. SDK, seller and buyer dist entries are built in the declared gate.
*/
import assert from "node:assert/strict";
import { once } from "node:events";
import { existsSync, readFileSync } from "node:fs";
import { createServer } from "node:http";
import { registerHooks } from "node:module";
import { fileURLToPath, pathToFileURL } from "node:url";
import test from "node:test";
import { buyerExportDrawFixture } from "./support/grant-draw-walkthrough.mjs";
import ts from "typescript";
import { ProductionClient, SecretBytes } from "@sidiora/layerx-sdk";
import {
  LAYERX_LEDGER_ADDRESS, MiddlewareError, PAYMENT_REQUIRED_HEADER,
  PAYMENT_RESPONSE_HEADER, PAYMENT_SIGNATURE_HEADER, ReceiptPayloadAuthority,
  SellerMiddleware, decodePaymentPayloadHeader, encodeLedgerDraw,
  encodePaymentRequiredHeader, encodeSettlementHeader,
} from "@sidiora/layerx-seller-middleware";
import { BuyerMiddleware, LayerXPaymentHttpTransport, accountIdentifiers, grantPaymentHeader } from "@sidiora/layerx-buyer-middleware";

const root = new URL("../../../../../", import.meta.url);
// Compile the actual consumer sources in memory. The declared gate builds only
// SDK/seller/buyer; importing another workspace package's dist would be stale.
const sourcePackages = new Map([
  ["@sidiora/layerx-express", "sdk/integrations/express/src/index.ts"],
  ["@sidiora/layerx-next", "sdk/integrations/next/src/index.ts"],
  ["@sidiora/layerx-agent-integrations", "sdk/integrations/agents/src/index.ts"],
  ["@sidiora/layerx-agent-middleware", "services/platform/middleware/agent/src/index.ts"],
  ["@sidiora/layerx-merchant-middleware", "services/platform/middleware/merchant/src/index.ts"],
]);
registerHooks({
  resolve(specifier, context, nextResolve) {
    const source = sourcePackages.get(specifier);
    if (source !== undefined) return nextResolve(new URL(source, root).href, context);
    if (specifier.startsWith(".") && specifier.endsWith(".js") && context.parentURL?.startsWith(root.href)) {
      let candidate = new URL(specifier.replace(/\.js$/u, ".ts"), context.parentURL);
      if (context.parentURL.endsWith("/conformance/service.mjs") && specifier.startsWith("./dist/")) {
        candidate = new URL(specifier.replace("./dist/", "./src/").replace(/\.js$/u, ".ts"), context.parentURL);
      }
      if (existsSync(candidate)) return nextResolve(candidate.href, context);
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url.startsWith(root.href) && url.endsWith(".ts")) {
      const source = readFileSync(new URL(url), "utf8");
      return { format: "module", shortCircuit: true, source: ts.transpileModule(source, {
        fileName: fileURLToPath(url), compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
      }).outputText };
    }
    return nextLoad(url, context);
  },
});
const express = await import(new URL("sdk/integrations/express/src/index.ts", root));
const next = await import(new URL("sdk/integrations/next/src/index.ts", root));
const { runBuyerAgent } = await import("../../../examples/buyer-agent/index.mjs");
const { paidMerchantOutcome } = await import("../../../examples/run-reference-apps.mjs");
const { captureServiceSettlement, servicePayee } = await import("../../conformance/service.mjs");
const { captureConformanceSettlement } = await import("../../conformance/src/scenarios.ts");
const { ConformanceSequencer, buildSignedReceipt, fixedBytes, offerFixture, toHex } = await import("../../conformance/src/receipts.ts");
const { FixedReceiptKeys, InMemoryFulfillmentRepository, InMemoryDeliveryStore } = await import("../../conformance/src/support.ts");
const hex = (value) => `0x${toHex(value)}`;
const bytes = (value) => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
const middlewareFailure = (code) => (error) => error instanceof MiddlewareError && error.code === code;

async function producerState(t) {
  const sequencer = await ConformanceSequencer.generate();
  const receiptKeys = await new FixedReceiptKeys([sequencer.receiptKey()]).resolve();
  const account = "agent:did:layerx:buyer-exports-seller:main";
  const payTo = await servicePayee(account);
  assert.deepEqual(payTo, (await accountIdentifiers(account))[1]);
  const facts = { payTo, asset: fixedBytes(0xbb), amount: 250000n, from: fixedBytes(0xcc),
    batchId: fixedBytes(0xd1), previousStateRoot: fixedBytes(0xd2), resultingStateRoot: fixedBytes(0xd3) };
  const signed = await buildSignedReceipt(sequencer, facts);
  const offer = offerFixture(payTo, facts.asset, facts.amount);
  const requirements = { ...offer.requirements, extra: { layerx: {
    ...offer.requirements.extra.layerx, account, currency: "LXP", payer: toHex(facts.from),
  } } };
  const required = { ...offer.paymentRequired, accepts: [requirements] };
  const header = encodePaymentRequiredHeader(required);
  const fulfillments = new InMemoryFulfillmentRepository();
  const seller = new SellerMiddleware({ paymentRequired: required, receiptKeys,
    authority: new ReceiptPayloadAuthority(receiptKeys), fulfillments });
  const counts = new Map();
  const state = { malformed: undefined, bodyReads: 0, signed, receiptKeys, requirements, required, header, sequencer, facts };
  const reference = { evidence_id: "evidence-exports", class: "layerx-receipt", verification: "receipt-verified" };
  const journey = { journey_id: "journey-exports", kind: "move", state: "done", state_copy_key: "journey.done",
    evidence: [reference], stages: [{ stage_id: "payment", copy_key: "payment.done", state: "done", evidence: [reference] }],
    started_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z" };
  const quote = { quote_id: "quote-exports", description_copy_key: "move.description", mechanism: "transfer",
    money: { amount: requirements.amount, currency: "LXP" }, fee_estimate: { amount: "0", currency: "LXP" },
    fee_ceiling: { amount: "0", currency: "LXP" }, arrival_estimate: "2026-01-01T00:00:00Z", expires_at: "2999-01-01T00:00:00Z" };
  const server = createServer((request, response) => {
    void (async () => {
      counts.set(request.url, (counts.get(request.url) ?? 0) + 1);
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const body = chunks.length === 0 ? undefined : JSON.parse(Buffer.concat(chunks).toString());
      const json = (status, result) => response.writeHead(status, { "content-type": "application/json" }).end(JSON.stringify({ ok: true, result }));
      if (request.url.startsWith("/v1/")) assert.equal(request.headers.authorization, "Bearer exports-proof-token");
      if (request.url === "/v1/moves/quote") {
        assert.equal(request.method, "POST");
        assert.deepEqual(body, { source: "acct:buyer-exports", destination: account, money: quote.money });
        return json(200, quote);
      }
      if (request.url === "/v1/moves") {
        assert.deepEqual(body, { quote_id: quote.quote_id });
        assert.ok(request.headers["idempotency-key"]);
        return json(200, { ...journey, state: "processing" });
      }
      if (request.url === "/v1/journeys/journey-exports") {
        return json(200, state.malformed === "journey" ? { ...journey, evidence: null } : journey);
      }
      if (request.url === "/v1/evidence/evidence-exports") {
        const material = { ...reference, content_type: "application/vnd.layerx.receipt-proof-v2+json",
          bytes_base64: Buffer.from(JSON.stringify(state.malformed === "proof" ? { ...signed, receiptDigest: "0x00" } : signed)).toString("base64") };
        return json(200, state.malformed === "material" ? { ...material, bytes_base64: 42 } : material);
      }
      if (request.url === "/paid") {
        const decision = await seller.handle("buyer-exports", request.headers[PAYMENT_SIGNATURE_HEADER.toLowerCase()], async () => "paid-resource");
        for (const [key, value] of Object.entries(decision.headers)) response.setHeader(key, value);
        if (decision.kind === "released") {
          if (state.malformed === "settlement") {
            response.setHeader(PAYMENT_RESPONSE_HEADER, encodeSettlementHeader({
              ...decision.settlement, extensions: { layerx: { ...signed, receiptDigest: "0x00" } },
            }));
            state.cancelledBody = once(response, "close", { signal: AbortSignal.timeout(2000) });
            response.writeHead(200);
            response.write("unverified response body remains open until the buyer cancels");
            return;
          }
          state.bodyReads += 1;
          return response.writeHead(200).end(JSON.stringify({ state: "paid", order: state.malformed === "order" ? null : {
            state: "paid-verified", orderId: "exports-checkout", receiptDigest: signed.receiptDigest.slice(2),
          } }));
        }
        return response.writeHead(decision.status).end(JSON.stringify(decision.body));
      }
      response.writeHead(404).end();
    })().catch((error) => { response.destroy(error); state.serverError = error; });
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(async () => {
    server.closeAllConnections();
    await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    assert.equal(state.serverError, undefined);
  });
  state.url = `http://127.0.0.1:${server.address().port}`;
  state.counts = counts;
  state.reference = reference;
  state.journey = journey;
  const token = new SecretBytes(new TextEncoder().encode("exports-proof-token"));
  t.after(() => token.destroy());
  state.config = { client: new ProductionClient(new LayerXPaymentHttpTransport({ baseUrl: state.url, bearerToken: token })),
    source: "acct:buyer-exports", supported: [{ scheme: "exact", network: "eip155:125" }], receiptKeys,
    retry: { baseDelayMs: 0, maximumQuoteAttempts: 1, maximumCommitAttempts: 1, maximumJourneyPolls: 1 } };
  return state;
}

function mountOptions(state) {
  return {
    environment: {
      LAYERX_PRINCIPAL: "exports-seller", LAYERX_PROTECTED_PATH: "/paid", LAYERX_PROTOCOL_VERSION: "3",
      LAYERX_RESOURCE_URL: `${state.url}/paid`, LAYERX_X402_SCHEME: "exact", LAYERX_X402_NETWORK: "eip155:125",
      LAYERX_PRICE: state.requirements.amount, LAYERX_ASSET: state.requirements.asset, LAYERX_PAY_TO: state.requirements.payTo,
      LAYERX_PAYMENT_TIMEOUT_SECONDS: "120", LAYERX_HUMAN_URL: state.url, LAYERX_SOURCE: "acct:buyer-exports",
      LAYERX_TOKEN: "exports-proof-token", LAYERX_WEBHOOK_PATH: "/webhook",
      LAYERX_WEBHOOK_PUBLIC_KEYS_JSON: JSON.stringify({ seq: Buffer.from(state.sequencer.publicKey).toString("base64") }),
      LAYERX_RECEIPT_KEYS_JSON: JSON.stringify(state.receiptKeys.map((key) => ({
        keyId: hex(key.keyId), publicKey: hex(key.publicKey), firstHeight: key.first_height.toString(), lastHeight: key.last_height.toString(),
      }))),
    },
    fulfillments: new InMemoryFulfillmentRepository(), deliveries: new InMemoryDeliveryStore(),
    resources: { async release() { return { contentType: "text/plain", body: "paid" }; } },
    events: { async handle() {} },
  };
}

async function proofMiddlewareBuyerTsExports(t) {
  const state = await producerState(t);
  assert.equal(express.BuyerMiddleware, BuyerMiddleware);
  assert.equal(next.BuyerMiddleware, BuyerMiddleware);
  assert.equal(express.grantPaymentHeader, grantPaymentHeader);
  assert.equal(next.grantPaymentHeader, grantPaymentHeader);
  const mounts = [express.createLayerXMount(mountOptions(state)), next.mountLayerX(mountOptions(state))];
  t.after(() => mounts.forEach((mount) => mount.destroy()));
  for (const mount of mounts) {
    assert.ok(mount.buyer instanceof BuyerMiddleware);
    for (let round = 0; round < 2; round += 1) {
      const before = new Map(state.counts);
      const result = await mount.buyer.fetch(`${state.url}/paid`, {}, `exports-${round}`);
      assert.equal(result.kind, "paid");
      const payment = result.payment;
      assert.deepEqual(payment.journey, state.journey);
      assert.deepEqual(payment.journey.evidence, [state.reference]);
      assert.deepEqual(payment.journey.stages[0].evidence, [state.reference]);
      assert.deepEqual(payment.offer, mount.buyer.parseOffer(state.header));
      assert.equal(payment.quote.quote_id, "quote-exports");
      assert.equal(payment.idempotencyKey, `exports-${round}`);
      assert.equal(payment.receiptDigest, state.signed.receiptDigest);
      assert.equal(payment.txHash, state.signed.txHash);
      assert.deepEqual(decodePaymentPayloadHeader(payment.paymentHeader), payment.payload);
      assert.equal(result.settlement.verification.rung, "receipted");
      assert.equal(result.settlement.txHash, payment.txHash);
      const settlementHeader = result.response.headers.get(PAYMENT_RESPONSE_HEADER);
      for (const consume of [captureServiceSettlement, captureConformanceSettlement]) {
        const first = await consume(mount.buyer, settlementHeader, payment);
        const second = await consume(mount.buyer, settlementHeader, payment);
        assert.equal(first.receiptDigest, state.signed.receiptDigest);
        assert.equal(second.txHash, first.txHash);
        assert.notEqual(first.verification, second.verification);
        await assert.rejects(consume(mount.buyer, encodeSettlementHeader({ ...first.response,
          extensions: { layerx: { ...state.signed, receiptDigest: "0x00" } },
        }), payment), middlewareFailure("verification-failure"));
      }
      const outcome = await paidMerchantOutcome(result, "proof", "exports-checkout");
      assert.equal(outcome.state, "paid");
      assert.equal(outcome.receiptDigest, state.signed.receiptDigest.slice(2));
      for (const route of ["/v1/moves/quote", "/v1/moves", "/v1/journeys/journey-exports", "/v1/evidence/evidence-exports"]) {
        assert.equal(state.counts.get(route), (before.get(route) ?? 0) + 1, `fresh producer read for ${route}`);
      }
      assert.equal(state.counts.get("/paid"), (before.get("/paid") ?? 0) + 2);
    }
    for (const malformed of ["journey", "material", "proof", "settlement"]) {
      state.malformed = malformed;
      await assert.rejects(mount.buyer.fetch(`${state.url}/paid`, {}, `malformed-${malformed}`),
        (error) => error.code === (malformed === "journey" || malformed === "material" ? "decode-failure" : "verification-failure"));
      if (malformed === "settlement") await state.cancelledBody;
    }
    state.malformed = undefined;
  }
  const supported = [{ scheme: "exact", network: "eip155:125" }];
  const stable = new BuyerMiddleware({ ...state.config, supported });
  supported[0].network = "eip155:1";
  assert.equal(stable.parseOffer(state.header).accepted.network, "eip155:125");
  const wrong = new BuyerMiddleware({ ...state.config, receiptKeys: state.receiptKeys.map((key) => ({ ...key, publicKey: fixedBytes(0xff) })) });
  await assert.rejects(wrong.prepare(state.header, "wrong-signer"), middlewareFailure("verification-failure"));
}

test(proofMiddlewareBuyerTsExports);

async function proofBuyerAgentObservesFreshResultsAndRefusal(t) {
  const state = await producerState(t);
  const environment = { BUYER_EXPORTS_TOKEN: "exports-proof-token", BUYER_EXPORTS_SOURCE: "acct:buyer-exports",
    BUYER_EXPORTS_KEY: "exports-buyer-agent", BUYER_EXPORTS_RECEIPT_KEYS: mountOptions(state).environment.LAYERX_RECEIPT_KEYS_JSON };
  const previous = Object.fromEntries(Object.keys(environment).map((key) => [key, process.env[key]]));
  Object.assign(process.env, environment);
  t.after(() => { for (const [key, value] of Object.entries(previous)) { if (value === undefined) delete process.env[key]; else process.env[key] = value; } });
  const config = { name: "proof", humanUrl: state.url, resourceUrl: `${state.url}/paid`, scheme: "exact", network: "eip155:125",
    tokenEnvironment: "BUYER_EXPORTS_TOKEN", sourceEnvironment: "BUYER_EXPORTS_SOURCE", idempotencyEnvironment: "BUYER_EXPORTS_KEY",
    receiptKeysEnvironment: "BUYER_EXPORTS_RECEIPT_KEYS" };
  for (let round = 1; round <= 2; round += 1) {
    const outcome = await runBuyerAgent(config);
    assert.equal(outcome.state, "paid");
    assert.equal(outcome.transactionHash, state.signed.txHash);
    assert.equal(outcome.receiptDigest, state.signed.receiptDigest);
    assert.equal(outcome.rung, "receipted");
    assert.equal(JSON.parse(outcome.body).order.state, "paid-verified");
    assert.equal(state.counts.get("/v1/evidence/evidence-exports"), round);
  }
  state.malformed = "proof";
  assert.deepEqual(await runBuyerAgent(config), { application: "buyer-agent", environment: "proof", state: "refused", detail: "verification-failure" });
  state.malformed = undefined;
  const buyer = new BuyerMiddleware(state.config);
  const paid = await buyer.fetch(`${state.url}/paid`, {}, "wrong-checkout");
  await assert.rejects(paidMerchantOutcome(paid, "proof", "unknown-checkout"), /merchant_reference_missing_verified_order/u);
  state.malformed = "order";
  const malformed = await buyer.fetch(`${state.url}/paid`, {}, "malformed-checkout");
  assert.equal(malformed.kind, "paid");
  await assert.rejects(paidMerchantOutcome(malformed, "proof", "exports-checkout"), /invalid_application_data/u);
}

test(proofBuyerAgentObservesFreshResultsAndRefusal);

async function proofGrantPaymentHeaderReachesIntegrationConsumers(t) {
  const state = await producerState(t);
  const draw = { recipient: state.facts.payTo, purposeHash: fixedBytes(0x22), amount: 25n, grantId: fixedBytes(0x33), sequence: 1n,
    idempotencyKey: fixedBytes(0x44), contextHash: fixedBytes(0x55) };
  assert.notDeepEqual(draw.recipient, state.facts.from);
  const transaction = hex(state.sequencer.signTransaction({ chainId: 125n, nonce: 0n, maxPriorityFeePerGas: 1n, maxFeePerGas: 2n,
    gasLimit: 200000n, to: bytes(LAYERX_LEDGER_ADDRESS), value: 0n, data: encodeLedgerDraw(draw), accessList: [] }));
  for (const scheme of ["metered", "subscription"]) {
    const accepted = { ...state.requirements, scheme, amount: "25", asset: hex(state.facts.asset), payTo: hex(draw.recipient), extra: { layerx: { commitment: "receipted",
      payer: toHex(state.facts.from), purposeHash: toHex(draw.purposeHash), contextHash: toHex(draw.contextHash),
      grant: { grantId: toHex(draw.grantId), asset: hex(state.facts.asset) },
      ...(scheme === "subscription" ? { windowSeconds: "3600" } : {}),
    } } };
    const required = { ...state.required, accepts: [accepted] };
    const buyer = new BuyerMiddleware({ ...state.config, supported: [{ scheme, network: accepted.network }] });
    for (const consumer of [express, next]) {
      for (let round = 0; round < 2; round += 1) {
        const header = consumer.grantPaymentHeader(required, accepted, transaction);
        assert.equal(header, buyer.grantHeader(encodePaymentRequiredHeader(required), transaction));
        const observed = decodePaymentPayloadHeader(header);
        assert.deepEqual(observed.accepted, accepted);
        assert.equal(observed.payload.rawTransaction, transaction);
        assert.equal(observed.payload.idempotencyKey, toHex(draw.idempotencyKey));
      }
      await assert.rejects(async () => consumer.grantPaymentHeader(required, { ...accepted, amount: "24" }, transaction), middlewareFailure("requirements-mismatch"));
      assert.throws(() => consumer.grantPaymentHeader(required, accepted, "0x00"), middlewareFailure("requirements-mismatch"));
    }
  }
}

test(proofGrantPaymentHeaderReachesIntegrationConsumers);

function proofBuyerTypesReachIntegrationExports() {
  const producerPath = fileURLToPath(new URL("services/platform/middleware/buyer/dist/index.d.ts", root));
  const consumers = ["express", "next"].map((name) => fileURLToPath(new URL(`sdk/integrations/${name}/src/index.ts`, root)));
  const program = ts.createProgram([producerPath, ...consumers], {
    module: ts.ModuleKind.NodeNext, moduleResolution: ts.ModuleResolutionKind.NodeNext,
    target: ts.ScriptTarget.ES2022, strict: true, noEmit: true,
  });
  const checker = program.getTypeChecker();
  const exportsOf = (path) => {
    const source = program.getSourceFile(path);
    assert.ok(source, `real module source exists: ${path}`);
    const symbol = checker.getSymbolAtLocation(source);
    assert.ok(symbol, `real module symbol exists: ${path}`);
    return new Map(checker.getExportsOfModule(symbol).map((entry) => [entry.name,
      entry.flags & ts.SymbolFlags.Alias ? checker.getAliasedSymbol(entry) : entry]));
  };
  const producer = exportsOf(producerPath);
  for (const path of consumers) {
    const consumer = exportsOf(path);
    for (const name of ["BuyerMiddleware", "BuyerMiddlewareConfig", "EvidenceReference", "EvidenceMaterial",
      "Journey", "ParsedOffer", "PreparedPayment", "CapturedSettlement", "grantPaymentHeader", "LayerXPaymentHttpTransport"]) {
      assert.ok(producer.get(name), `producer declares ${name}`);
      assert.equal(consumer.get(name), producer.get(name), `${path} exports the actual producer ${name}`);
    }
  }
}

test(proofBuyerTypesReachIntegrationExports);

test(buyerExportDrawFixture);
