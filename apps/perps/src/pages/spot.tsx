import React from "react";
import Router from "next/router";
import type { GetServerSidePropsContext } from "next";
import { isSpotOpen } from "@/core/nav";
import {
  DEFAULT_SPOT_SYMBOL,
  getSpotMarket,
  type SpotMarket,
} from "@/core/spot/config";
import {
  SpotMarketWatchlist,
  SpotOrderForm,
  SpotOrderbook,
  SpotPairHeader,
  SpotPositionsPanel,
  SpotTradesTape,
} from "@/widgets/spot";

export interface SpotPageProps {
  symbol: string;
}

function marketFor(symbol: string): SpotMarket {
  const market = getSpotMarket(symbol);
  if (!market) throw new Error("Unknown spot market");
  return market;
}

function selectMarket(symbol: string): void {
  void Router.push({ pathname: "/spot", query: { market: symbol } });
}

// Spot stays unavailable, including direct navigation and bookmarks,
// until the release build sets NEXT_PUBLIC_RELEASE_SPOT_OPEN to 1.
export const getServerSideProps = async ({
  query,
}: Pick<GetServerSidePropsContext, "query"> = { query: {} }): Promise<
  { notFound: true } | { props: SpotPageProps }
> => {
  if (!isSpotOpen()) return { notFound: true };
  const requested = query.market === undefined ? DEFAULT_SPOT_SYMBOL : query.market;
  if (typeof requested !== "string") return { notFound: true };
  const market = getSpotMarket(requested);
  return market ? { props: { symbol: market.symbol } } : { notFound: true };
};

export function SpotPage({ symbol }: SpotPageProps) {
  const market = marketFor(symbol);
  return (
    <div className="flex flex-col min-h-full bg-background text-foreground">
      <SpotPairHeader selectedPair={market.symbol} onPairChange={selectMarket} />
      <main className="flex-1 min-h-0 grid grid-cols-1 xl:grid-cols-[280px_minmax(0,1fr)_320px] gap-2 p-2">
        <aside className="min-h-0 flex flex-col gap-2">
          <SpotMarketWatchlist selectedSymbol={market.symbol} onSelect={selectMarket} />
        </aside>
        <section className="min-h-0 min-w-0 grid grid-cols-1 md:grid-cols-2 gap-2">
          <SpotOrderbook symbol={market.symbol} />
          <SpotTradesTape symbol={market.symbol} />
        </section>
        <aside className="min-h-0">
          <SpotOrderForm market={market} />
        </aside>
      </main>
      <section className="p-2">
        <SpotPositionsPanel />
      </section>
    </div>
  );
}

export default SpotPage;
