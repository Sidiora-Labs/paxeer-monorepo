"use server";

import { ethTransactionReceipt } from "./gateway.ts";

/**
 * Task 22.31.433 / receipt-deployment-wiring: runtime HTTPS origin, CA trust
 * and both network-policy edges are supplied by human/web-deployment.yaml.
 * Proof: e2e/repair-22-31-433.test.ts, receipt_deployment_wiring and
 * receipt_deployment_reaches_market_consumer. Out-of-path wiring requests: none.
 * Baseline verification was not run: the bounded gate order requires the whole
 * implementation before hf-build and hf-done. Inspection found no gateway env
 * or network-policy edges in the original deployment.
 */
export async function gatewayTransactionReceipt(
  ...args: Parameters<typeof ethTransactionReceipt>
): ReturnType<typeof ethTransactionReceipt> {
  return ethTransactionReceipt(...args);
}
