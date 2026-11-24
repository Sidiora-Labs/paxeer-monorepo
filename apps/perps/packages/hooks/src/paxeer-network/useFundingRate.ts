import { useEffect, useState } from "react";
import { API, SDKError } from "@paxeer-network/types";
import { getTimestamp } from "@paxeer-network/utils";
import { useQuery } from "../useQuery";
import { useMarketMap } from "./useMarket/market.store";
import { fundingRateDisplay } from "./fundingRateDisplay";

export const useFundingRate = (symbol: string) => {
  if (!symbol) {
    throw new SDKError("Symbol is required");
  }

  const [now, setNow] = useState(getTimestamp);
  const { data: queried } = useQuery<API.FundingRate>(
    `/v1/public/funding_rate/${symbol}`,
  );

  const marketMap = useMarketMap();
  const data = marketMap?.[symbol] ?? queried;
  const nextFundingTime = data?.next_funding_time;

  useEffect(() => {
    const current = getTimestamp();
    setNow(current);
    if (nextFundingTime == null || nextFundingTime <= current) return;

    const timer = setInterval(() => {
      const current = getTimestamp();
      setNow(current);
      if (current >= nextFundingTime) clearInterval(timer);
    }, 1000);
    return () => clearInterval(timer);
  }, [nextFundingTime]);

  return {
    ...data,
    ...fundingRateDisplay(data, now),
  };
};
