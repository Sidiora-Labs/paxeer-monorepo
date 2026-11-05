import Link from "next/link";

import { Badge } from "../../kit/collection";

import { GatewayRpcError, GatewayUnavailableError, surfaceCapability } from "../../api/gateway";
import {
  launchpadConfig,
  launchpadMarketCount,
  launchpadMarkets,
  type LaunchpadConfig,
  type LaunchpadMarket,
  type LaunchpadMarketRow,
} from "../../api/precompiles";
import { ExplorerPanel, ExplorerTable } from "../../kit/explorer";
import { LabelValue } from "../../kit/money";
import { StateEmpty } from "../../kit/surface";
import { ActivityFeed, activitySide } from "../_markets/activity-feed";
import { SurfaceCapabilityGate } from "../_markets/capability";
import {
  LAUNCHPAD_DECIMALS,
  LAUNCHPAD_PRICE_DECIMALS,
  firstParam,
  formatUnits,
  isEvmAddress,
  shortId,
  type MarketSearchParams,
} from "../_markets/format";
import { MarketFrame, MarketUnavailable } from "../_markets/frame";
import { LaunchpadActions } from "./launchpad-client";

export const dynamic = "force-dynamic";

const PAGE_SIZE = 50n;
const FEE_STRATEGIES = ["Claim", "Burn", "Airdrop", "LP rewards"] as const;

function marketPageHref(parameters: MarketSearchParams, page: bigint): string {
  const query = new URLSearchParams();
  for (const [name, value] of Object.entries(parameters)) {
    if (name === "marketPage" || value === undefined) continue;
    for (const item of typeof value === "string" ? [value] : value) {
      query.append(name, item);
    }
  }
  query.set("marketPage", page.toString());
  return `/launchpad?${query.toString()}`;
}

export default async function LaunchpadPage({ searchParams }: Readonly<{ searchParams: Promise<MarketSearchParams> }>) {
  const parameters = await searchParams;
  const candidate = firstParam(parameters.account)?.toLowerCase();
  const account = isEvmAddress(candidate) ? candidate : undefined;
  const capability = await surfaceCapability("launchpad");
  const feed = (
    <ActivityFeed
      route="/launchpad"
      account={account}
      side={activitySide(firstParam(parameters.side))}
      cursor={firstParam(parameters.cursor)}
      kind={firstParam(parameters.kind)}
    />
  );
  if (!capability.live) {
    return (
      <MarketFrame
        title="Launchpad"
        description="Bonding-curve markets on the Launchpad precompile. Quotes come from the curve before your wallet opens."
        account={account}
      >
        <SurfaceCapabilityGate surface="launchpad" live={false} detail={capability.detail} />
        {feed}
      </MarketFrame>
    );
  }
  let config: LaunchpadConfig | undefined;
  let count = 0n;
  let marketPage = 1n;
  let pageCount = 1n;
  let markets: readonly LaunchpadMarketRow[] = [];
  let problem: string | undefined;
  try {
    [config, count] = await Promise.all([
      launchpadConfig(),
      launchpadMarketCount(),
    ]);
    pageCount = count === 0n ? 1n : (count + PAGE_SIZE - 1n) / PAGE_SIZE;
    const pageParameter = firstParam(parameters.marketPage);
    const requestedPage = pageParameter !== undefined && /^[1-9][0-9]{0,77}$/u.test(pageParameter)
      ? BigInt(pageParameter)
      : 1n;
    marketPage = requestedPage > pageCount ? pageCount : requestedPage;
    markets = await launchpadMarkets((marketPage - 1n) * PAGE_SIZE, PAGE_SIZE);
  } catch (error) {
    if (!(error instanceof GatewayUnavailableError || error instanceof GatewayRpcError)) {
      throw error;
    }
    problem = error.message;
  }
  return (
    <MarketFrame
      title="Launchpad"
      description="Bonding-curve markets on the Launchpad precompile. Quotes come from the curve before your wallet opens."
      account={account}
    >
      <SurfaceCapabilityGate surface="launchpad" live detail={null} />
      {problem === undefined ? null : <MarketUnavailable detail={problem} />}
      {config === undefined ? null : (
        <div className="flex flex-wrap gap-6">
          <LabelValue label="Quote asset" value={config.quoteSymbol} />
          <LabelValue label="Markets" value={count.toString()} />
          <LabelValue label="Creation fee" value={`${formatUnits(config.creationFee, LAUNCHPAD_DECIMALS)} ${config.quoteSymbol}`} />
          <LabelValue
            label="Swap fee range"
            value={`${config.minFeeBps.toString()}–${config.maxFeeBps.toString()} bps (base ${config.baseFeeBps.toString()})`}
          />
        </div>
      )}
      <ExplorerPanel title="Markets">
        {markets.length === 0 ? (
          <StateEmpty title="No markets yet" description="Create the first market below." />
        ) : (
          <ExplorerTable
            caption="Launchpad markets"
            columns={["Market", "Token", "Price", "Supply", "Reserve", "Volume", "Fee strategy", "State"]}
            rows={markets.map((market) => "unavailable" in market
              ? {
                  id: market.pool,
                  cells: [
                    <span key="pool" title={market.unavailable}>{market.pool}</span>,
                    "—", "—", "—", "—", "—", "—",
                    <Badge key="state" variant="warning">Unavailable</Badge>,
                  ],
                }
              : {
                  id: market.token,
                  cells: [
                    `${market.name} (${market.symbol})`,
                    shortId(market.token),
                    formatUnits(market.price, LAUNCHPAD_PRICE_DECIMALS),
                    formatUnits(market.totalSupply, LAUNCHPAD_DECIMALS),
                    formatUnits(market.tokenReserve, LAUNCHPAD_DECIMALS),
                    formatUnits(market.cumulativeVolume, LAUNCHPAD_DECIMALS),
                    FEE_STRATEGIES[Number(market.feeStrategy)] ?? market.feeStrategy.toString(),
                    <Badge key="state" variant={market.paused ? "warning" : "success"}>
                      {market.paused ? "Paused" : "Trading"}
                    </Badge>,
                  ],
                })}
          />
        )}
        {problem !== undefined || count === 0n ? null : (
          <nav aria-label="Market pages" className="flex items-center gap-4">
            {marketPage === 1n ? null : (
              <Link href={marketPageHref(parameters, marketPage - 1n)} prefetch={false}>Previous markets</Link>
            )}
            <span aria-live="polite">Page {marketPage.toString()} of {pageCount.toString()}</span>
            {marketPage === pageCount ? null : (
              <Link href={marketPageHref(parameters, marketPage + 1n)} prefetch={false}>Next markets</Link>
            )}
          </nav>
        )}
      </ExplorerPanel>
      <LaunchpadActions
        account={account}
        quoteSymbol={config?.quoteSymbol ?? "quote"}
        markets={markets.filter((market): market is LaunchpadMarket => !("unavailable" in market)).map((market) => ({
          token: market.token,
          pool: market.pool,
          label: `${market.name} (${market.symbol})`,
          paused: market.paused,
          currentFeeBps: market.currentFeeBps,
        }))}
        feeStrategies={FEE_STRATEGIES}
      />
      {feed}
    </MarketFrame>
  );
}
