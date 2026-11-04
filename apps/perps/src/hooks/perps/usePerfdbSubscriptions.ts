"use client";

// Multiplexed graphql-transport-ws subscriptions for PerfDB.
//
// Previously each hook instance opened its own WebSocket, so 10 components
// subscribing to priceUpdated meant 10 simultaneous connections to the same
// endpoint. Now all subscribers share ONE socket via `subscribeStream` from
// `graphqlClient.ts` — subscribe/next/complete frames are multiplexed over
// that shared connection per the graphql-transport-ws protocol.
//
// Reconnection and backoff are handled centrally inside graphqlClient.ts.

import { useState, useEffect } from "react";
import { subscribeStream, type StreamSubscriptionOptions } from "@/core/graphql/client";
import { PERFDB_GRAPHQL } from "@/core/perfdb/client";

// ── Interfaces ─────────────────────────────────────────────────────────
export interface PriceUpdate {
  marketId: number;
  price: string;
  timestamp: number;
}

export interface TradeUpdate {
  tradeId: string;
  marketId: number;
  isLong: boolean;
  sizeUsd: string;
  price: string;
  blockTimestamp: number;
}

// ── Price Subscription ─────────────────────────────────────────────────
const PRICE_SUBSCRIPTION = /* GraphQL */ `
  subscription PriceUpdated($marketId: Int) {
    priceUpdated(marketId: $marketId) {
      marketId
      price
      timestamp
    }
  }
`;

export type PerfdbConnectionStatus = "unavailable" | "connected" | "disconnected";

type ObservableStreamOptions<T> = StreamSubscriptionOptions<T> & {
  onStatus: (connected: boolean) => void;
};

function usePerfdbStream<T, P>(marketId: number | undefined, query: string, pick: (data: T) => P) {
  const [state, setState] = useState<{
    marketId: number | undefined;
    payload: P | null;
    connectionStatus: PerfdbConnectionStatus;
  }>({ marketId, payload: null, connectionStatus: "unavailable" });
  useEffect(() => {
    let active = true;
    setState({ marketId, payload: null, connectionStatus: "unavailable" });
    const options: ObservableStreamOptions<T> = {
      endpoint: PERFDB_GRAPHQL,
      query,
      variables: marketId !== undefined ? { marketId } : undefined,
      onData: data => {
        if (active) setState(previous => ({ ...previous, payload: pick(data) }));
      },
      onStatus: connected => {
        if (active) setState(previous => ({ ...previous, connectionStatus: connected ? "connected" : "disconnected" }));
      },
      onError: () => {
        if (active) setState(previous => ({ ...previous, connectionStatus: "disconnected" }));
      },
    };
    const unsubscribe = subscribeStream(options);
    return () => { active = false; unsubscribe(); };
  }, [marketId, query, pick]);
  const visible = state.marketId === marketId ? state : { payload: null, connectionStatus: "unavailable" as const };
  return { ...visible, connected: visible.connectionStatus === "connected" };
}

const pickPrice = (data: { priceUpdated: PriceUpdate; }) => data.priceUpdated;
export function usePriceSubscription(marketId?: number) {
  const { payload, connected, connectionStatus } = usePerfdbStream(marketId, PRICE_SUBSCRIPTION, pickPrice);
  return { latestUpdate: payload, connected, connectionStatus };
}

// ── Trade Subscription ─────────────────────────────────────────────────
const TRADE_SUBSCRIPTION = /* GraphQL */ `
  subscription TradeExecuted($marketId: Int) {
    tradeExecuted(marketId: $marketId) {
      tradeId
      marketId
      isLong
      sizeUsd
      price
      blockTimestamp
    }
  }
`;

const pickTrade = (data: { tradeExecuted: TradeUpdate; }) => data.tradeExecuted;
export function useTradeSubscription(marketId?: number) {
  const { payload, connected, connectionStatus } = usePerfdbStream(marketId, TRADE_SUBSCRIPTION, pickTrade);
  return { latestTrade: payload, connected, connectionStatus };
}
