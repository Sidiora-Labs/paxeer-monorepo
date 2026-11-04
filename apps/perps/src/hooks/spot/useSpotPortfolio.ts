"use client";
import { SPOT_USER_WS_URL } from "./useSpotUserStream";
import { activePoll, onStreamResync } from "@/core/net/realtime";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  spotApi,
  type SpotHoldingsResponse,
  type SpotOrder,
  type SpotPosition,
  type SpotPnlSummary,
  type SpotTrade,
  type SpotTradesResponse,
} from "@/core/spot/api";
import { subscribeSpotUserStream } from "./useSpotUserStream";
import {
  mergeSpotTradeSnapshot,
  spotTradeSnapshotForAddress,
} from "@/core/spot/trade-history";

/* ══════════════════════════════════════════════════════════════════════
   `useSpotPortfolio(address)`

   One composite hook for every panel that needs the user's full SPOT
   trading context. Layers REST snapshots on top of the user-scoped
   WebSocket push channel so the UI is fresh without aggressive polling:

     · Initial load (parallel) — holdings + positions + open-orders +
       recent trades + closed-legs (first page) + pnl summary.
     · WS push (`order` stream)      → upsert into open-orders cache.
     · WS push (`trade` stream)      → prepend to trades cache + flag
                                       holdings + positions for refetch.
     · WS push (`position` stream)   → upsert into positions cache.
     · WS push (`pnl` stream)        → replace pnl summary cache.
     · WS push (`closed-leg` stream) → prepend to closed-legs cache.

   Holdings is the only field with **no** push channel exposed by the API,
   so we still poll holdings every 8 s while the hook is mounted.

   Cancellation: every fetch uses an AbortController scoped to the
   current address. Address change cancels in-flight requests.
   ══════════════════════════════════════════════════════════════════════ */

const HOLDINGS_POLL_MS = 8_000;
const TRADES_INITIAL_LIMIT = 50;
const ORDERS_INITIAL_LIMIT = 100;

export interface SpotPortfolioState {
  isConnected: boolean;
  isLoading: boolean;
  /** True once the initial REST snapshot has resolved (success or fail). */
  hasLoadedOnce: boolean;
  /** First load error, if the snapshot fetch failed entirely. */
  error: Error | null;
  // Snapshots
  holdings: SpotHoldingsResponse | null;
  positions: SpotPosition[];
  openOrders: SpotOrder[];
  recentTrades: SpotTrade[];
  tradeHistory: SpotTradesResponse | null;
  tradeHistoryError: Error | null;
  pnlSummary: SpotPnlSummary | null;
  /** Last frame received per stream, for components that want raw events. */
  lastFrames: {
    trade: unknown;
    order: unknown;
    position: unknown;
    pnl: unknown;
    closedLeg: unknown;
  };
  /** Manual refresh (re-pulls everything except cursor-paged tables). */
  refresh: () => void;
}

const EMPTY_STATE: SpotPortfolioState = {
  isConnected: false,
  isLoading: false,
  hasLoadedOnce: false,
  error: null,
  holdings: null,
  positions: [],
  openOrders: [],
  recentTrades: [],
  tradeHistory: null,
  tradeHistoryError: null,
  pnlSummary: null,
  lastFrames: {
    trade: null,
    order: null,
    position: null,
    pnl: null,
    closedLeg: null,
  },
  refresh: () => {},
};

/**
 * Extract the data payload from a user-scoped WS frame. The server wraps
 * most events under a `data` key but some legacy frames put the payload
 * at top level. We strip the routing fields and return whatever's left.
 */
function extractPayload(frame: Record<string, unknown>): unknown {
  if (frame == null) return null;
  if ("data" in frame && frame.data && typeof frame.data === "object") {
    return frame.data;
  }
  // Strip routing keys.
  const {
    stream: _s,
    address: _a,
    op: _op,
    type: _t,
    ...rest
  } = frame as {
    stream?: unknown;
    address?: unknown;
    op?: unknown;
    type?: unknown;
  } & Record<string, unknown>;
  return rest;
}

