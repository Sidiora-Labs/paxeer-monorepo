"use client";

// Multiplexed graphql-transport-ws subscriptions for the Sidiora indexer.
//
// The indexer exposes subscriptions at /graphql-ws (graphql-transport-ws
// subprotocol). All hooks here share ONE socket per endpoint via
// `subscribeStream` from `graphqlClient.ts`.
//
// Migration guide:
//   indexerStatusUpdated  → useIndexerStatus()
//   latestPricesUpdated   → useLatestPricesSubscription()
//   priceUpdated          → useSingleMarketPriceSubscription(marketId)
//   positionChanged       → usePositionChangedSubscription(userAddress)
//   orderChanged          → useOrderChangedSubscription(userAddress)
//   tradeCreated          → useTradeCreatedSubscription(marketId)
//   liquidationCreated    → useLiquidationSubscription(marketId)

import { useState, useEffect, useMemo, useRef } from "react";
import { subscribeStream } from "@/core/graphql/client";
import { INDEXER_GRAPHQL } from "@/core/perps/config";

// ── Shared types ────────────────────────────────────────────────────────

export interface IndexerStatus {
  lastIndexedBlock: number;
  chainHead: number;
  isSynced: boolean;
  blocksScanned: number;
  eventsProcessed: number;
}

export interface IndexerPriceUpdate {
  marketId: number;
  price: string;
  updatedAt: string;
  blockNumber: number;
}

export interface IndexerPosition {
  positionId: string;
  marketId: number;
  isLong: boolean;
  sizeUsd: string;
  leverage: string;
  entryPrice: string;
  status: string;
  realizedPnl: string;
}

export interface IndexerOrder {
  orderId: string;
  status: string;
  marketId: number;
  orderType: number;
  remainingSize: string | null;
  blockTimestamp: string | null;
}

export interface IndexerTrade {
  positionId: string;
  userAddress: string | null;
  sizeUsd: string;
  executionPrice: string | null;
  tradeType: string;
  blockTimestamp: string;
}

export interface IndexerLiquidation {
  positionId: string;
  userAddress: string;
  sizeUsd: string | null;
  executionPrice: string | null;
  blockTimestamp: string;
}

function eventFields(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== "object" || Array.isArray(value)) {
    throw new TypeError("Indexer event must be an object");
  }
  return value as Record<string, unknown>;
}

function eventString(value: unknown, field: string): string {
  if (typeof value !== "string") {
    throw new TypeError(`Indexer ${field} must be a string`);
  }
  return value;
}

function eventDecimal(value: unknown, field: string): string {
  const text = eventString(value, field);
  if (!/^\d+$/.test(text)) {
    throw new TypeError(`Indexer ${field} must be an unsigned decimal integer`);
  }
  return text;
}

function eventInt(value: unknown, field: string): number {
  if (
    typeof value !== "number" ||
    !Number.isInteger(value) ||
    value < 0 ||
    value > 2147483647
  ) {
    throw new TypeError(`Indexer ${field} must be a nonnegative GraphQL Int`);
  }
  return value;
}

function eventTime(value: unknown): string {
  const text = eventString(value, "blockTimestamp");
  if (
    !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/.test(
      text,
    ) ||
    !Number.isFinite(Date.parse(text))
  ) {
    throw new TypeError("Indexer blockTimestamp must be an RFC3339 DateTime");
  }
  return text;
}

export function decodeIndexerOrder(value: unknown): IndexerOrder {
  const row = eventFields(value);
  return {
    orderId: eventDecimal(row.orderId, "orderId"),
    status: eventString(row.status, "status"),
    marketId: eventInt(row.marketId, "marketId"),
    orderType: eventInt(row.orderType, "orderType"),
    remainingSize:
      row.remainingSize === null
        ? null
        : eventDecimal(row.remainingSize, "remainingSize"),
    blockTimestamp:
      row.blockTimestamp === null ? null : eventTime(row.blockTimestamp),
  };
}

