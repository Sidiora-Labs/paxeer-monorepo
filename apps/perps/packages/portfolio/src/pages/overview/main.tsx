import styles from "../../presentation.module.css";
import React, { FC } from "react";
import { Grid, useScreen } from "@paxeer-network/ui";
import { MobileOverview } from "./mobile";
import { MobileOverviewProps } from "./mobile/mobile.widget";
import { OverviewProvider } from "./provider/overviewProvider";

const LazyAssetWidget = React.lazy(() =>
  import("./assets").then((mod) => {
    return { default: mod.AssetWidget };
  }),
);

const LazyAssetsChartWidget = React.lazy(() =>
  import("./assetChart").then((mod) => {
    return { default: mod.AssetsChartWidget };
  }),
);

const LazyPerformanceWidget = React.lazy(() =>
  import("./performance").then((mod) => {
    return { default: mod.PerformanceWidget };
  }),
);

const LazyHistoryDataGroupWidget = React.lazy(() =>
  import("./historyDataGroup").then((mod) => {
    return { default: mod.HistoryDataGroupWidget };
  }),
);

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
        <div className={styles.overview}>
          <React.Suspense fallback={null}>
            <LazyAssetWidget />
          </React.Suspense>
          <React.Suspense fallback={null}>
            <LazyAssetsChartWidget />
          </React.Suspense>
          <div className={styles.fullWidth}>
            <React.Suspense fallback={null}>
              <LazyPerformanceWidget />
            </React.Suspense>
          </div>
          <div className={styles.fullWidth}>
            <React.Suspense fallback={null}>
              <LazyHistoryDataGroupWidget />
            </React.Suspense>
          </div>
        </div>
      )}
    </OverviewProvider>
  );
};
