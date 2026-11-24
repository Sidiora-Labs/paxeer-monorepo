import { useMemo } from "react";
import { useTickerStream } from "@paxeer-network/hooks";
import { Decimal } from "@paxeer-network/utils";
import { useTradingPageContext } from "../../../provider/tradingPageContext";

export const useTradeDataScript = (props: { symbol: string }) => {
  const { symbol } = props;
  const ticker = useTickerStream(symbol);
  const { symbolInfo } = useTradingPageContext();
  const vol_24h = useMemo(() => {
    const volume = ticker?.["24h_amount"];
    if (volume != null && Number.isFinite(volume)) {
      return new Decimal(volume).toFixed(
        symbolInfo.quote_dp,
        Decimal.ROUND_DOWN,
      );
    }
    return undefined;
  }, [ticker]);

  const openInterest = useMemo(() => {
    const markPrice = ticker?.["index_price"];
    const openInterest = ticker?.["open_interest"];
    if (markPrice && openInterest != null && !isNaN(markPrice)) {
      return new Decimal(markPrice)
        .mul(Number(openInterest))
        .toFixed(symbolInfo.quote_dp, Decimal.ROUND_DOWN);
    }
    return undefined;
  }, [ticker]);
  return {
    ticker,
    symbolInfo,
    vol_24h,
    openInterest,
  };
};

export type TradeDataState = ReturnType<typeof useTradeDataScript>;
