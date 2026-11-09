"use client";

import React from "react";
import { ArrowDownToLine, ArrowUpFromLine } from "lucide-react";
import { Button } from "@/ui/atoms/button";
import type { PortfolioBalanceSnapshot } from "@/page-modules/portfolio/balances";
import type { PortfolioNavKey } from "./PortfolioSideNav";

/**
 * Sticky-feeling header strip rendered above the active tab panel.
 *
 *   PORTFOLIO                                   [Connect | Create vault | Deposit / Withdraw]
 *   Active tab title
 *
 * The CTA cluster on the right adapts to wallet/vault state so the user
 * always sees the next action they can actually take.
 */

const NAV_LABELS: Record<PortfolioNavKey, string> = {
  overview: "Overview",
  positions: "Positions",
  orders: "Orders",
  settlements: "Settlements",
  assets: "Assets",
  feeTier: "Fee tier",
  apiKeys: "API Keys",
  settings: "Settings",
};

export interface PortfolioTopBarProps {
  activeNav: PortfolioNavKey;
  isConnected: boolean;
  hasVault: boolean;
  /** True once the on-chain vault lookup has resolved. Suppresses CTA flicker. */
  vaultResolved: boolean;
  accountStatus: PortfolioBalanceSnapshot["account"]["status"];
  walletStatus: PortfolioBalanceSnapshot["wallet"]["status"];
  onRetryBalances: () => void;
  onDeposit: () => void;
  onWithdraw: () => void;
  onConnect: () => void;
}

export function portfolioTopBarBalanceProps(
  snapshot: PortfolioBalanceSnapshot,
  onRetryBalances: () => void,
): Pick<
  PortfolioTopBarProps,
  "vaultResolved" | "accountStatus" | "walletStatus" | "onRetryBalances"
> {
  return {
    vaultResolved:
      snapshot.account.status === "ready" && snapshot.wallet.status === "ready",
    accountStatus: snapshot.account.status,
    walletStatus: snapshot.wallet.status,
    onRetryBalances,
  };
}

export function PortfolioTopBar({
  activeNav,
  isConnected,
  hasVault,
  vaultResolved,
  accountStatus,
  walletStatus,
  onRetryBalances,
  onDeposit,
  onWithdraw,
  onConnect,
}: PortfolioTopBarProps) {
  const balanceFailed = accountStatus === "error" || walletStatus === "error";
  const balancePending =
    accountStatus === "loading" || walletStatus === "loading";

  return (
    <div className="flex items-center justify-between gap-3 flex-wrap">
      <div>
        <div className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
          Portfolio
        </div>
        <h1 className="mt-0.5 text-xl md:text-2xl font-semibold text-foreground tracking-tight">
          {NAV_LABELS[activeNav]}
        </h1>
      </div>

      <div className="flex items-center gap-2">
        {!isConnected ? (
          <Button
            onClick={onConnect}
            className="h-9 rounded-[var(--radius-md)]"
          >
            Connect wallet
          </Button>
        ) : balanceFailed ? (
          <div
            role="alert"
            aria-busy={balancePending}
            className="flex items-center gap-2 text-xs"
          >
            <span>Portfolio balances are unavailable.</span>
            <Button
              type="button"
              onClick={onRetryBalances}
              disabled={balancePending}
              variant="outline"
              className="h-9 rounded-[var(--radius-md)]"
            >
              Retry balances
            </Button>
          </div>
        ) : !vaultResolved ? (
          <div
            role="status"
            aria-busy={balancePending}
            className="h-9 px-3 rounded-[var(--radius-md)] bg-surface-1 border border-white/[0.04] flex items-center text-xs text-muted-foreground"
          >
            {balancePending ? "Resolving vault…" : "Waiting for wallet…"}
          </div>
        ) : !hasVault ? (
          <Button
            onClick={onDeposit}
            className="h-9 rounded-[var(--radius-md)]"
          >
            Create vault
          </Button>
        ) : (
          <>
            <Button
              onClick={onDeposit}
              className="h-9 rounded-[var(--radius-md)]"
            >
              <ArrowDownToLine className="h-4 w-4 mr-1.5" />
              Deposit
            </Button>
            <Button
              onClick={onWithdraw}
              variant="outline"
              className="h-9 rounded-[var(--radius-md)]"
            >
              <ArrowUpFromLine className="h-4 w-4 mr-1.5" />
              Withdraw
            </Button>
          </>
        )}
      </div>
    </div>
  );
}
