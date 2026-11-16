import { withMarketTableControls } from "../shared/tableColumns";
import styles from "../../presentation.module.css";
import type { FC } from "react";
import { cn } from "@paxeer-network/ui";
import { DataTable } from "@paxeer-network/ui";
import { useFundingColumns } from "./columns";
import { FundingComparisonReturn } from "./fundingComparison.script";

export const FundingComparison: FC<FundingComparisonReturn> = (props) => {
  const columns = useFundingColumns();

  return (
    <DataTable
      className={`${styles.surface} ${styles.table}`}
      columns={withMarketTableControls(columns)}
      dataSource={props.dataSource}
      loading={props.isLoading}
      onRow={() => {
        return {
          className: cn("oui-h-12 oui-cursor-pointer"),
        };
      }}
      classNames={{
        header: "oui-h-12",
      }}
      bordered
      pagination={props.pagination}
    />
  );
};
