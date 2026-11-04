/** Money + number formatting helpers used across LayerX UI. */

export interface FormatMoneyOptions {
  /** ISO-style currency symbol/code shown after the amount, e.g. "USD". */
  currency?: string;
  /** Force a leading + for positive values. Default true when `signed`. */
  signed?: boolean;
  /** Fraction digits. Default 2. */
  decimals?: number;
  /** Symbol prepended to the number. Default "$". Pass "" for none. */
  symbol?: string;
  /** BCP 47 locale used for separators and digit grouping. */
  locale?: string;
}

export function formatMoney(value: number, opts: FormatMoneyOptions = {}): string {
  const { currency, signed = true, decimals = 2, locale = "en-US" } = opts;
  const symbol = opts.symbol ?? (currency === undefined ? "$" : "");
  const abs = Math.abs(value);
  const num = abs.toLocaleString(locale, {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
  const sign = value < 0 ? "- " : signed && value > 0 ? "+ " : "";
  const cur = currency ? ` ${currency}` : "";
  return `${sign}${symbol}${num}${cur}`;
}

/** "$23,043.00" — unsigned, for balances. */
export function formatBalance(value: number, symbol = "$"): string {
  return `${symbol} ${value.toLocaleString("en-US", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}`;
}

/** Compact "30m ago" / "2h ago" style recency labels. */
export function formatRecency(date: Date, now: Date = new Date()): string {
  const mins = Math.round((now.getTime() - date.getTime()) / 60000);
  if (mins < 1) return "now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.round(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.round(hours / 24);
  return `${days}d ago`;
}

/** Group key for month bands, e.g. "February 2025". */
export function monthBandLabel(date: Date): string {
  return date.toLocaleDateString("en-US", { month: "long", year: "numeric" });
}

/**
 * Encode string cells as spreadsheet text when they start with a formula prefix
 * (including after whitespace/control characters), tab, CR or LF. A leading
 * apostrophe preserves the original text; numeric cells keep their numeric form.
 */
export function createCsvBlob(header: string[], rows: (string | number)[][]): Blob {
  const escape = (v: string | number) => {
    const text = String(v);
    const s = typeof v === "string" && /^(?:[\s\u0000-\u001f]*[=+\-@]|[\t\r\n])/u.test(text)
      ? `'${text}`
      : text;
    return /[",\r\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
  };
  const csv = [header, ...rows].map((r) => r.map(escape).join(",")).join("\n");
  return new Blob([csv], { type: "text/csv;charset=utf-8" });
}

/** Build a spreadsheet-safe CSV from primitive cells and trigger a download. */
export function downloadCsv(filename: string, header: string[], rows: (string | number)[][]) {
  const blob = createCsvBlob(header, rows);
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}
