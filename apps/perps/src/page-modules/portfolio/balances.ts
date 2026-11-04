import { useEffect, useMemo, useSyncExternalStore } from "react";
import type { Address } from "viem";

import { COLLATERAL_TOKENS } from "@/core/perps/config";
import { getAccount, getWalletBalance } from "@/hooks/perps/useAccount";
import type { TokenBalance } from "@/widgets/portfolio/types";

type Source<T> =
  | { status: "idle" | "loading" }
  | { status: "ready"; value: T }
  | { status: "error"; error: unknown };

export interface PortfolioBalanceSnapshot {
  address: Address | null;
  account: Source<Awaited<ReturnType<typeof getAccount>>>;
  wallet: Source<bigint>;
}

function initialSnapshot(address: Address | null): PortfolioBalanceSnapshot {
  const status = address ? "loading" : "idle";
  return { address, account: { status }, wallet: { status } };
}

export class PortfolioBalanceReader {
  private snapshot: PortfolioBalanceSnapshot;
  private generation = 0;
  private listeners = new Set<() => void>();

  constructor(readonly address: Address | null) {
    this.snapshot = initialSnapshot(address);
  }

  getSnapshot = () => this.snapshot;

  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => { this.listeners.delete(listener); };
  };

  private publish(snapshot: PortfolioBalanceSnapshot) {
    this.snapshot = snapshot;
    this.listeners.forEach((listener) => listener());
  }

  refresh = async () => {
    const generation = ++this.generation;
    this.publish(initialSnapshot(this.address));
    if (!this.address) return;

    const read = async <K extends "account" | "wallet">(
      source: K,
      request: () => Promise<Extract<PortfolioBalanceSnapshot[K], { status: "ready" }>["value"]>,
    ) => {
      try {
        const value = await request();
        if (generation === this.generation) {
          this.publish({ ...this.snapshot, [source]: { status: "ready", value } });
        }
      } catch (error) {
        if (generation === this.generation) {
          this.publish({ ...this.snapshot, [source]: { status: "error", error } });
        }
      }
    };
    const address = this.address;
    await Promise.all([
      read("account", () => getAccount(address)),
      read("wallet", () => getWalletBalance(address)),
    ]);
  };

  cancel = () => { ++this.generation; };
}

export function portfolioBalances(snapshot: PortfolioBalanceSnapshot): TokenBalance[] {
  if (snapshot.account.status !== "ready" || snapshot.wallet.status !== "ready") return [];
  const { collateral, withdrawable } = snapshot.account.value;
  return [{
    ...COLLATERAL_TOKENS.usdl,
    vaultBalance: collateral,
    lockedBalance: collateral > withdrawable ? collateral - withdrawable : 0n,
    walletBalance: snapshot.wallet.value,
    available: withdrawable,
  }];
}

export function usePortfolioBalances(address: Address | null, refreshCounter: number) {
  // A different account gets an empty snapshot during render, before effects run.
  const reader = useMemo(() => new PortfolioBalanceReader(address), [address]);
  const snapshot = useSyncExternalStore(reader.subscribe, reader.getSnapshot, reader.getSnapshot);
  useEffect(() => {
    void reader.refresh();
    return reader.cancel;
  }, [reader, refreshCounter]);
  return { snapshot, retry: reader.refresh, balances: portfolioBalances(snapshot) };
}
