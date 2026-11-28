export function rewardsLedgerEndpoint(_env: string): string {
  return "";
}

export function claimedReadTarget(
  endpoint: unknown,
  ledger: unknown,
): { endpoint: string; ledger: string } | null {
  if (typeof endpoint !== "string" || typeof ledger !== "string") {
    return null;
  }

  const trimmedEndpoint = endpoint.trim();
  const trimmedLedger = ledger.trim();
  if (trimmedEndpoint === "" || trimmedLedger === "") {
    return null;
  }

  return { endpoint: trimmedEndpoint, ledger: trimmedLedger };
}
