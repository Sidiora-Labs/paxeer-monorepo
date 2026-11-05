"use client";

import { useEffect, useMemo, useState } from "react";
import { usePerpsWallet } from "./usePerpsWallet";
import { subgraphQuery, POSITIONS_QUERY, rawUint256 } from "./useSubgraph";
import { z } from "zod";
import { buildGetAccount, type AccountState } from "@/core/perps/calls";
import { portfolioFromAccount } from "@/core/perps/portfolio";
import { getPublicClient } from "@/core/viem/clients";
import { INDEXER_GRAPHQL } from "@/core/perps/config";
import { subscribeStream } from "@/core/graphql/client";
import {
  activePoll,
  isPageActive,
  onStreamResync,
  type CancelTask,
} from "@/core/net/realtime";

type AccountTotals = Pick<
  AccountState,
  "collateral" | "equityUsd" | "withdrawable"
>;

const EMPTY_ACCOUNT: AccountTotals = {
  collateral: 0n,
  equityUsd: 0n,
  withdrawable: 0n,
};

const portfolioPositionsSchema = z.object({
  positions: z.array(
    z.object({
      positionId: rawUint256,
      userAddress: z.string(),
      marketId: z.number().int().nonnegative(),
      isLong: z.boolean(),
      sizeUsd: rawUint256,
      leverage: rawUint256,
      entryPrice: rawUint256,
      collateralToken: z.string().nullable(),
      collateralAmount: rawUint256,
      collateralUsd: rawUint256,
      status: z.literal("open"),
      openedAt: z.string().nullable(),
      openTxHash: z.string().nullable(),
      market: z.object({ symbol: z.string() }).nullable(),
    }),
  ),
});

export function decodePortfolioPositions(data: unknown) {
  return portfolioPositionsSchema.parse(data).positions;
}

export async function fetchPortfolioPositions(addr: string) {
  return subgraphQuery(
    POSITIONS_QUERY,
    { userAddress: addr },
    decodePortfolioPositions,
  );
}

export interface PortfolioStats {
  totalVaultUsd: number;
  freeCollateral: number;
  unrealizedPnl: number;
  totalLeverage: number;
  maintenanceMarginUsd: AccountState["maintenanceMarginUsd"] | null;
  isLoading: boolean;
  positions: any[];
}

/* ══════════════════════════════════════════════════════════════════════
   Module-level singleton — one account poller (15s) + one positions
   stream per connected address, shared by all subscribers.
   ══════════════════════════════════════════════════════════════════════ */

interface RawSnapshot {
  positions: any[];
  account: AccountTotals;
  accountAddress: string | null;
  maintenanceMarginUsd: AccountState["maintenanceMarginUsd"] | null;
  isLoading: boolean;
}

const EMPTY_RAW: RawSnapshot = {
  positions: [],
  account: EMPTY_ACCOUNT,
  accountAddress: null,
  maintenanceMarginUsd: null,
  isLoading: false,
};

let _raw: RawSnapshot = EMPTY_RAW;
const _subs = new Set<(s: RawSnapshot) => void>();
let _activeAddress: string | null = null;
interface PortfolioSession {
  address: string;
  liveVersion: number;
  snapshotRequest: number;
}
let _session: PortfolioSession | null = null;
let _accountTimer: CancelTask | null = null;
let _resyncCleanup: CancelTask | null = null;
let _posUnsubscribe: (() => void) | null = null;

// ── Locally-closed position IDs ────────────────────────────────────────
// Tracks positionIds the user closed during this browser session so the
// rows stay hidden in the UI even when:
//   • the user navigates between /perps and /portfolio (component
//     unmount destroys local state — but this Set is module-level)
//   • the user hard-reloads while the indexer hasn't yet processed the
//     close (sessionStorage rehydrates the Set on next load)
//
// Self-healing: each time fresh raw positions arrive from the indexer
// we drop any ID in this Set that the indexer has already removed from
// its result. So the list shrinks naturally once the backend catches up.
const _locallyClosed = new Set<string>();
const _LOCAL_CLOSED_SS_KEY = "perps.locallyClosed.v1";

