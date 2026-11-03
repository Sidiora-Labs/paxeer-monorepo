import { pathToFileURL } from "node:url";
import { BuyerMiddleware as BuyerMiddlewareImplementation, LayerXPaymentHttpTransport } from "@sidiora/layerx-buyer-middleware";
import { PlatformSdkError, ProductionClient, SecretBytes } from "@sidiora/layerx-sdk";
import {
  LayerXApplicationStateError,
  loadApplicationConfig,
  receiptKeysFromEnvironment,
  requiredEnvironment,
} from "../support/runtime.mjs";

const BuyerMiddleware = BuyerMiddlewareImplementation;

export function platform_ref_buyer() {
  return "buyer-middleware-metered-api-receipt-verified";
}

export async function runBuyerAgent(config) {
  const token = new SecretBytes(new TextEncoder().encode(requiredEnvironment(config.tokenEnvironment)));
  try {
    const buyer = new BuyerMiddleware({
      client: new ProductionClient(new LayerXPaymentHttpTransport({ baseUrl: config.humanUrl, bearerToken: token })),
      source: requiredEnvironment(config.sourceEnvironment),
      supported: [{ scheme: config.scheme, network: config.network }],
      receiptKeys: receiptKeysFromEnvironment(config.receiptKeysEnvironment),
    });
    const result = await buyer.fetch(
      config.resourceUrl,
      { method: "GET", headers: { accept: "application/json" } },
      requiredEnvironment(config.idempotencyEnvironment),
    );
    if (result.kind === "pending" || result.kind === "unknown" || result.kind === "refused") {
      return { application: "buyer-agent", environment: config.name, state: result.kind, result };
    }
    if (result.kind === "not-payment-required") {
      await result.response.body?.cancel();
      throw new LayerXApplicationStateError("refused", `metered_resource_did_not_require_payment_http_${result.response.status}`);
    }
    if (!result.response.ok) {
      await result.response.body?.cancel();
      throw new LayerXApplicationStateError("unknown", `paid_resource_http_${result.response.status}`);
    }
    return {
      application: "buyer-agent", environment: config.name, state: "paid", status: result.response.status,
      transactionHash: result.settlement.txHash, receiptDigest: result.settlement.receiptDigest,
      rung: result.settlement.verification.rung, body: await result.response.text(),
    };
  } catch (error) {
    const stateError = classifyBoundaryError(error);
    if (stateError === undefined) throw error;
    return { application: "buyer-agent", environment: config.name, state: stateError.state, detail: stateError.message };
  } finally {
    token.destroy();
  }
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const outcome = await runBuyerAgent(await loadApplicationConfig(import.meta.url, "buyer-agent"));
  process.stdout.write(`${JSON.stringify(outcome)}\n`);
  process.exitCode = outcome.state === "paid" ? 0 : outcome.state === "pending" ? 2 : outcome.state === "unknown" ? 3 : 4;
}

function classifyBoundaryError(error) {
  if (error instanceof LayerXApplicationStateError) return error;
  if (error instanceof PlatformSdkError) {
    const refused = ["invalid-argument", "capability-refusal", "policy-refusal", "budget-refusal", "unavailable-capability"].includes(error.code);
    return new LayerXApplicationStateError(refused ? "refused" : "unknown", error.code);
  }
  if (error?.name === "MiddlewareError" && typeof error.code === "string") {
    if (error.code === "payment-pending") return new LayerXApplicationStateError("pending", error.code);
    const refused = [
      "invalid-payment-required",
      "invalid-payment-payload",
      "requirements-mismatch",
      "unsupported-payment",
      "payment-refused",
      "verification-failure",
    ].includes(error.code);
    return new LayerXApplicationStateError(refused ? "refused" : "unknown", error.code);
  }
  if (error instanceof TypeError) return new LayerXApplicationStateError("unknown", "resource_transport_failure");
  return undefined;
}
