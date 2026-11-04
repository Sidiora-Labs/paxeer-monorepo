import { formatUnits } from "viem";
import { create } from "zustand";
import { useOrderLineStore } from "@/hooks/perps/useOrderLineStore";

export type ChartPriceField = "entryPrice" | "tpPrice" | "slPrice";

export interface ChartPriceChange {
  symbol: string;
  field: ChartPriceField;
  price: string;
}

export interface DraggablePriceLine {
  getPrice(): number;
  setPrice(price: number): unknown;
  setText(text: string): unknown;
}

export const useChartPriceChange = create<{ change: ChartPriceChange | null }>()(
  () => ({ change: null }),
);

export function snapChartPrice(price: number, tickSize: bigint): string {
  if (!Number.isFinite(price) || price <= 0 || tickSize <= 0n)
    throw new RangeError("A chart price and market tick size must be positive.");

  const [coefficient, exponent = "0"] = price.toString().toLowerCase().split("e");
  const [whole, fraction = ""] = coefficient.split(".");
  const digits = BigInt(whole + fraction);
  const scale = fraction.length - Number(exponent);
  const numerator = digits * 10n ** BigInt(Math.max(18 - scale, 0));
  const denominator = tickSize * 10n ** BigInt(Math.max(scale - 18, 0));
  const ticks = (2n * numerator + denominator) / (2n * denominator);
  if (ticks === 0n)
    throw new RangeError("The chart price rounds below the market's first tick.");
  return formatUnits(ticks * tickSize, 18);
}

export function moveChartOrderLine(
  line: DraggablePriceLine,
  symbol: string,
  field: ChartPriceField,
  tickSize: bigint,
): void {
  const price = snapChartPrice(line.getPrice(), tickSize);
  const coordinate = Number(price);
  const labels = { entryPrice: "Entry", tpPrice: "TP", slPrice: "SL" };
  line.setPrice(coordinate);
  line.setText(`${labels[field]} @ $${price}`);
  const store = useOrderLineStore.getState();
  const handlers = {
    entryPrice: store.onChartDragEntry,
    tpPrice: store.onChartDragTp,
    slPrice: store.onChartDragSl,
  };
  handlers[field](coordinate);
  useChartPriceChange.setState({ change: { symbol, field, price } });
}