export function decodeIndexerTrade(value: unknown): IndexerTrade {
  const row = eventFields(value);
  return {
    positionId: eventDecimal(row.positionId, "positionId"),
    userAddress:
      row.userAddress === null
        ? null
        : eventString(row.userAddress, "userAddress"),
    sizeUsd: eventDecimal(row.sizeUsd, "sizeUsd"),
    executionPrice:
      row.executionPrice === null
        ? null
        : eventDecimal(row.executionPrice, "executionPrice"),
    tradeType: eventString(row.tradeType, "tradeType"),
    blockTimestamp: eventTime(row.blockTimestamp),
  };
}

export function decodeIndexerLiquidation(value: unknown): IndexerLiquidation {
  const row = eventFields(value);
  return {
    positionId: eventDecimal(row.positionId, "positionId"),
    userAddress: eventString(row.userAddress, "userAddress"),
    sizeUsd: row.sizeUsd === null ? null : eventDecimal(row.sizeUsd, "sizeUsd"),
    executionPrice:
      row.executionPrice === null
        ? null
        : eventDecimal(row.executionPrice, "executionPrice"),
    blockTimestamp: eventTime(row.blockTimestamp),
  };
}

/** Retained stream values are only visible to the generation that received them. */
function useIndexerSelection<T>(
  selection: string | number | undefined,
  subscribe: (onData: (data: T) => void) => () => void,
): T | null {
  const generation = useMemo(() => ({}), [selection]);
  const subscribeRef = useRef(subscribe);
  subscribeRef.current = subscribe;
  const [state, setState] = useState<{ generation: object; value: T | null }>({
    generation,
    value: null,
  });
  useEffect(() => {
    let active = true;
    setState({ generation, value: null });
    const unsubscribe =
      selection === undefined
        ? undefined
        : subscribeRef.current((value) => {
            if (active) setState({ generation, value });
          });
    return () => {
      active = false;
      unsubscribe?.();
    };
  }, [selection, generation]);
  return selection !== undefined && state.generation === generation
    ? state.value
    : null;
}

// ── indexerStatusUpdated ───────────────────────────────────────────────

const INDEXER_STATUS_SUBSCRIPTION = /* GraphQL */ `
  subscription {
    indexerStatusUpdated {
      lastIndexedBlock
      chainHead
      isSynced
      blocksScanned
      eventsProcessed
    }
  }
`;

export function useIndexerStatus() {
  const [status, setStatus] = useState<IndexerStatus | null>(null);

  useEffect(() => {
    return subscribeStream<{ indexerStatusUpdated: IndexerStatus }>({
      endpoint: INDEXER_GRAPHQL,
      query: INDEXER_STATUS_SUBSCRIPTION,
      onData: (data) => setStatus(data.indexerStatusUpdated),
    });
  }, []);

  return status;
}

// ── latestPricesUpdated ────────────────────────────────────────────────

const LATEST_PRICES_SUBSCRIPTION = /* GraphQL */ `
  subscription {
    latestPricesUpdated {
      marketId
      price
      updatedAt
      blockNumber
    }
  }
`;

export function useLatestPricesSubscription() {
  const [prices, setPrices] = useState<IndexerPriceUpdate[]>([]);

  useEffect(() => {
    return subscribeStream<{ latestPricesUpdated: IndexerPriceUpdate[] }>({
      endpoint: INDEXER_GRAPHQL,
      query: LATEST_PRICES_SUBSCRIPTION,
      onData: (data) => setPrices(data.latestPricesUpdated),
    });
  }, []);

  return prices;
}

// ── priceUpdated(marketId) ─────────────────────────────────────────────

const PRICE_UPDATED_SUBSCRIPTION = /* GraphQL */ `
  subscription PriceUpdated($marketId: Int!) {
    priceUpdated(marketId: $marketId) {
      marketId
      price
      updatedAt
      blockNumber
    }
  }
`;

export function useSingleMarketPriceSubscription(marketId: number | undefined) {
  return useIndexerSelection<IndexerPriceUpdate>(marketId, (onData) =>
    subscribeStream<{ priceUpdated: IndexerPriceUpdate }>({
      endpoint: INDEXER_GRAPHQL,
      query: PRICE_UPDATED_SUBSCRIPTION,
      variables: { marketId },
      onData: (data) => onData(data.priceUpdated),
    }),
  );
}

