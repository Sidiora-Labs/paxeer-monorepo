import { useAccount } from "@paxeer-network/hooks";
import { useAppContext, useDataTap } from "@paxeer-network/react-app";
import { EMPTY_LIST, AccountStatusEnum } from "@paxeer-network/types";
import { useOverviewContext } from "../provider/overviewContext";

export const useAssetsChartScript = () => {
  const assetHistory = useOverviewContext();

  const { wrongNetwork, disabledConnect } = useAppContext();
  const { state } = useAccount();

  const filteredData = useDataTap(assetHistory.data, {
    accountStatus:
      state.status === AccountStatusEnum.EnableTradingWithoutConnected
        ? AccountStatusEnum.EnableTradingWithoutConnected
        : AccountStatusEnum.EnableTrading,
    fallbackData: EMPTY_LIST,
  });

  const invisible = wrongNetwork || disabledConnect || !filteredData?.length;

  return {
    ...assetHistory,
    data: filteredData ?? EMPTY_LIST,
    invisible,
  } as const;
};

export type useAssetsChartScriptReturn = ReturnType<
  typeof useAssetsChartScript
>;