if (typeof window !== "undefined") {
  try {
    const saved = sessionStorage.getItem(_LOCAL_CLOSED_SS_KEY);
    if (saved)
      JSON.parse(saved).forEach((id: string) => _locallyClosed.add(String(id)));
  } catch {}
}

function _persistLocallyClosed() {
  if (typeof window === "undefined") return;
  try {
    sessionStorage.setItem(
      _LOCAL_CLOSED_SS_KEY,
      JSON.stringify([..._locallyClosed]),
    );
  } catch {}
}

function _reconcileLocallyClosed(rawPositions: any[]) {
  if (_locallyClosed.size === 0) return;
  const live = new Set(rawPositions.map((p: any) => String(p.positionId)));
  let changed = false;
  for (const id of [..._locallyClosed]) {
    if (!live.has(id)) {
      _locallyClosed.delete(id);
      changed = true;
    }
  }
  if (changed) _persistLocallyClosed();
}

/**
 * Mark a position as locally closed. Hides it from `usePortfolioStats().positions`
 * everywhere (perps panel, portfolio panel) until the indexer confirms the
 * close and removes it from its result. Persisted to sessionStorage so it
 * survives page reloads.
 */
export function markPositionLocallyClosed(positionId: string | bigint) {
  _locallyClosed.add(String(positionId));
  _persistLocallyClosed();
  _notify();
}

/** Roll back a `markPositionLocallyClosed` call (e.g. on close-tx failure). */
export function unmarkPositionLocallyClosed(positionId: string | bigint) {
  if (_locallyClosed.delete(String(positionId))) {
    _persistLocallyClosed();
    _notify();
  }
}

function _notify() {
  // Spread on every notify so React subscribers always see a new object
  // reference — needed because `markPositionLocallyClosed` doesn't mutate
  // `_raw` itself but still requires consumers to re-derive their filtered
  // positions list.
  const snapshot = { ..._raw };
  _subs.forEach((cb) => cb(snapshot));
}

function _setRaw(patch: Partial<RawSnapshot>) {
  _raw = { ..._raw, ...patch };
  _notify();
}

async function _fetchAccount(session: PortfolioSession) {
  const addr = session.address;
  if (!isPageActive()) return;
  try {
    const account = await getPublicClient().readContract(
      buildGetAccount(addr as `0x${string}`),
    );
    if (_session !== session) return;
    _setRaw({
      account,
      accountAddress: addr,
      maintenanceMarginUsd: account.maintenanceMarginUsd,
    });
  } catch {
    if (_session === session) _setRaw({ maintenanceMarginUsd: null });
  }
}

const POSITION_CHANGED_QUERY = /* GraphQL */ `
  subscription PositionChanged($userAddress: String!) {
    positionChanged(userAddress: $userAddress) {
      positionId
      marketId
      isLong
      sizeUsd
      leverage
      entryPrice
      collateralToken
      collateralAmount
      collateralUsd
      status
      openedAt
      openTxHash
      market {
        symbol
      }
    }
  }
`;

async function _fetchPositionsOnce(session: PortfolioSession) {
  const addr = session.address;
  const liveVersion = session.liveVersion;
  const request = ++session.snapshotRequest;
  try {
    const positions = await fetchPortfolioPositions(addr);
    // A full-list push supersedes every snapshot already in flight. Session
    // identity also invalidates requests across a stop/reopen of the same account.
    if (
      _session !== session ||
      session.liveVersion !== liveVersion ||
      session.snapshotRequest !== request
    )
      return;
    _reconcileLocallyClosed(positions);
    _setRaw({ positions });
  } catch {}
}

/**
 * Force-refresh the positions list from the subgraph (HTTP) — used
 * after a close / partial-close / margin / size action when the UI
 * shouldn't have to wait for the WS `positionChanged` push to land.
 *
 * Issues an immediate HTTP fetch and schedules two backoff retries
 * (2 s and 6 s) so we still catch the indexer's state once it has
 * processed the on-chain event, even if it lagged behind our first
 * fetch. Silent: no spinner, no UI signal.
 *
 * Captures the active account session at call time so unmounting or
 * switching accounts mid-flight cancels remaining retries cleanly.
 */
