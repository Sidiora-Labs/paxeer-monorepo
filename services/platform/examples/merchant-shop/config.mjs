import {
  loadApplicationConfig,
  optionalEnvironment,
  requiredEnvironment,
  secureUrl,
} from "../support/runtime.mjs";

export function merchantShopConfiguration(config) {
  if (config.name !== "beta") return config;
  if (optionalEnvironment("LAYERX_EXAMPLE_ENDPOINT") !== undefined) {
    throw new Error("use_layerx_testnet_settlement_url_for_merchant_gateway");
  }
  const rpc = secureUrl(config.rpcUrl);
  const settlement = secureUrl(requiredEnvironment(config.settlementUrlEnvironment));
  if (rpc.pathname !== "/" || config.network !== "eip155:125") {
    throw new Error("invalid_merchant_chain_origin");
  }
  if (settlement.origin === rpc.origin || settlement.pathname !== "/v1/settle") {
    throw new Error("invalid_merchant_settlement_origin");
  }
  return Object.freeze({ ...config, settlementUrl: settlement.toString() });
}

export async function loadMerchantShopConfig(moduleUrl) {
  return merchantShopConfiguration(await loadApplicationConfig(moduleUrl, "merchant-shop"));
}