/**
 * Try to extract an order id from an `order` push payload. The schema
 * field name is `orderId` per swagger; we also accept `id` as a fallback
 * because the WS push protocol isn't formally documented.
 */
function orderIdOf(o: Partial<SpotOrder>): string | null {
  if (typeof o?.orderId === "string" && o.orderId.length > 0) return o.orderId;
  if (typeof o?.id === "string" || typeof o?.id === "number")
    return String(o.id);
  return null;
}

function tokenIdOf(p: Partial<SpotPosition>): string | null {
  if (typeof p?.token === "string" && p.token.length > 0)
    return p.token.toLowerCase();
  return null;
}

interface PortfolioRead {
  watermark: number;
  controller: AbortController;
  trades: SpotTrade[];
  orders: Map<string, SpotOrder>;
  positions: Map<string, SpotPosition>;
  pnl: SpotPnlSummary | null;
}

function mergeOrders(
  snapshot: SpotOrder[],
  updates: Map<string, SpotOrder>,
): SpotOrder[] {
  let orders = snapshot;
  for (const [id, order] of updates) {
    orders = orders.filter((item) => orderIdOf(item) !== id);
    if (order.status === "open" || order.status === "activated")
      orders = [order, ...orders];
  }
  return orders;
}

function mergePositions(
  snapshot: SpotPosition[],
  updates: Map<string, SpotPosition>,
): SpotPosition[] {
  let positions = snapshot;
  for (const [id, position] of updates) {
    positions = [
      position,
      ...positions.filter((item) => tokenIdOf(item) !== id),
    ];
  }
  return positions;
}

