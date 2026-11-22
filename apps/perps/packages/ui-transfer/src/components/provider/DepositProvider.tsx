import React, { PropsWithChildren, createContext, useMemo } from "react";
import type { API } from "@paxeer-network/types";

export interface DepositContextState {
  collateralUnit?: API.TokenInfo;
}

export const DepositContext = createContext<DepositContextState>({});

export const DepositProvider: React.FC<
  PropsWithChildren<DepositContextState>
> = (props) => {
  const value = useMemo(
    () => ({ collateralUnit: props.collateralUnit }),
    [props.collateralUnit],
  );
  return (
    <DepositContext.Provider value={value}>
      {props.children}
    </DepositContext.Provider>
  );
};
