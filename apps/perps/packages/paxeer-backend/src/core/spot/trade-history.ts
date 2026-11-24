import type { SpotTrade, SpotTradesResponse } from "./api";

export interface SpotTradeHistory {
  snapshot: SpotTradesResponse | null;
  page: SpotTradesResponse | null;
}

function mergeTrades(...groups: readonly SpotTrade[][]): SpotTrade[] {
  const trades = new Map<string, SpotTrade>();
  for (const group of groups) {
    for (const trade of group) {
      const id = String(trade.id);
      if (!trades.has(id)) trades.set(id, trade);
    }
  }
  return [...trades.values()].sort((a, b) => {
    const aBlock = BigInt(a.blockNumber);
    const bBlock = BigInt(b.blockNumber);
    return aBlock === bBlock
      ? b.logIndex - a.logIndex
      : aBlock > bBlock
        ? -1
        : 1;
  });
}

export function spotTradeSnapshotForAddress(
  snapshot: SpotTradesResponse | null,
  address: string | null | undefined,
): SpotTradesResponse | null {
  return address && snapshot?.address.toLowerCase() === address.toLowerCase()
    ? snapshot
    : null;
}

export function mergeSpotTradeSnapshot(
  snapshot: SpotTradesResponse,
  live: SpotTrade[] = [],
): SpotTradesResponse {
  const cursor = snapshot.nextCursor;
  const visibleLive = cursor
    ? live.filter((trade) => {
        const block = BigInt(trade.blockNumber);
        const afterBlock = BigInt(cursor.afterBlock);
        return (
          block > afterBlock ||
          (block === afterBlock && trade.logIndex >= cursor.afterIndex)
        );
      })
    : live;
  const merged = mergeTrades(visibleLive, snapshot.items);
  const items = merged.slice(0, 200);
  const last = items[items.length - 1];
  return {
    ...snapshot,
    items,
    count: items.length,
    nextCursor:
      merged.length > items.length
        ? { afterBlock: String(last.blockNumber), afterIndex: last.logIndex }
        : snapshot.nextCursor,
  };
}

export function spotTradeHistory(
  snapshot: SpotTradesResponse | null,
): SpotTradeHistory {
  return { snapshot, page: snapshot };
}

export function appendSpotTradeHistoryPage(
  history: SpotTradeHistory,
  snapshot: SpotTradesResponse,
  cursor: NonNullable<SpotTradesResponse["nextCursor"]>,
  response: SpotTradesResponse,
): SpotTradeHistory {
  if (
    history.snapshot !== snapshot ||
    history.page?.nextCursor !== cursor ||
    response.address.toLowerCase() !== snapshot.address.toLowerCase()
  ) {
    return history;
  }
  const items = mergeTrades(history.page.items, response.items);
  return {
    snapshot,
    page: { ...response, items, count: items.length },
  };
}
