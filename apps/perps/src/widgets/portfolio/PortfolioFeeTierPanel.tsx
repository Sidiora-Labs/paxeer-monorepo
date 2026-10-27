"use client";

import * as React from "react";
import { Percent } from "lucide-react";
import type { FeeTierView } from "@/core/perps/feeTier";
import { useFeeTier } from "@/hooks/perps/useFeeTier";
import { FeesBreakdownCard } from "./FeesBreakdownCard";
import { SectionCard } from "./PortfolioPrimitives";

export interface PortfolioFeeTierPanelProps {
  address: string | undefined;
  view?: FeeTierView;
}

export function PortfolioFeeTierPanel({ address, view }: PortfolioFeeTierPanelProps) {
  if (view === undefined) return <ConnectedFeeTierPanel address={address} />;
  return <FeeTierPanelContent address={address} view={view} />;
}

function ConnectedFeeTierPanel({ address }: Pick<PortfolioFeeTierPanelProps, "address">) {
  const { view, isLoading, error } = useFeeTier(address);
  return (
    <FeeTierPanelContent address={address} view={view} isLoading={isLoading} error={error} />
  );
}

function FeeTierPanelContent({
  address,
  view,
  isLoading = false,
  error = null,
}: PortfolioFeeTierPanelProps & { isLoading?: boolean; error?: Error | null }) {
  return (
    <div className="space-y-4 md:space-y-6">
      <SectionCard title="Fee tier">
        <div className="rounded-[var(--radius-md)] bg-surface-0 border border-white/[0.04] p-5 space-y-3">
          <div className="flex items-center gap-3">
            <div className="h-10 w-10 rounded-[var(--radius-md)] bg-[var(--brand-blue)]/15 text-[var(--brand-blue)] flex items-center justify-center">
              <Percent className="h-5 w-5" />
            </div>
            <div>
              <div className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
                Current tier
              </div>
              <div className="text-base font-semibold text-foreground">
                {view
                  ? `Tier ${view.tier} · Taker ${view.takerBps} bps / Maker ${view.makerBps} bps`
                  : error
                    ? "Unable to load fee tier"
                    : isLoading
                      ? "Loading fee tier…"
                      : "Connect your wallet to view your fee tier"}
              </div>
            </div>
          </div>
        </div>
      </SectionCard>

      {address ? <FeesBreakdownCard userAddress={address} /> : null}
    </div>
  );
}
