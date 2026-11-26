import { useState, useEffect, useLayoutEffect } from "react";
import { formatUnits } from "viem";
import { useAccount as useWagmiAccount } from "@paxeer-network/paxeer-backend/walletRuntime";
import { Decimal } from "@paxeer-network/utils";
import { Account, SimpleDI } from "@paxeer-network/core";
import {
  getNativeSnapshot,
  updateNativeSnapshot,
} from "@paxeer-network/paxeer-backend/runtime";
import {
  nativeSymbols,
  nativeTokens,
  nativeChains,
  nativeMarketRows,
} from "@paxeer-network/paxeer-backend/nativeMetadata";
import {
  nativeAccountInfo,
  nativePositionRows,
} from "@paxeer-network/paxeer-backend/nativeClient";
import { COLLATERAL_TOKENS } from "@paxeer-network/paxeer-backend/core/perps/config";
import { useMarketsSnapshot } from "@paxeer-network/paxeer-backend/hooks/perps/useMarketsStream";
import { usePerpsMarkets } from "@paxeer-network/paxeer-backend/hooks/perps/usePerpsMarkets";
import { useFeeTier } from "@paxeer-network/paxeer-backend/hooks/perps/useFeeTier";
import { useMarginMode } from "@paxeer-network/paxeer-backend/hooks/perps/useMarginMode";
import { usePortfolioStats } from "@paxeer-network/paxeer-backend/hooks/perps/usePortfolioStats";
import { useDailyPnl } from "@paxeer-network/paxeer-backend/hooks/perps/useDailyPnl";
import { getWalletBalance } from "@paxeer-network/paxeer-backend/hooks/perps/useVault";
import { activePoll } from "@paxeer-network/paxeer-backend/core/net/realtime";
import { useAppStore } from "../paxeer-network/appStore";
import { useSymbolStore } from "../provider/store/symbolStore";
import { useMarkPriceStore } from "../paxeer-network/useMarkPrice/useMarkPriceStore";
import {
  usePositionStore,
  POSITION_EMPTY,
} from "../paxeer-network/usePositionStream/usePosition.store";
import { getPrecisionByNumber } from "@paxeer-network/utils";
import { useMainnetChainsStore } from "../provider/store/chainInfoMainStore";
import { useTestnetChainsStore } from "../provider/store/chainInfoTestStore";
import { useMainTokenStore } from "../provider/store/mainTokenStore";
import { useTestTokenStore } from "../provider/store/testTokenStore";
import { useMarketStore } from "../paxeer-network/useMarket/market.store";
const EMPTY_CATALOG: ReturnType<typeof usePerpsMarkets>["catalog"] = [];

