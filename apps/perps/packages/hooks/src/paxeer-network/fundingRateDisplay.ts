import { Decimal, timeConvertString } from "@paxeer-network/utils";
import { fundingWadDecimal } from "@paxeer-network/paxeer-backend/core/perps/fundingRate";

export interface FundingRateRow {
  est_funding_rate?: number | null;
  last_funding_rate_hourly?: bigint | null;
  next_funding_time?: number | null;
}

/** Funding percentage and time remaining for a market row at a clock value. */
export function fundingRateDisplay(
  row: FundingRateRow | null | undefined,
  now: number,
): { est_funding_rate: string | null; countDown: string } {
  const next = row?.next_funding_time;
  const remaining = next == null ? 0 : next - now;
  const countDown =
    Number.isFinite(remaining) && remaining > 0
      ? timeConvertString(remaining)
          .map((part) => part.toString().padStart(2, "0"))
          .join(":")
      : "00:00:00";

  const predicted = row?.est_funding_rate;
  if (predicted != null && Number.isFinite(predicted)) {
    return {
      est_funding_rate:
        next != null && now > next
          ? null
          : new Decimal(Number(predicted) * 100).toFixed(4, Decimal.ROUND_DOWN),
      countDown,
    };
  }

  const hourly = fundingWadDecimal(row?.last_funding_rate_hourly ?? null);
  return {
    est_funding_rate:
      hourly === null
        ? null
        : new Decimal(hourly).mul(100).toFixed(4, Decimal.ROUND_DOWN),
    countDown,
  };
}
