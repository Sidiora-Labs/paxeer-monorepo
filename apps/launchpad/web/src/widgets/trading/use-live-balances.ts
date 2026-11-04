'use client';
import { useCallback, useEffect, useRef, useState } from 'react';
import type { Address } from 'viem';
import { failureReason } from './transaction-controls/runtime';

/** The one chain read the panel needs: ERC-20 `balanceOf` at the latest block (ChainTradeProvider.readBalance). */
export interface BalanceReader { readBalance(token: Address, account: Address, blockNumber?: bigint, signal?: AbortSignal): Promise<bigint> }

/**
 * A wallet balance may be read and shown only for an authenticated account
 * session (the header's sign-in) of exactly the connected wallet. A connected
 * or auto-reconnected wallet without that session is signed out.
 */
export function authenticatedFor(walletSelected: boolean, connected: string | null | undefined, sessionWallet: string | null | undefined): boolean {
  return walletSelected && !!connected && !!sessionWallet && connected.toLowerCase() === sessionWallet.toLowerCase();
}

/** Re-read cadence while signed in; stays inside the 10s freshness gate trade execution enforces. */
export const BALANCE_POLL_MS = 4000;

export interface LiveBalances {
  usdl: bigint | null;
  token: bigint | null;
  /** Time of the last successful chain read. */
  observedAt: number | undefined;
  /** Full reason of the most recent failed read; cleared by the next successful read. */
  error: string | undefined;
  refresh(): void;
}

export interface LiveBalanceArgs {
  reader: BalanceReader | null;
  usdl: Address | null;
  token: Address | null;
  account: Address | null;
  /** Only an authenticated session for `account` may read; otherwise nothing is held or shown. */
  enabled: boolean;
  /** Changes whenever previously read values must be discarded (pool, session epoch). */
  scope?: string;
  intervalMs?: number;
}

/**
 * Live USDL and token balances of the signed-in wallet, read straight from
 * chain 125 and re-read every BALANCE_POLL_MS while the page is visible. When
 * the session ends or the account changes, held values are dropped at once.
 */
export function useLiveBalances({ reader, usdl, token, account, enabled, scope = '', intervalMs = BALANCE_POLL_MS }: LiveBalanceArgs): LiveBalances {
  const empty = { usdl: null, token: null, observedAt: undefined, error: undefined };
  const [state, setState] = useState<Omit<LiveBalances, 'refresh'> & { key?: string; reader?: BalanceReader | null }>(empty);
  const refreshRef = useRef<() => void>(() => {});
  const active = enabled && !!reader && !!usdl && !!token && !!account;
  const key = active ? `${account!.toLowerCase()}|${usdl!.toLowerCase()}|${token!.toLowerCase()}|${scope}` : '';

  useEffect(() => {
    setState(empty);
    if (!active) return;
    let live = true;
    let pending = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let controller: AbortController | undefined;
    const visible = () => typeof document === 'undefined' || document.visibilityState !== 'hidden';
    const poll = async () => {
      if (!live || pending || !visible()) return;
      clearTimeout(timer);
      pending = true;
      controller = new AbortController();
      try {
        const results = await Promise.allSettled([reader!.readBalance(usdl!, account!, undefined, controller.signal), reader!.readBalance(token!, account!, undefined, controller.signal)]);
        if (!live) return;
        const [usdlResult, tokenResult] = results;
        if (usdlResult.status === 'rejected') throw usdlResult.reason;
        if (tokenResult.status === 'rejected') throw tokenResult.reason;
        setState({ key, reader, usdl: usdlResult.value, token: tokenResult.value, observedAt: Date.now(), error: undefined });
      } catch (error) {
        if (live) setState(previous => ({ ...previous, key, reader, error: failureReason(error) }));
      } finally {
        pending = false;
        controller = undefined;
        if (live && intervalMs > 0 && visible()) timer = setTimeout(() => void poll(), intervalMs);
      }
    };
    const onVisible = () => {
      if (visible()) void poll();
      else { clearTimeout(timer); }
    };
    refreshRef.current = () => { void poll(); };
    if (typeof document !== 'undefined') document.addEventListener('visibilitychange', onVisible);
    void poll();
    return () => {
      live = false;
      clearTimeout(timer);
      controller?.abort();
      refreshRef.current = () => {};
      if (typeof document !== 'undefined') document.removeEventListener('visibilitychange', onVisible);
    };
  }, [active, key, reader, intervalMs]); // eslint-disable-line react-hooks/exhaustive-deps

  const refresh = useCallback(() => refreshRef.current(), []);
  return active && state.key === key && state.reader === reader ? { ...state, refresh } : { ...empty, refresh };
}
