/* eslint-disable @typescript-eslint/no-explicit-any */
import React from "react";
import { AssetAreaChart } from "@paxeer-network/chart";
import { useTranslation } from "@paxeer-network/i18n";
import { Card, EmptyDataState } from "@paxeer-network/ui";
import { useAssetsChartScriptReturn } from "./assetsChart.script";

import { PeriodTitle } from "../shared/periodHeader";

export type AssetsLineChartProps = {} & useAssetsChartScriptReturn;

export const AssetsChart: React.FC<AssetsLineChartProps> = (props) => {
  const { onPeriodChange, data, periodTypes, period } = props;
  const { t } = useTranslation();
  return (
    <Card
      title={
        <React.Suspense fallback={null}>
          <PeriodTitle
            onPeriodChange={onPeriodChange}
            periodTypes={periodTypes}
            period={period}
            title={t("common.assets")}
          />
        </React.Suspense>
      }
      id="portfolio-overview-assets-chart"
      classNames={{ content: "oui-h-[168px] oui-pb-0" }}
    >
      <React.Suspense fallback={null}>
        {props.invisible || !data.length ? (
          <EmptyDataState className="oui-h-full oui-justify-center" />
        ) : (
          <AssetAreaChart data={data as any} />
        )}
      </React.Suspense>
    </Card>
  );
};
