import {
  Client,
  Rung,
  requireVerified,
  type IdempotentMutation,
  type SubmissionState,
  type VerifiedRead,
} from "../src/index.js";

export interface PaymentRequest { readonly canonicalBytes: Uint8Array }

export async function paymentWithVerification(
  client: Client,
  request: IdempotentMutation<PaymentRequest>,
): Promise<VerifiedRead<SubmissionState>> {
  const result = await client.call<typeof request, VerifiedRead<SubmissionState>>("submit", request);
  const verified = requireVerified(Rung.Receipted, result);
  if (verified.value.kind === "Reverted") throw new Error(`payment_reverted:${verified.value.revertReason}`);
  if (verified.value.kind !== "Receipted" && verified.value.kind !== "Committed") {
    throw new Error(`payment_not_receipted:${verified.value.kind}`);
  }
  return verified;
}