// ── positionChanged(userAddress) ───────────────────────────────────────

const POSITION_CHANGED_SUBSCRIPTION = /* GraphQL */ `
  subscription PositionChanged($userAddress: String!) {
    positionChanged(userAddress: $userAddress) {
      positionId
      marketId
      isLong
      sizeUsd
      leverage
      entryPrice
      status
      realizedPnl
    }
  }
`;

export function usePositionChangedSubscription(
  userAddress: string | undefined,
) {
  const account = userAddress?.toLowerCase() || undefined;
  return useIndexerSelection<IndexerPosition[]>(account, (onData) =>
    subscribeStream<{ positionChanged: IndexerPosition[] }>({
      endpoint: INDEXER_GRAPHQL,
      query: POSITION_CHANGED_SUBSCRIPTION,
      variables: { userAddress: account },
      onData: (data) => onData(data.positionChanged),
    }),
  );
}

// ── orderChanged(userAddress) ──────────────────────────────────────────

const ORDER_CHANGED_SUBSCRIPTION = /* GraphQL */ `
  subscription OrderChanged($userAddress: String!) {
    orderChanged(userAddress: $userAddress) {
      orderId
      status
      marketId
      orderType
      remainingSize
      blockTimestamp
    }
  }
`;

export function useOrderChangedSubscription(userAddress: string | undefined) {
  const selection = userAddress?.toLowerCase() || undefined;
  return useIndexerSelection<IndexerOrder>(selection, (onData) => {
    if (selection === undefined) return () => {};
    return subscribeIndexerOrder(selection, onData);
  });
}

export function subscribeIndexerOrder(
  userAddress: string,
  onOrder: (order: IndexerOrder) => void,
) {
  return subscribeStream<{ orderChanged: unknown }>({
    endpoint: INDEXER_GRAPHQL,
    query: ORDER_CHANGED_SUBSCRIPTION,
    variables: { userAddress },
    onData: (data) => onOrder(decodeIndexerOrder(data.orderChanged)),
  });
}

// ── tradeCreated(marketId) ─────────────────────────────────────────────

const TRADE_CREATED_SUBSCRIPTION = /* GraphQL */ `
  subscription TradeCreated($marketId: Int!) {
    tradeCreated(marketId: $marketId) {
      positionId
      userAddress
      sizeUsd
      executionPrice
      tradeType
      blockTimestamp
    }
  }
`;

export function useTradeCreatedSubscription(marketId: number | undefined) {
  const selection = marketId;
  return useIndexerSelection<IndexerTrade>(selection, (onData) => {
    if (selection === undefined) return () => {};
    return subscribeIndexerTrade(selection, onData);
  });
}

export function subscribeIndexerTrade(
  marketId: number,
  onTrade: (trade: IndexerTrade) => void,
) {
  return subscribeStream<{ tradeCreated: unknown }>({
    endpoint: INDEXER_GRAPHQL,
    query: TRADE_CREATED_SUBSCRIPTION,
    variables: { marketId },
    onData: (data) => onTrade(decodeIndexerTrade(data.tradeCreated)),
  });
}

// ── liquidationCreated(marketId) ───────────────────────────────────────

const LIQUIDATION_SUBSCRIPTION = /* GraphQL */ `
  subscription Liquidations($marketId: Int!) {
    liquidationCreated(marketId: $marketId) {
      positionId
      userAddress
      sizeUsd
      executionPrice
      blockTimestamp
    }
  }
`;

export function useLiquidationSubscription(marketId: number | undefined) {
  const selection = marketId;
  return useIndexerSelection<IndexerLiquidation>(selection, (onData) => {
    if (selection === undefined) return () => {};
    return subscribeIndexerLiquidation(selection, onData);
  });
}

export function subscribeIndexerLiquidation(
  marketId: number,
  onLiquidation: (liquidation: IndexerLiquidation) => void,
) {
  return subscribeStream<{ liquidationCreated: unknown }>({
    endpoint: INDEXER_GRAPHQL,
    query: LIQUIDATION_SUBSCRIPTION,
    variables: { marketId },
    onData: (data) =>
      onLiquidation(decodeIndexerLiquidation(data.liquidationCreated)),
  });
}
