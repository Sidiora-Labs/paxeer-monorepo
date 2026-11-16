import surface from "../../../../surface.module.css";
import { FC, useCallback } from "react";
import { useConfig } from "@paxeer-network/hooks";
import {
  cn,
  DataTable,
  Flex,
  Spinner,
  TanstackColumn,
  useScreen,
} from "@paxeer-network/ui";
import { type CampaignRankingData } from "../campaignRanking/campaignRanking.script";
import {
  type GeneralRankingData,
  GeneralRankingScriptReturn,
} from "../generalRanking/generalRanking.script";
import { type RankingColumnFields, useRankingColumns } from "./column";
import { getCurrentAddressRowKey } from "./util";

type RankingData = GeneralRankingData | CampaignRankingData;

export type RankingProps = {
  style?: React.CSSProperties;
  className?: string;
  fields: RankingColumnFields[];
} & Omit<GeneralRankingScriptReturn, "dataList" | "dataSource"> & {
    dataList: RankingData[];
    dataSource: RankingData[];
    type?: "general" | "campaign";
  };

export const Ranking: FC<RankingProps> = (props) => {
  const column = useRankingColumns(
    props.fields,
    props.address,
    typeof props.onSort === "function",
    props.type,
  );
  const { isMobile } = useScreen();

  const onRow = useCallback(
    (record: RankingData, index: number) => {
      const isYou = record.key === getCurrentAddressRowKey(props.address!);
      const isFirst = record.rank === 1;
      const isSecond = record.rank === 2;
      const isThird = record.rank === 3;

      const showBg = isFirst || isSecond || isThird;

      return {
        className: cn(
          "oui-h-10 md:oui-h-12",
          // use oui-relative to let the background image position based on row
          "oui-relative",
          isYou
            ? // add 4px extra height to make row has 2px space
              "oui-h-10 md:oui-h-12"
            : cn(
                showBg && "oui-border-b-2 oui-border-b-transparent",
                isFirst && "oui-bg-warning/10",
                isSecond && "oui-bg-base-contrast/10",
                isThird && "oui-bg-warning/10",
              ),
        ),
      };
    },
    [props.address],
  );

  const onCell = useCallback(
    (
      column: TanstackColumn<RankingData>,
      record: RankingData,
      index: number,
    ) => {
      const isFirstColumn = column.getIsFirstColumn();
      const isLastColumn = column.getIsLastColumn();
      const isRank = [1, 2, 3].includes(record.rank as number);
      if (record.key === getCurrentAddressRowKey(props.address!)) {
        return {
          className: cn(
            "after:oui-absolute after:oui-h-10 after:oui-w-full md:after:oui-h-12",
            "after:oui-border-primary",
            "after:oui-start-0 after:oui-top-0 after:oui-z-[-1]",
            "after:oui-border-y",
            isFirstColumn && "after:oui-rounded-s-lg after:oui-border-s",
            isLastColumn && "after:oui-rounded-e-lg after:oui-border-e",
          ),
        };
      }
      return {
        className: cn(
          isFirstColumn &&
            isRank &&
            "oui-rounded-s-lg oui-mix-blend-luminosity",
          isLastColumn && isRank && "oui-rounded-e-lg oui-mix-blend-luminosity",
        ),
        // style: isRank
        //   ? {
        //       position: "unset",
        //     }
        //   : undefined,
      };
    },
    [props.address],
  );

  if (isMobile) {
    return (
      <>
        <div
          role="region"
          aria-label="Trading rankings"
          tabIndex={0}
          className={surface.scrollRegion}
        >
          <DataTable
            classNames={{
              root: "oui-trading-leaderboard-ranking-table",
              body: "oui-text-2xs",
              scroll: "oui-overflow-y-hidden oui-h-full",
            }}
            loading={props.isLoading}
            columns={column}
            bordered
            initialSort={props.initialSort}
            onSort={props.onSort}
            dataSource={props.dataList}
            generatedRowKey={(record: RankingData) =>
              record.key || record.address
            }
            manualPagination
            manualSorting
            onRow={onRow}
            onCell={onCell}
          />
        </div>
        <div
          ref={props.sentinelRef}
          className="oui-invisible oui-relative oui-top-[calc(var(--oui-space-24)*3*-1)] oui-h-px"
        />
        {props.isLoading && props.dataList.length > 0 && (
          <Flex
            itemAlign="center"
            justify="center"
            width="100%"
            height={"var(--oui-space-10)"}
          >
            <Spinner size="sm" />
          </Flex>
        )}
      </>
    );
  }

  return (
    <div
      role="region"
      aria-label="Trading rankings"
      tabIndex={0}
      className={surface.scrollRegion}
    >
      <DataTable
        loading={props.isLoading}
        columns={column}
        initialSort={props.initialSort}
        onSort={props.onSort}
        bordered
        dataSource={props.dataSource}
        generatedRowKey={(record: RankingData) => record.key || record.address}
        // manualPagination
        manualSorting
        // pagination={props.pagination}
        classNames={{
          root: cn("oui-trading-leaderboard-ranking-table", "!oui-h-auto"),
          scroll:
            "oui-min-h-[calc(var(--oui-space-24)*6)] oui-max-h-[calc(var(--oui-space-24)*13)]",
        }}
        onRow={onRow}
        onCell={onCell}
      />
    </div>
  );
};
