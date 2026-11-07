import { createMerchantApplicationServer } from "../support/merchant-app.mjs";
import { loadMerchantShopConfig } from "./config.mjs";

const config = await loadMerchantShopConfig(import.meta.url);
const port = Number(config.port);
if (!Number.isSafeInteger(port) || port <= 0 || port > 65535) throw new Error("invalid_port");
createMerchantApplicationServer(config).listen(port, "127.0.0.1", () => {
  process.stdout.write(`${JSON.stringify({ application: "merchant-shop", environment: config.name, listening: port, checkout: "/checkout", webhook: "/webhooks/settlement" })}\n`);
});
