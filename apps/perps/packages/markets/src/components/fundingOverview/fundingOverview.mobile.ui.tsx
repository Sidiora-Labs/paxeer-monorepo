import { withMarketTableControls } from "../shared/tableColumns";
import styles from "../../presentation.module.css";
import { FC, useState } from "react";
import { DataTable, cn } from "@paxeer-network/ui";
import { useFundingOverviewColumns } from "./columns";
import { FundingOverviewReturn } from "./fundingOverview.script";

export type MobileFundingOverviewProps = FundingOverviewReturn;

export const MobileFundingOverview: FC<MobileFundingOverviewProps> = (
  props,
) => {
  const { dataSource, isLoading, onSort } = props;
  const [selectedPeriod, setSelectedPeriod] = useState("1dPositive");

  const columns = useFundingOverviewColumns(selectedPeriod, setSelectedPeriod);

  return (
    <DataTable
      className={`${styles.surface} ${styles.table}`}
      columns={withMarketTableControls(columns)}
      dataSource={dataSource}
      loading={isLoading}
      onRow={() => {
        return {
          className: cn("oui-h-8 oui-cursor-pointer"),
        };
      }}
      classNames={{
        header: "oui-h-9",
        body: "oui-text-2xs",
      }}
      onSort={onSort}
      manualSorting
      generatedRowKey={(record) => record.symbol}
    />
  );
};
