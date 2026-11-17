import { useMemo } from "react";
import { useSubAccountQuery } from "@paxeer-network/hooks";
import { API } from "@paxeer-network/types";
import { Decimal } from "@paxeer-network/utils";

const EMPTY_ACCOUNT_VALUE: Record<string, number> = {};

export const useAccountValue = (mainAccountId?: string) => {
  const { data: positions } = useSubAccountQuery<API.PositionExt[]>(
    "/v1/client/aggregate/positions",
    {
      accountId: mainAccountId,
      errorRetryCount: 0,
      revalidateIfStale: false,
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
    },
  );

  const accountValue = useMemo(() => {
    if (!positions?.length) return EMPTY_ACCOUNT_VALUE;

    return positions.reduce<Record<string, number>>((values, position) => {
      const accountId = position.account_id!;
      values[accountId] = new Decimal(values[accountId] ?? 0)
        .plus(position.unsettled_pnl)
        .toNumber();
      return values;
    }, {});
  }, [positions]);

  return { accountValue };
};
