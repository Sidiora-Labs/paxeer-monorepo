import React from "react";
import { pick } from "ramda";
import { MarketsProvider, MarketsProviderProps } from "../marketsProvider";
import { useSideMarketsScript } from "./sideMarkets.script";
import type { SideMarketsProps } from "./sideMarkets.ui";

import { SideMarkets } from "./sideMarkets.ui";

export type SideMarketsWidgetProps = MarketsProviderProps &
  Partial<Pick<SideMarketsProps, "className">> & {
    panelSize?: "small" | "middle" | "large";
    collapsed?: boolean;
  };

export const SideMarketsWidget: React.FC<SideMarketsWidgetProps> = (props) => {
  const state = useSideMarketsScript(props);

  return (
    <MarketsProvider {...pick(["symbol", "onSymbolChange"], props)}>
      <React.Suspense fallback={null}>
        <SideMarkets {...state} className={props.className} />
      </React.Suspense>
    </MarketsProvider>
  );
};
