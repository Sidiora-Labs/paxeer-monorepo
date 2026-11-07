"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/router";

import { ErrorBoundary } from "@/app/shell/ErrorBoundary";
import { DepositWithdrawModal } from "@/app/modals/DepositWithdrawModal";
import { SettingsModal } from "@/app/modals/SettingsModal";

import { useIsMobile } from "@/hooks/ui/useMobile";
import { usePerpsWallet } from "@/hooks/perps/usePerpsWallet";
import { usePortfolioStats } from "@/hooks/perps/usePortfolioStats";
import { useDailyPnl } from "@/hooks/perps/useDailyPnl";
import { usePerfdbTraderStats } from "@/hooks/perps/usePerfdbTraderStats";

import { parse18 } from "@/ui/utils/format";

import {
  PortfolioSideNav,
  PortfolioTopBar,
  PortfolioMobileNav,
  PortfolioOverviewPanel,
  PortfolioPositionsPanel,
  PortfolioFeeTierPanel,
  PortfolioSettingsPanel,
  TradeSettlementHistory,
  OnChainHistoryPanel,
  SectionCard,
  PlaceholderPanel,
  ConnectGate,
  type PortfolioNavKey,
} from "@/widgets/portfolio";
import { PortfolioSpotSection } from "@/widgets/portfolio/PortfolioSpotSection";
import { usePortfolioBalances } from "./balances";
import { PortfolioBalanceBoundary } from "./PortfolioBalanceBoundary";
import { PortfolioOrdersAvailability } from "./PortfolioOrdersAvailability";

/* ══════════════════════════════════════════════════════════════════════
   /portfolio page — thin orchestrator.

   Owns: hook wiring (wallet, account, daily PnL, trader stats, balances),
         page-local UI state (active tab, sidebar collapsed, modals),
         deep-link routing via `?tab=…`.

   Renders: layout chrome from `@/widgets/portfolio` + dispatches the
            right tab panel. Every panel lives in its own widget file.
   ══════════════════════════════════════════════════════════════════════ */

/** The overview panel's daily PnL props from one useDailyPnl result. */
export function overviewDailyPnlProps(result: ReturnType<typeof useDailyPnl>) {
  return {
    dailyPnl: result.dailyPnl,
    dailyPnlLoading: result.isLoading,
    dailyPnlError: result.error,
    dailyPnlSkippedDays: result.skippedDays,
    dailyPnlHistoryCapped: result.historyCapped,
  };
}

const ALLOWED_NAV: PortfolioNavKey[] = [
  "overview",
  "positions",
  "orders",
  "settlements",
  "assets",
  "feeTier",
  "apiKeys",
  "settings",
];

