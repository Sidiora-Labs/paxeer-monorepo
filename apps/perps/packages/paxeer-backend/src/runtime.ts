import type { GatewayCatalogEntry } from "./hooks/perps/usePerpsMarkets";
import type { FeeTierView } from "./core/perps/feeTier";

export interface NativeSnapshot {
  address: `0x${string}` | null;
  chainId: number | null;
  marketCatalog: readonly GatewayCatalogEntry[];
  markets: any[];
  positions: any[];
  totalVaultUsd: number | null;
  freeCollateral: number | null;
  unrealizedPnl: number | null;
  maintenanceMarginUsd: number | null;
  walletBalances: Record<string, string>;
  dailyPnl?: any[];
  leverage: Record<string, number>;
  connected: boolean;
  marginMode?: "CROSS" | "ISOLATED";
  feeTier?: FeeTierView;
}
let snapshot: NativeSnapshot = {
  address: null,
  chainId: null,
  marketCatalog: [],
  markets: [],
  positions: [],
  totalVaultUsd: null,
  freeCollateral: null,
  unrealizedPnl: null,
  maintenanceMarginUsd: null,
  walletBalances: {},
  leverage: {},
  connected: false,
};
const listeners = new Set<() => void>();
export const getNativeSnapshot = () => snapshot;
export function updateNativeSnapshot(update: Partial<NativeSnapshot>) {
  if (
    "address" in update &&
    update.address?.toLowerCase() !== snapshot.address?.toLowerCase()
  )
    snapshot = {
      ...snapshot,
      walletBalances: {},
      positions: [],
      totalVaultUsd: null,
      freeCollateral: null,
      unrealizedPnl: null,
      maintenanceMarginUsd: null,
      dailyPnl: undefined,
      leverage: {},
      marginMode: undefined,
      feeTier: undefined,
    };
  snapshot = { ...snapshot, ...update };
  listeners.forEach((listener) => listener());
}
export function subscribeNativeSnapshot(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
