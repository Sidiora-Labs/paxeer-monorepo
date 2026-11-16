import { withMarketTableControls } from "../shared/tableColumns";
import styles from "../../presentation.module.css";
import { FC } from "react";
import { cn, DataTable } from "@paxeer-network/ui";
import { useMarketsContext } from "../marketsProvider";
import { LowVolumeEmptyView } from "../shared/lowVolumeEmptyView";
import { useMarketsListFullColumns } from "./column";
import {
  type MarketsListFullType,
  type UseMarketsListFullReturn,
} from "./marketsListFull.script";

export type MarketsListFullProps = UseMarketsListFullReturn & {
  type?: MarketsListFullType;
};

export const MarketsListFull: FC<MarketsListFullProps> = (props) => {
  const {
    loading,
    dataSource,
    isHiddenByLowVolumeFilter,
    favorite,
    onSort,
    initialSort,
    type,
    pagination,
  } = props;

  const { symbol, onSymbolChange } = useMarketsContext();

  const columns = useMarketsListFullColumns(favorite, false);

  return (
    <DataTable
      className={`${styles.surface} ${styles.table}`}
      bordered
      columns={withMarketTableControls(columns)}
      loading={loading}
      dataSource={dataSource}
      onRow={(record, index) => {
        return {
          tabIndex: 0,
          onKeyDown: (event: React.KeyboardEvent<HTMLTableRowElement>) => {
            if (
              event.target === event.currentTarget &&
              (event.key === "Enter" || event.key === " ")
            ) {
              event.preventDefault();
              event.currentTarget.click();
            }
          },
          className: cn("oui-h-14 oui-cursor-pointer"),
          onClick: () => {
            onSymbolChange?.(record);
            favorite.addToHistory(record);
          },
          "data-testid": `oui-testid-markets-${
            type === "new"
              ? "newListing"
              : type === "rwa"
                ? "rwa"
                : type === "preTge"
                  ? "preTge"
                  : "all"
          }-tr-${record.symbol}`,
        };
      }}
      generatedRowKey={(record) => record.symbol}
      rowSelection={{ [symbol!]: true }}
      onSort={onSort}
      initialSort={initialSort}
      pagination={pagination}
      classNames={{
        header: "oui-h-12",
        empty: isHiddenByLowVolumeFilter
          ? "oui-items-start oui-pt-16"
          : undefined,
      }}
      manualSorting
      emptyView={isHiddenByLowVolumeFilter ? <LowVolumeEmptyView /> : undefined}
    />
  );
};
