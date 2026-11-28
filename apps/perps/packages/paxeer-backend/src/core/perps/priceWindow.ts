export function priceWindowChange(
  price: number | null | undefined,
  price24h: number | null | undefined,
): number | null {
  if (
    price == null ||
    price24h == null ||
    !Number.isFinite(price) ||
    !Number.isFinite(price24h) ||
    price <= 0 ||
    price24h <= 0
  )
    return null;
  return (price / price24h - 1) * 100;
}

type TickerRow = {
  symbol: string;
  "24h_open"?: number | null;
  "24h_close"?: number | null;
  "24h_high"?: number | null;
  "24h_low"?: number | null;
  "24h_volume"?: number | null;
  "24h_amount"?: number | null;
};

type Ticker = {
  symbol: string;
  open: number | null | undefined;
  close: number | null | undefined;
  high: number | null | undefined;
  low: number | null | undefined;
  volume: number | null | undefined;
  amount: number | null | undefined;
};

export function tickerOfRow(row: TickerRow): Ticker {
  return {
    symbol: row.symbol,
    open: row["24h_open"],
    close: row["24h_close"],
    high: row["24h_high"],
    low: row["24h_low"],
    volume: row["24h_volume"],
    amount: row["24h_amount"],
  };
}

type TickerInfoFields = {
  "24h_open"?: unknown;
  "24h_close"?: unknown;
  "24h_high"?: unknown;
  "24h_low"?: unknown;
  "24h_volumn"?: unknown;
  "24h_volume"?: unknown;
  "24h_amount"?: unknown;
};

export function infoFieldsOfTicker(ticker: unknown): TickerInfoFields {
  const fields: TickerInfoFields = {};
  if (ticker === null || typeof ticker !== "object") return fields;

  if ("open" in ticker && ticker.open !== undefined)
    fields["24h_open"] = ticker.open;
  if ("close" in ticker && ticker.close !== undefined)
    fields["24h_close"] = ticker.close;
  if ("high" in ticker && ticker.high !== undefined)
    fields["24h_high"] = ticker.high;
  if ("low" in ticker && ticker.low !== undefined)
    fields["24h_low"] = ticker.low;
  if ("volume" in ticker && ticker.volume !== undefined) {
    fields["24h_volumn"] = ticker.volume;
    fields["24h_volume"] = ticker.volume;
  }
  if ("amount" in ticker && ticker.amount !== undefined)
    fields["24h_amount"] = ticker.amount;

  return fields;
}
