import React from "react";
import {
  type PanelRegistry,
  TRADING_PANEL_IDS,
} from "@paxeer-network/layout-core";
import {
  SideMarketsWidget,
  SymbolInfoBarFullWidget,
  HorizontalMarketsWidget,
} from "@paxeer-network/markets";
import { cn } from "@paxeer-network/ui";
import { OrderEntryWidget } from "@paxeer-network/ui-order-entry";
import type { SharePnLConfig } from "@paxeer-network/ui-share";
import { TradingviewWidget } from "@paxeer-network/ui-tradingview";
import { DepositStatusWidget } from "@paxeer-network/ui-transfer";
import type { LayoutPosition } from "./switchLayout";

/** Re-export for consumers that import from trading package */
export { TRADING_PANEL_IDS };
export type { TradingPanelId } from "@paxeer-network/layout-core";

import { DataListWidget } from "../dataList";
import { OrderBookAndTradesWidget } from "../orderBookAndTrades";
import { RiskRateWidget } from "../riskRate";
import { AssetViewWidget } from "../assetView";

/**
 * Props required to build the trading panel registry.
 * Extends trading state with computed values from the desktop layout.
 */
export interface TradingPanelRegistryProps {
  symbol: string;
  /** Called when symbol changes; registry passes through to widgets */
  onSymbolChange: (symbol: unknown) => void;
  /** TradingView config; registry uses library_path and spreads rest to widget */
  tradingViewConfig?: unknown;
  disableFeatures?: unknown;
  navigateToPortfolio?: () => void;
  sharePnLConfig?: SharePnLConfig;
  isFirstTimeDeposit?: boolean;
  resizeable?: boolean;
  /** Optional when layout is rule-driven; defaults used for SwitchLayout when undefined. */
  layout?: LayoutPosition;
  onLayout?: (layout: LayoutPosition) => void;
  marketLayout?: "left" | "top" | "bottom" | "hide";
  onMarketLayout?: (layout: "left" | "top" | "bottom" | "hide") => void;
  /** Computed: symbol bar height (e.g. 104 with countdown, 56 without) */
  symbolInfoBarHeight: number;
  showCountdown: boolean;
  closeCountdown: () => void;
  /** TradingView fullscreen from localStorage */
  tradingViewFullScreen: boolean;
  animating?: boolean;
  setAnimating?: (v: boolean) => void;
  dataListHeight?: {
    height: number;
    minHeight: number;
    maxHeight: number;
  };
}

/**
 * Creates a PanelRegistry (Map<panelId, ReactNode>) for the trading desktop layout.
 * Call from DesktopLayout with full state + computed values; use result as LayoutHost panels.
 */
export function createTradingPanelRegistry(
  props: TradingPanelRegistryProps,
  // t: (key: string) => string,
  languageMap: Record<string, string>,
): PanelRegistry {
  const {
    symbol,
    onSymbolChange,
    tradingViewConfig,
    showCountdown,
    closeCountdown,
    tradingViewFullScreen,
    resizeable,
    sharePnLConfig,
    animating,
    setAnimating,
    symbolInfoBarHeight,
  } = props;

  const config = (tradingViewConfig ?? {}) as {
    library_path?: string;
    [key: string]: unknown;
  };
  const { library_path, ...restTradingViewConfig } = config;

  const panels = new Map<
    string,
    { node: React.ReactNode; props?: Record<string, unknown> }
  >();

  // symbolInfoBar — min-height reserves baseline; omit fixed height; flex column so full-width bar stacks predictably.
  panels.set(TRADING_PANEL_IDS.SYMBOL_INFO_BAR, {
    node: (
      <div
        style={{
          display: "flex",
          flexDirection: "column",
          minHeight: symbolInfoBarHeight,
          height: symbolInfoBarHeight,
          width: "100%",
        }}
      >
        <SymbolInfoBarFullWidget
          symbol={symbol}
          onSymbolChange={onSymbolChange}
          closeCountdown={closeCountdown}
          showCountdown={showCountdown}
        />
      </div>
    ),
    props: {},
  });

  // tradingView (chart)
  const tradingviewWidget = (
    <TradingviewWidget
      classNames={{
        root: cn(
          tradingViewFullScreen
            ? "!oui-absolute oui-top-0 oui-start-0 oui-end-0 oui-bottom-0 oui-z-[40] oui-bg-base-10"
            : "oui-z-1",
        ),
        content: cn(
          tradingViewFullScreen
            ? "oui-top-3 oui-bottom-3 oui-start-3 oui-end-3 oui-bg-base-9 oui-rounded-[16px] oui-overflow-hidden"
            : "",
        ),
      }}
      symbol={symbol}
      {...restTradingViewConfig}
      libraryPath={library_path}
    />
  );
  panels.set(TRADING_PANEL_IDS.TRADING_VIEW, {
    node: tradingviewWidget,
    props: {},
  });

  // orderbook
  panels.set(TRADING_PANEL_IDS.ORDERBOOK, {
    node: (
      <React.Suspense fallback={null}>
        <OrderBookAndTradesWidget symbol={symbol} />
      </React.Suspense>
    ),
    props: {},
  });

  // dataList
  panels.set(TRADING_PANEL_IDS.DATA_LIST, {
    node: (
      <React.Suspense fallback={null}>
        <DataListWidget
          current={undefined}
          symbol={symbol}
          sharePnLConfig={sharePnLConfig}
        />
      </React.Suspense>
    ),
    props: {},
  });

  // order-entry sub panels: margin / assets / main order entry
  panels.set(TRADING_PANEL_IDS.MARGIN, {
    node: (
      <React.Suspense fallback={null}>
        <RiskRateWidget />
      </React.Suspense>
    ),
    props: {},
  });

  panels.set(TRADING_PANEL_IDS.ASSETS, {
    node: (
      <>
        <React.Suspense fallback={null}>
          <AssetViewWidget isFirstTimeDeposit={props.isFirstTimeDeposit} />
        </React.Suspense>
        <DepositStatusWidget
          className="oui-mt-3 oui-gap-y-2"
          onClick={props.navigateToPortfolio}
        />
      </>
    ),
    props: {},
  });

  panels.set(TRADING_PANEL_IDS.ORDER_ENTRY, {
    node: (
      <OrderEntryWidget
        symbol={symbol}
        disableFeatures={
          props.disableFeatures as
            | ("slippageSetting" | "feesInfo")[]
            | undefined
        }
      />
    ),
    props: {},
  });

  panels.set(TRADING_PANEL_IDS.HORIZONTAL_MARKETS, {
    node: (
      <HorizontalMarketsWidget
        symbol={props.symbol}
        onSymbolChange={props.onSymbolChange}
        maxItems={-1}
      />
    ),
    props: {},
  });

  // markets: always register; layout plugin decides whether to show (e.g. include in layout model)
  const marketsWidget = (
    <SideMarketsWidget
      // resizeable={resizeable}
      symbol={symbol}
      onSymbolChange={onSymbolChange}
    />
  );
  panels.set(TRADING_PANEL_IDS.MARKETS, {
    node: marketsWidget,
    // node: (
    //   <div onTransitionEnd={() => setAnimating?.(false)}>
    //     {!animating && marketsWidget}
    //   </div>
    // ),
    props: { title: languageMap["common.markets"] },
  });
  // t("common.markets")
  return panels;
}
