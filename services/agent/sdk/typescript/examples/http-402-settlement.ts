import { Client, type IdempotentMutation, type SubmissionState } from "../src/index.js";

export interface SettlementRequest { readonly paymentRequirement: Uint8Array }

export async function settle402(
  client: Client,
  request: IdempotentMutation<SettlementRequest>,
): Promise<Extract<SubmissionState, { kind: "Receipted" | "Committed" }>> {
  const state = await client.call<typeof request, SubmissionState>("submit", request);
  if (state.kind === "Submitted") throw new Error(`settlement_not_receipted:${state.txHash}`);
  if (state.kind === "Reverted") throw new Error(`settlement_reverted:${state.revertReason}`);
  return state;
}
