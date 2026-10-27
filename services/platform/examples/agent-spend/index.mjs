import { fileURLToPath } from "node:url";

const required = (name) => {
  const value = process.env[name];
  if (value === undefined || value.length === 0) throw new Error(`missing_${name.toLowerCase()}`);
  return value;
};
const action = process.argv[2] ?? "spend";
if (!["spend", "wallet-review", "wallet-approve", "wallet-status"].includes(action)) throw new Error("invalid_agent_spend_action");
const agentRpcUrl = required("LAYERX_AGENT_RPC_URL");
const tenant = required("LAYERX_TENANT");
const AgentMiddleware = (await import("@sidiora/layerx-agent-middleware")).AgentMiddleware;
const { ProductionClient } = await import("@sidiora/layerx-sdk");
const { LayerXAgentTransport, authenticatedAgentTransport, loadAgentServiceProviders } = await import("@sidiora/layerx-agent-integrations");
const bundledServices = fileURLToPath(new URL("./services.mjs", import.meta.url));
const servicesEnvironment = process.env.LAYERX_AGENT_SERVICES_MODULE === undefined
  ? { ...process.env, LAYERX_AGENT_SERVICES_MODULE: bundledServices } : process.env;
if (action !== "spend" && servicesEnvironment.LAYERX_AGENT_SERVICES_MODULE !== bundledServices) throw new Error("original_wallet_composition_required");
const providers = await loadAgentServiceProviders(servicesEnvironment);
let authenticated;
try {
  authenticated = authenticatedAgentTransport(process.env, {
  agentRpcUrl,
  tenant,
  requestTimeoutMs: 30000,
});
  if (action === "wallet-approve" || action === "wallet-status") {
    const { ownerWalletHandoff } = await import("./services.mjs");
    const owner = ownerWalletHandoff();
    const preparationRef = required("LAYERX_WALLET_PREPARATION_REF");
    const reviewId = required("LAYERX_WALLET_REVIEW_ID");
    const artifact = await (action === "wallet-approve" ? owner.approve(preparationRef, reviewId) : owner.status(preparationRef, reviewId));
    process.stdout.write(JSON.stringify({ action, preparationRef, artifact }) + "\n");
  } else {
  const request = JSON.parse(required("LAYERX_SPEND_REQUEST_JSON"));
  if (request === null || typeof request !== "object" || Array.isArray(request)
    || request.tenant !== required("LAYERX_TENANT")) throw new Error("invalid_spend_request");
  if (action === "wallet-review" && request.walletApprovalId !== undefined) throw new Error("review_cannot_replace_bound_approval");
  if (providers.signer.walletApprovalRequired === true && providers.preparationBudgets === undefined && typeof request.walletApprovalId !== "string") {
    throw new Error("retained_wallet_approval_required");
  }
  const middleware = new AgentMiddleware({
    client: new ProductionClient(new LayerXAgentTransport(authenticated.transport)),
    receiptKeys: JSON.parse(required("LAYERX_RECEIPT_KEYS_JSON")).map((key) => {
      if (!/^[0-9a-f]{64}$/u.test(key.keyId) || !/^[0-9a-f]{64}$/u.test(key.publicKey)
        || typeof key.first_height !== "string" || !/^[0-9]+$/u.test(key.first_height)
        || typeof key.last_height !== "string" || !/^[0-9]+$/u.test(key.last_height)) throw new Error("invalid_receipt_key");
      return { keyId: Buffer.from(key.keyId, "hex"), publicKey: Buffer.from(key.publicKey, "hex"),
        first_height: BigInt(key.first_height), last_height: BigInt(key.last_height) };
    }),
    ...(providers.budgets === undefined ? {} : { budgets: providers.budgets }),
    ...(providers.preparationBudgets === undefined ? {} : { preparationBudgets: providers.preparationBudgets }),
    signer: providers.signer,
    receipts: providers.receipts,
  });
  const result = await middleware.spend(request);
  if (action === "wallet-review") {
    if (result.kind !== "owner-budget" || result.state !== "wallet-consent" || result.prepared === undefined) throw new Error("retained_wallet_consent_preparation_required");
    const { ownerWalletHandoff } = await import("./services.mjs");
    const artifact = await ownerWalletHandoff().review(result.prepared);
    process.stdout.write(JSON.stringify({ action, preparationId: result.preparationId, preparationRef: result.prepared.preparation_ref, artifact }) + "\n");
  } else {
  process.stdout.write(JSON.stringify({
    kind: result.kind,
    ...(result.kind === "owner-budget" ? { ...(result.prepared === undefined ? {} : { prepared: result.prepared }), preparationId: result.preparationId, admissionObserved: result.admissionObserved, ownerState: result.state } : {}),
    ...("verification" in result && result.verification !== undefined ? {
      transactionHash: result.verification.txHash,
      receiptDigest: Buffer.from(result.verification.receiptDigest).toString("hex"),
      rung: result.verification.rung,
    } : {}),
    ...(result.kind === "approval-hold" ? { approvalId: result.approval.approvalId } : {}),
    ...(result.kind === "refused" || result.kind === "budget-refused" ? { code: result.code, retry: result.retry,
      ...(result.retryAfterMs === undefined ? {} : { retryAfterMs: result.retryAfterMs }) } : {}),
    ...("reservation" in result ? { reservationState: result.reservation.state } : {}),
  }) + "\n");
  if (!(result.kind === "owner-budget" ? ["settled", "wallet-consent", "approval", "pending"].includes(result.state) : ["verified", "approval-hold", "pending"].includes(result.kind))) process.exitCode = 2;
  }
  }
} finally {
  authenticated?.destroy();
  await providers.destroy?.();
}
