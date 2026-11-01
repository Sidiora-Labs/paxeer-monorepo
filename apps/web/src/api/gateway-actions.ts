"use server";

import { ethTransactionReceipt } from "./gateway.ts";

export async function gatewayTransactionReceipt(
  ...args: Parameters<typeof ethTransactionReceipt>
): ReturnType<typeof ethTransactionReceipt> {
  return ethTransactionReceipt(...args);
}
