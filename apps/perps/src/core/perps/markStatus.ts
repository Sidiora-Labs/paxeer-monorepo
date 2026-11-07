export interface MarkPriceDetails {
  markPrice: bigint;
  indexPrice: bigint;
  midPrice: bigint;
  fresh: boolean;
}

export function markStatus(details: MarkPriceDetails, enabled: boolean): "open" | "stale" | "disabled" {
  if (!enabled) return "disabled";
  return details.fresh ? "open" : "stale";
}
