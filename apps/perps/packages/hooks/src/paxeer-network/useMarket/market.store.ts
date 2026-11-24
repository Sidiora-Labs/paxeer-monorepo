import { create } from "zustand";
import { API } from "@paxeer-network/types";

type MarketStore = {
  market: API.MarketInfoExt[];
  marketMap: Record<string, API.MarketInfoExt> | null;
  quotesPending: boolean;
};

type MarketActions = {
  updateMarket: (data: API.MarketInfoExt[], quotesPending?: boolean) => void;
};

export const useMarketStore = create<MarketStore & { actions: MarketActions }>(
  (set, get) => ({
    market: [],
    marketMap: null,
    quotesPending: true,
    actions: {
      updateMarket: (data, quotesPending = false) => {
        const marketMap: Record<string, API.MarketInfoExt> = {};

        data.forEach((item) => {
          marketMap[item.symbol] = item;
        });

        set({
          market: data,
          marketMap: marketMap,
          quotesPending,
        });
      },
    },
  }),
);

export const useMarketList = () => {
  return useMarketStore((state) => state.market);
};

export const useMarketMap = () => {
  return useMarketStore((state) => state.marketMap);
};