export function NativeRuntime() {
  const wallet = useWagmiAccount();
  const { catalog: loadedCatalog } = usePerpsMarkets();
  const catalog = loadedCatalog.length ? loadedCatalog : EMPTY_CATALOG;
  const feedMarkets = useMarketsSnapshot();
  const [chainMarkets, setChainMarkets] = useState<any[]>([]);
  const [quotesPending, setQuotesPending] = useState(true);
  const markets = feedMarkets.length ? feedMarkets : chainMarkets;
  const margin = useMarginMode(wallet.address);
  const feeTier = useFeeTier(wallet.address);
  const portfolio = usePortfolioStats();
  const daily = useDailyPnl(wallet.address);
  useLayoutEffect(() => {
    if (feedMarkets.length) return;
    let active = true;
    let stop = () => {};
    const timer = setTimeout(() => {
      stop = activePoll(
        async () => {
          try {
            try {
              const { readAuthenticatedFeedMarkets } =
                await import("@paxeer-network/paxeer-backend/nativeFeedMarkets");
              const rows = await readAuthenticatedFeedMarkets();
              if (rows.length) {
                if (active) setChainMarkets(rows);
                return;
              }
            } catch {
              /* Native quotes remain an independent fallback. */
            }
            const { readNativeChainMarkets } =
              await import("@paxeer-network/paxeer-backend/nativeChainMarkets");
            const rows = await readNativeChainMarkets();
            if (active) setChainMarkets(rows);
          } finally {
            if (active) setQuotesPending(false);
          }
        },
        10000,
        "now",
      );
    }, 500);
    return () => {
      active = false;
      clearTimeout(timer);
      stop();
    };
  }, [feedMarkets.length]);
  useLayoutEffect(() => {
    updateNativeSnapshot({ marketCatalog: catalog });
    const symbols = nativeSymbols();
    useMainnetChainsStore.setState({
      data: nativeChains() as any,
      dataOrigin: "broker",
    });
    useTestnetChainsStore.setState({ data: [] });
    useMainTokenStore.setState({ data: nativeTokens() as any });
    useTestTokenStore.setState({ data: [] });
    useSymbolStore.setState({ data: symbols as any });
    const information = Object.fromEntries(
      symbols.map((symbol) => [
        symbol.symbol,
        {
          ...symbol,
          base: symbol.symbol.split("_")[1],
          quote: "USDC",
          type: "PERP",
          name: symbol.symbol,
          base_dp: getPrecisionByNumber(symbol.base_tick),
          quote_dp: symbol.quote_dp,
        },
      ]),
    );
    useAppStore.getState().actions.setSymbolsInfo(information as any);
    useAppStore.getState().actions.setTokensInfo(nativeTokens() as any);
    useAppStore.getState().actions.updateAppStatus("ready", true);
  }, [catalog]);
  useLayoutEffect(() => {
    updateNativeSnapshot({
      address: wallet.address ?? null,
      chainId: wallet.chainId ?? null,
      connected: wallet.isConnected,
      marginMode: margin.data
        ? margin.data.marginMode === 0
          ? "CROSS"
          : "ISOLATED"
        : undefined,
      feeTier: feeTier.view,
      markets,
      positions: portfolio.positions.filter(
        (row) =>
          row.userAddress?.toLowerCase() === wallet.address?.toLowerCase(),
      ),
      totalVaultUsd: portfolio.isLoading ? null : portfolio.totalVaultUsd,
      freeCollateral: portfolio.isAccountLoading
        ? null
        : portfolio.freeCollateral,
      unrealizedPnl: portfolio.isLoading ? null : portfolio.unrealizedPnl,
      dailyPnl: daily.isLoading ? undefined : daily.dailyPnl,
    });
    useMarketStore
      .getState()
      .actions.updateMarket(
        nativeMarketRows() as any,
        quotesPending && feedMarkets.length === 0,
      );
    const state = getNativeSnapshot();
    const app = useAppStore.getState().actions;
    if (wallet.isConnected) {
      app.setAccountInfo(nativeAccountInfo() as any);
      app.batchUpdateForPortfolio({
        totalCollateral: new Decimal(state.totalVaultUsd ?? 0),
        freeCollateral: new Decimal(state.freeCollateral ?? 0),
        freeCollateralUSDCOnly: new Decimal(state.freeCollateral ?? 0),
        totalValue:
          state.totalVaultUsd === null
            ? null
            : new Decimal(state.totalVaultUsd),
        availableBalance: state.freeCollateral ?? 0,
        unsettledPnL: state.unrealizedPnl ?? 0,
      } as any);
      const rows = nativePositionRows();
      usePositionStore.getState().actions.setPositions("all", {
        ...POSITION_EMPTY,
        rows,
        free_collateral: state.freeCollateral ?? 0,
        total_collateral_value: state.totalVaultUsd ?? 0,
      } as any);
      for (const symbol of new Set(rows.map((row) => row.symbol!))) {
        usePositionStore.getState().actions.setPositions(symbol, {
          ...POSITION_EMPTY,
          rows: rows.filter((row) => row.symbol === symbol),
        } as any);
      }
    } else {
      app.cleanAll();
      usePositionStore.getState().actions.clearAll();
    }
    useMarkPriceStore
      .getState()
      .actions.updateMarkPrice(
        Object.fromEntries(
          markets
            .filter((row) => row.price > 0)
            .map((row) => [`PERP_${row.symbol}_USDC`, row.price]),
        ),
      );
  }, [
    catalog,
    margin.data,
    feeTier.view,
    wallet.address,
    wallet.chainId,
    wallet.isConnected,
    markets,
    quotesPending,
    feedMarkets.length,
    portfolio.positions,
    portfolio.totalVaultUsd,
    portfolio.freeCollateral,
    portfolio.unrealizedPnl,
    portfolio.isLoading,
    portfolio.isAccountLoading,
    daily.dailyPnl,
    daily.isLoading,
  ]);
  useEffect(() => {
    if (!wallet.address) return;
    let cancelled = false;
    const stop = activePoll(async () => {
      const results = await Promise.allSettled(
        Object.values(COLLATERAL_TOKENS).map(
          async (token) =>
            [
              token.symbol,
              formatUnits(
                await getWalletBalance(token.address, wallet.address!),
                token.decimals,
              ),
            ] as const,
        ),
      );
      if (cancelled) return;
      updateNativeSnapshot({
        walletBalances: Object.fromEntries(
          results.map((result, index) =>
            result.status === "fulfilled"
              ? result.value
              : [Object.values(COLLATERAL_TOKENS)[index].symbol, "0"],
          ),
        ),
      });
    }, 15000);
    return () => {
      cancelled = true;
      stop();
    };
  }, [wallet.address]);
  return null;
}
