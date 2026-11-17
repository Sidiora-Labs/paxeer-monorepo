import React, { FC } from "react";
import { Grid, useScreen } from "@paxeer-network/ui";
import { MobileOverview } from "./mobile";
import { MobileOverviewProps } from "./mobile/mobile.widget";
import { OverviewProvider } from "./provider/overviewProvider";

import { AssetWidget } from "./assets";

import { AssetsChartWidget } from "./assetChart";

import { PerformanceWidget } from "./performance";

import { HistoryDataGroupWidget } from "./historyDataGroup";

export type OverviewPageProps = MobileOverviewProps;

export const OverviewPage: FC<OverviewPageProps> = (props) => {
  const { isMobile } = useScreen();
  return (
    <OverviewProvider>
      {isMobile ? (
        <MobileOverview
          hideAffiliateCard={props.hideAffiliateCard}
          hideTraderCard={props.hideTraderCard}
        />
      ) : (
        <Grid cols={2} gap={4}>
          <React.Suspense fallback={null}>
            <AssetWidget />
          </React.Suspense>
          <React.Suspense fallback={null}>
            <AssetsChartWidget />
          </React.Suspense>
          <Grid.span colSpan={2}>
            <React.Suspense fallback={null}>
              <PerformanceWidget />
            </React.Suspense>
          </Grid.span>
          <Grid.span colSpan={2}>
            <React.Suspense fallback={null}>
              <HistoryDataGroupWidget />
            </React.Suspense>
          </Grid.span>
        </Grid>
      )}
    </OverviewProvider>
  );
};