export function refreshPositions() {
  const session = _session;
  if (!session) return;
  _fetchPositionsOnce(session);
  for (const delay of [2_000, 6_000]) {
    setTimeout(() => {
      if (_session === session) _fetchPositionsOnce(session);
    }, delay);
  }
}

function _stopPolling() {
  _session = null;
  if (_accountTimer) {
    _accountTimer();
    _accountTimer = null;
  }
  _resyncCleanup?.();
  _resyncCleanup = null;
  if (_posUnsubscribe) {
    _posUnsubscribe();
    _posUnsubscribe = null;
  }
}

function _startPolling(addr: string) {
  _stopPolling();
  _activeAddress = addr;
  const session: PortfolioSession = {
    address: addr,
    liveVersion: 0,
    snapshotRequest: 0,
  };
  _session = session;
  _raw = { ...EMPTY_RAW, isLoading: true };
  _notify();
  Promise.all([_fetchAccount(session), _fetchPositionsOnce(session)]).finally(
    () => {
      if (_session === session) _setRaw({ isLoading: false });
    },
  );
  _accountTimer = activePoll(() => _fetchAccount(session), 15_000, false);
  _resyncCleanup = onStreamResync((url) => {
    if (url === INDEXER_GRAPHQL.wsUrl && _session === session) {
      void _fetchPositionsOnce(session);
      void _fetchAccount(session);
    }
  });
  // Subscribe to positionChanged — server pushes full position list on each change.
  // The initial query above fills the gap while the WS handshake completes.
  _posUnsubscribe = subscribeStream<{ positionChanged: any[] }>({
    endpoint: INDEXER_GRAPHQL,
    query: POSITION_CHANGED_QUERY,
    variables: { userAddress: addr },
    onData: (data) => {
      if (_session !== session) return;
      session.liveVersion++;
      const positions = data.positionChanged ?? [];
      _reconcileLocallyClosed(positions);
      _setRaw({ positions });
    },
  });
}

function _reset() {
  _stopPolling();
  _activeAddress = null;
  _raw = EMPTY_RAW;
  _notify();
}

/**
 * Lightweight portfolio stats hook — shared singleton pollers.
 * - Positions from the indexer stream (no RPC), shared
 * - Account totals: one getAccount(account) read at 0x1015 every 15s, shared
 */
export function usePortfolioStats(): PortfolioStats {
  const { isConnected, address } = usePerpsWallet();
  const [raw, setRaw] = useState<RawSnapshot>(_raw);

  const wantedAddress = isConnected && address ? address : null;

  useEffect(() => {
    _subs.add(setRaw);
    // Sync in case snapshot changed between render and subscribe
    if (raw !== _raw) setRaw(_raw);

    if (wantedAddress) {
      if (_activeAddress !== wantedAddress) _startPolling(wantedAddress);
    } else {
      if (_activeAddress !== null) _reset();
    }

    return () => {
      _subs.delete(setRaw);
      if (_subs.size === 0) _reset();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [wantedAddress]);

  // Hide IDs the user closed locally until the indexer drops them. The
  // filtering happens here (not in `_setRaw`) so we always keep the raw
  // indexer response intact for reconciliation, while consumers see only
  // positions that should still be open from the user's perspective.
  const visiblePositions = useMemo(
    () =>
      raw.positions.filter(
        (p: any) => !_locallyClosed.has(String(p.positionId)),
      ),
    [raw],
  );

  const totals = useMemo(
    () => portfolioFromAccount(raw.account, visiblePositions),
    [raw.account, visiblePositions],
  );

  return {
    ...totals,
    maintenanceMarginUsd:
      wantedAddress !== null && raw.accountAddress === wantedAddress
        ? raw.maintenanceMarginUsd
        : null,
    isLoading: raw.isLoading,
    positions: visiblePositions,
  };
}
