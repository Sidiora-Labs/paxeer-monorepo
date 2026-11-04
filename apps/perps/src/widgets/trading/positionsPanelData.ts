import {
  subgraphQuery,
  ALL_ORDERS_V2_QUERY,
  TPSL_ORDERS_QUERY,
  CLOSED_POSITIONS_QUERY,
  TRADES_QUERY,
  LIQUIDATIONS_QUERY,
} from "@/hooks/perps/useSubgraph";
import type { PositionRowInput } from "@/core/perps/pnlHelpers";

export interface PanelOrder {
  orderId: string;
  positionId: string | null;
  marketId: number;
  isLong: boolean;
  orderType: number;
  triggerPrice: string;
  sizeUsd: string;
  status: string;
  failureReason: string | null;
  resolvedTxHash: string | null;
}

interface ClosedPosition extends PositionRowInput {
  positionId: string;
  realizedPnl: string;
  exitPrice: string;
  closedAt: string;
}

interface PanelTrade {
  id: string;
  marketId: number;
  tradeType: string;
  isLong: boolean;
  sizeUsd: string;
  price: string;
  realizedPnl: string;
  feeUsd: string;
  txHash: string;
  blockTimestamp: string;
}

interface PanelLiquidation {
  id: string;
  marketId: number;
  price: string;
  penalty: string;
  txHash: string;
  blockTimestamp: string;
}

export type PanelDataKey = "orders" | "tpsl" | "positions" | "trades" | "liquidations";
export interface PositionsPanelSnapshot {
  allOrders: PanelOrder[];
  closedPositions: ClosedPosition[];
  trades: PanelTrade[];
  liquidations: PanelLiquidation[];
  tpslMap: Record<string, { tp?: string; sl?: string }>;
  errors: Partial<Record<PanelDataKey, string>>;
  loading: boolean;
}

function emptySnapshot(): PositionsPanelSnapshot {
  return { allOrders: [], closedPositions: [], trades: [], liquidations: [], tpslMap: {}, errors: {}, loading: false };
}

export class PositionsPanelData {
  private snapshot = emptySnapshot();
  private listeners = new Set<() => void>();
  private active = false;
  private liveGeneration = 0;
  private historyGeneration = 0;

  constructor(readonly address: string | undefined) {}

  getSnapshot = () => this.snapshot;
  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => { this.listeners.delete(listener); };
  };

  private publish(change: Partial<PositionsPanelSnapshot>) {
    this.snapshot = { ...this.snapshot, ...change };
    this.listeners.forEach((listener) => listener());
  }

  open() {
    this.active = true;
  }

  close() {
    this.active = false;
    ++this.liveGeneration;
    ++this.historyGeneration;
    this.publish(emptySnapshot());
  }

  private async receive<T>(
    key: PanelDataKey,
    request: Promise<T>,
    current: () => boolean,
    apply: (data: T) => Partial<PositionsPanelSnapshot>,
  ) {
    try {
      const data = await request;
      if (!current()) return;
      const errors = { ...this.snapshot.errors };
      delete errors[key];
      this.publish({ ...apply(data), errors });
    } catch (error) {
      if (!current()) return;
      this.publish({ errors: { ...this.snapshot.errors, [key]: error instanceof Error ? error.message : "Request failed" } });
    }
  }

  refreshLive = async () => {
    if (!this.active || !this.address) return;
    const generation = ++this.liveGeneration;
    const current = () => this.active && generation === this.liveGeneration;
    this.publish({ loading: true });
    await Promise.all([
      this.receive("orders", subgraphQuery<{ orders: PanelOrder[] }>(ALL_ORDERS_V2_QUERY, { userAddress: this.address, limit: 100 }), current,
        (data) => ({ allOrders: data.orders ?? [] })),
      this.receive("tpsl", subgraphQuery<{ takeProfits: Pick<PanelOrder, "positionId" | "triggerPrice">[]; stopLosses: Pick<PanelOrder, "positionId" | "triggerPrice">[] }>(TPSL_ORDERS_QUERY, { userAddress: this.address }), current,
        (data) => {
          const tpslMap: PositionsPanelSnapshot["tpslMap"] = {};
          for (const tp of data.takeProfits ?? []) {
            if (tp.positionId) (tpslMap[tp.positionId] ??= {}).tp = tp.triggerPrice;
          }
          for (const sl of data.stopLosses ?? []) {
            if (sl.positionId) (tpslMap[sl.positionId] ??= {}).sl = sl.triggerPrice;
          }
          return { tpslMap };
        }),
    ]);
    if (current()) this.publish({ loading: false });
  };

  refreshHistory = async () => {
    if (!this.active || !this.address) return;
    const generation = ++this.historyGeneration;
    const current = () => this.active && generation === this.historyGeneration;
    const variables = { userAddress: this.address, limit: 50 };
    await Promise.all([
      this.receive("positions", subgraphQuery<{ positions: ClosedPosition[] }>(CLOSED_POSITIONS_QUERY, variables), current,
        (data) => ({ closedPositions: data.positions ?? [] })),
      this.receive("trades", subgraphQuery<{ trades: PanelTrade[] }>(TRADES_QUERY, variables), current,
        (data) => ({ trades: data.trades ?? [] })),
      this.receive("liquidations", subgraphQuery<{ liquidations: PanelLiquidation[] }>(LIQUIDATIONS_QUERY, variables), current,
        (data) => ({ liquidations: data.liquidations ?? [] })),
    ]);
  };
}
