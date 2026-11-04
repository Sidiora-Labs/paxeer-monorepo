import { keccak256 } from "viem";

// The gateway must retain one review per authenticated principal and signing hash.
// Missing lookup support is an error; an unknown outcome must never create another review.
export function walletReviewTransport(apiUrl, transport = globalThis.fetch) {
  const reviewUrl = `${apiUrl.replace(/\/$/u, "")}/v1/wallet/evm/review`;
  return async (input, init) => {
    if (String(input) !== reviewUrl || init?.method !== "POST") return transport(input, init);
    const { transaction } = JSON.parse(init.body);
    const hash = keccak256(transaction);
    const headers = new Headers(init.headers);
    if (!headers.get("authorization")) throw new Error("original_session_required");
    headers.set("idempotency-key", hash);
    const options = { headers, signal: AbortSignal.timeout(30000), redirect: "error", cache: "no-store" };
    const retained = await transport(`${reviewUrl}s/by-signing-hash/${hash}`, { ...options, method: "GET" });
    if (retained.status !== 404) return retained;
    const missing = await retained.json();
    if (missing?.error !== "evm_review_not_found") throw new Error("wallet_review_lookup_required");
    return transport(input, { ...init, ...options });
  };
}
