import "./production-sources.mjs";
import { SecretBytes } from "@sidiora/layerx-sdk";
const { LayerXServiceEndpoint, LayerXBudgetLedger, LayerXRemoteSigner, LayerXReceiptResolver } =
  await import("../../../../../sdk/integrations/agents/src/services.ts");

export function createAgentServices(environment) {
  const token = new SecretBytes(new TextEncoder().encode("proof-local-services"));
  const endpoint = (path) => new LayerXServiceEndpoint({
    url: `${environment.LAYERX_AGENT_RPC_URL}/${path}`, token, timeoutMs: 1000,
  });
  return {
    token,
    budgets: new LayerXBudgetLedger(endpoint("budget")),
    signer: new LayerXRemoteSigner(endpoint("sign")),
    receipts: new LayerXReceiptResolver(endpoint("receipt")),
    destroy() { this.token.destroy(); },
  };
}