export function useSpotPortfolio(
  address: string | null | undefined,
): SpotPortfolioState {
  const account = address?.toLowerCase() || null;
  const selectedAccount = useRef(account);
  selectedAccount.current = account;
  const generation = useRef(0);
  const [portfolio, setPortfolio] = useState<{
    address: string | null;
    data: SpotPortfolioState;
  }>({ address: null, data: EMPTY_STATE });
  const [refreshTick, setRefreshTick] = useState(0);
  const refresh = useCallback(() => setRefreshTick((tick) => tick + 1), []);

  useEffect(
    () =>
      onStreamResync((url) => {
        if (account && url === SPOT_USER_WS_URL) refresh();
      }),
    [account, refresh],
  );

  useEffect(() => {
    const currentGeneration = ++generation.current;
    let active = true;
    const isCurrent = () =>
      active &&
      generation.current === currentGeneration &&
      selectedAccount.current === account;
    const publish = (
      update: (previous: SpotPortfolioState) => SpotPortfolioState,
    ) => {
      if (!isCurrent()) return;
      setPortfolio((previous) =>
        isCurrent() && previous.address === account
          ? { address: account, data: update(previous.data) }
          : previous,
      );
    };
    setPortfolio((previous) => ({
      address: account,
      data:
        account && previous.address === account ? previous.data : EMPTY_STATE,
    }));
    if (!account)
      return () => {
        active = false;
      };

    let watermark = 0;
    const pending = new Set<PortfolioRead>();
    const controllers = new Set<AbortController>();
    const applied = new Map<string, number>();
    const beginRead = (): PortfolioRead => {
      const read: PortfolioRead = {
        watermark: ++watermark,
        controller: new AbortController(),
        trades: [],
        orders: new Map(),
        positions: new Map(),
        pnl: null,
      };
      controllers.add(read.controller);
      pending.add(read);
      return read;
    };
    const finishRead = (read: PortfolioRead) => {
      pending.delete(read);
      controllers.delete(read.controller);
    };
    const commit = (
      field: string,
      read: PortfolioRead,
      update: (previous: SpotPortfolioState) => SpotPortfolioState,
    ) => {
      if (
        !isCurrent() ||
        read.controller.signal.aborted ||
        (applied.get(field) ?? 0) > read.watermark
      )
        return;
      applied.set(field, read.watermark);
      publish(update);
    };
    const recordFrame = (
      key: keyof SpotPortfolioState["lastFrames"],
      frame: unknown,
    ) =>
      publish((previous) => ({
        ...previous,
        lastFrames: { ...previous.lastFrames, [key]: frame },
      }));

    // Each request marks its start and retains the latest push per entity.
    // Replaying these updates also preserves terminal-order tombstones.
    const snapshot = beginRead();
    publish((previous) => ({
      ...previous,
      isLoading: true,
      error: null,
      tradeHistoryError: null,
    }));
    void Promise.allSettled([
      spotApi.getHoldings(account, snapshot.controller.signal),
      spotApi.getPositions(account, snapshot.controller.signal),
      spotApi.getOrders(account, {
        status: "open",
        limit: ORDERS_INITIAL_LIMIT,
        signal: snapshot.controller.signal,
      }),
      spotApi.getTrades(account, {
        limit: TRADES_INITIAL_LIMIT,
        signal: snapshot.controller.signal,
      }),
      spotApi.getPnlSummary(account, snapshot.controller.signal),
    ])
      .then(([holdings, positions, orders, trades, pnl]) => {
        if (!isCurrent() || snapshot.controller.signal.aborted) return;
        if (holdings.status === "fulfilled")
          commit("holdings", snapshot, (previous) => ({
            ...previous,
            holdings: holdings.value,
          }));
        if (positions.status === "fulfilled")
          commit("positions", snapshot, (previous) => ({
            ...previous,
            positions: mergePositions(
              positions.value.positions,
              snapshot.positions,
            ),
          }));
        if (orders.status === "fulfilled")
          commit("orders", snapshot, (previous) => ({
            ...previous,
            openOrders: mergeOrders(orders.value.items, snapshot.orders),
          }));
        if (trades.status === "fulfilled")
          publish((previous) => ({
            ...previous,
            tradeHistory: mergeSpotTradeSnapshot(trades.value, snapshot.trades),
          }));
        else
          publish((previous) => ({
            ...previous,
            tradeHistoryError:
              trades.reason instanceof Error
                ? trades.reason
                : new Error(String(trades.reason)),
          }));
        if (pnl.status === "fulfilled")
          commit("pnl", snapshot, (previous) => ({
            ...previous,
            pnlSummary: snapshot.pnl ?? pnl.value,
          }));
        const results = [holdings, positions, orders, trades, pnl];
        const firstFailure = results.find(
          (result) => result.status === "rejected",
        );
        publish((previous) => ({
          ...previous,
          isLoading: false,
          hasLoadedOnce: true,
          error:
            results.every((result) => result.status === "rejected") &&
            firstFailure?.status === "rejected"
              ? firstFailure.reason instanceof Error
                ? firstFailure.reason
                : new Error(String(firstFailure.reason))
              : null,
        }));
      })
      .finally(() => finishRead(snapshot));

    const refreshDerived = () => {
      const read = beginRead();
      void Promise.allSettled([
        spotApi
          .getHoldings(account, read.controller.signal)
          .then((holdings) =>
            commit("holdings", read, (previous) => ({ ...previous, holdings })),
          ),
        spotApi
          .getPositions(account, read.controller.signal)
          .then((positions) =>
            commit("positions", read, (previous) => ({
              ...previous,
              positions: mergePositions(positions.positions, read.positions),
            })),
          ),
        spotApi
          .getPnlSummary(account, read.controller.signal)
          .then((pnl) =>
            commit("pnl", read, (previous) => ({
              ...previous,
              pnlSummary: read.pnl ?? pnl,
            })),
          ),
      ]).finally(() => finishRead(read));
    };

    const stopPoll = activePoll(
      async () => {
        const read = beginRead();
        try {
          const holdings = await spotApi.getHoldings(
            account,
            read.controller.signal,
          );
          commit("holdings", read, (previous) => ({ ...previous, holdings }));
        } catch {
          /* Retain this account's last known holdings. */
        } finally {
          finishRead(read);
        }
      },
      HOLDINGS_POLL_MS,
      false,
    );

    const offs = [
      subscribeSpotUserStream("trade", account, (frame) => {
        if (!isCurrent()) return;
        recordFrame("trade", frame);
        const payload = extractPayload(frame);
        if (payload && typeof payload === "object") {
          const trade = payload as SpotTrade;
          if (
            (typeof trade.id === "string" || typeof trade.id === "number") &&
            ((typeof trade.blockNumber === "string" &&
              /^\d+$/.test(trade.blockNumber)) ||
              (typeof trade.blockNumber === "number" &&
                Number.isSafeInteger(trade.blockNumber) &&
                trade.blockNumber >= 0)) &&
            Number.isSafeInteger(trade.logIndex) &&
            trade.logIndex >= 0
          ) {
            for (const read of pending) read.trades.unshift(trade);
            publish((previous) => {
              const history = spotTradeSnapshotForAddress(
                previous.tradeHistory,
                account,
              );
              return {
                ...previous,
                tradeHistory: history
                  ? mergeSpotTradeSnapshot(history, [trade])
                  : null,
              };
            });
          } else refresh();
        }
        refreshDerived();
      }),
      subscribeSpotUserStream("order", account, (frame) => {
        if (!isCurrent()) return;
        recordFrame("order", frame);
        const payload = extractPayload(frame);
        if (!payload || typeof payload !== "object") return;
        const order = payload as SpotOrder;
        const id = orderIdOf(order);
        if (!id) return;
        for (const read of pending) read.orders.set(id, order);
        publish((previous) => ({
          ...previous,
          openOrders: mergeOrders(previous.openOrders, new Map([[id, order]])),
        }));
      }),
      subscribeSpotUserStream("position", account, (frame) => {
        if (!isCurrent()) return;
        recordFrame("position", frame);
        const payload = extractPayload(frame);
        if (!payload || typeof payload !== "object") return;
        const position = payload as SpotPosition;
        const id = tokenIdOf(position);
        if (!id) return;
        for (const read of pending) read.positions.set(id, position);
        publish((previous) => ({
          ...previous,
          positions: mergePositions(
            previous.positions,
            new Map([[id, position]]),
          ),
        }));
      }),
      subscribeSpotUserStream("pnl", account, (frame) => {
        if (!isCurrent()) return;
        recordFrame("pnl", frame);
        const payload = extractPayload(frame);
        if (payload && typeof payload === "object") {
          const pnl = payload as SpotPnlSummary;
          for (const read of pending) read.pnl = pnl;
          publish((previous) => ({ ...previous, pnlSummary: pnl }));
          return;
        }
        const read = beginRead();
        void spotApi
          .getPnlSummary(account, read.controller.signal)
          .then((pnl) =>
            commit("pnl", read, (previous) => ({
              ...previous,
              pnlSummary: read.pnl ?? pnl,
            })),
          )
          .catch(() => {})
          .finally(() => finishRead(read));
      }),
      subscribeSpotUserStream("closed-leg", account, (frame) => {
        if (isCurrent()) recordFrame("closedLeg", frame);
      }),
    ];
    return () => {
      active = false;
      for (const controller of controllers) controller.abort();
      controllers.clear();
      pending.clear();
      stopPoll();
      offs.forEach((off) => off());
    };
  }, [account, refreshTick, refresh]);

  if (!account) return { ...EMPTY_STATE, refresh };
  const current = portfolio.address === account ? portfolio.data : EMPTY_STATE;
  const tradeHistory = spotTradeSnapshotForAddress(
    current.tradeHistory,
    account,
  );
  return {
    ...current,
    isConnected: true,
    refresh,
    tradeHistory,
    recentTrades: tradeHistory?.items ?? EMPTY_STATE.recentTrades,
  };
}