export default function PortfolioPage() {
  const router = useRouter();
  const isMobile = useIsMobile();

  const { isConnected, address, connect } = usePerpsWallet();
  const {
    totalVaultUsd,
    unrealizedPnl,
    positions,
    isLoading: statsLoading,
  } = usePortfolioStats();
  const dailyPnlResult = useDailyPnl(address ?? undefined);
  const { stats: traderStats } = usePerfdbTraderStats(address ?? undefined);

  // ── Page-local UI state ──
  const [activeNav, setActiveNav] = useState<PortfolioNavKey>("overview");
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [depositModalOpen, setDepositModalOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [privacy, setPrivacy] = useState(false);
  const [refreshCounter, setRefreshCounter] = useState(0);

  const { snapshot: balanceSnapshot, balances, retry: retryBalances } = usePortfolioBalances(
    isConnected && address ? address as `0x${string}` : null,
    refreshCounter,
  );
  const accountResolved = balanceSnapshot.account.status === "ready" &&
    balanceSnapshot.wallet.status === "ready";

  // ── Deep-link the nav tab via `?tab=overview|positions|…` ──
  useEffect(() => {
    const q = router.query.tab;
    if (typeof q !== "string") return;
    if ((ALLOWED_NAV as string[]).includes(q)) {
      setActiveNav(q as PortfolioNavKey);
    }
  }, [router.query.tab]);

  // ── Derived ──
  const realizedPnl = useMemo(
    () => (traderStats?.totalPnl ? parse18(traderStats.totalPnl) : 0),
    [traderStats],
  );
  const totalTrades = traderStats?.totalTrades ?? 0;

  // ── Stable handlers ──
  const onTogglePrivacy = useCallback(() => setPrivacy((p) => !p), []);
  const onRefresh = useCallback(() => setRefreshCounter((c) => c + 1), []);
  const onDepositOrWithdraw = useCallback(() => setDepositModalOpen(true), []);

  return (
    <div className="flex flex-col md:flex-row min-h-full bg-background text-foreground">
      {!isMobile && (
        <PortfolioSideNav
          active={activeNav}
          onChange={setActiveNav}
          collapsed={sidebarCollapsed}
          onToggleCollapsed={() => setSidebarCollapsed((c) => !c)}
        />
      )}

      <main className="flex-1 min-w-0 min-h-0 w-full px-4 md:px-6 xl:px-8 py-4 md:py-6 space-y-4 md:space-y-6">
        <PortfolioTopBar
          activeNav={activeNav}
          isConnected={isConnected}
          hasVault={isConnected}
          vaultResolved={accountResolved}
          onDeposit={onDepositOrWithdraw}
          onWithdraw={onDepositOrWithdraw}
          onConnect={connect}
        />

        {isMobile ? (
          <PortfolioMobileNav active={activeNav} onChange={setActiveNav} />
        ) : null}

        <ErrorBoundary fallbackLabel={`Portfolio · ${activeNav}`}>
          {activeNav === "overview" ? (
            <PortfolioBalanceBoundary snapshot={balanceSnapshot} onRetry={retryBalances}>
              <PortfolioOverviewPanel
                isConnected={isConnected}
                hasVault={isConnected}
                address={address}
                totalVaultUsd={totalVaultUsd}
                unrealizedPnl={unrealizedPnl}
                openPositions={positions.length}
                realizedPnl={realizedPnl}
                totalTrades={totalTrades}
                balances={balances}
                {...overviewDailyPnlProps(dailyPnlResult)}
                isLoading={statsLoading}
                privacy={privacy}
                onTogglePrivacy={onTogglePrivacy}
                onRefresh={onRefresh}
                onConnect={connect}
                onDeposit={onDepositOrWithdraw}
                onWithdraw={onDepositOrWithdraw}
              />
            </PortfolioBalanceBoundary>
          ) : activeNav === "positions" ? (
            <PortfolioPositionsPanel
              positions={positions}
              isConnected={isConnected}
              hasVault={isConnected}
              onConnect={connect}
            />
          ) : activeNav === "orders" ? (
            <>
              <PortfolioOrdersAvailability />
              <PortfolioSpotSection
                title="Spot Orders"
                emptyMessage="No open spot orders"
              />
            </>
          ) : activeNav === "settlements" ? (
            <SectionCard title="Trade settlements">
              <TradeSettlementHistory userAddress={address ?? undefined} />
            </SectionCard>
          ) : activeNav === "assets" ? (
            <SectionCard title="On-chain activity">
              {address ? (
                <OnChainHistoryPanel address={address} variant="desktop" />
              ) : (
                <ConnectGate isConnected={isConnected} onConnect={connect} />
              )}
            </SectionCard>
          ) : activeNav === "feeTier" ? (
            <PortfolioFeeTierPanel address={address ?? undefined} />
          ) : activeNav === "apiKeys" ? (
            <PlaceholderPanel
              title="API keys"
              subtitle="Programmatic trading access."
              hint="API key issuance is on the roadmap. For now use the SDK with your own wallet signer."
            />
          ) : (
            <PortfolioSettingsPanel onOpenFullSettings={() => setSettingsOpen(true)} />
          )}
        </ErrorBoundary>
      </main>

      <DepositWithdrawModal
        isOpen={depositModalOpen}
        onClose={() => {
          setDepositModalOpen(false);
          setRefreshCounter((c) => c + 1);
        }}
      />
      <SettingsModal
        isOpen={settingsOpen}
        onClose={() => setSettingsOpen(false)}
      />
    </div>
  );
}
