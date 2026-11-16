import { MouseEventHandler, useCallback } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { Flex, Text, cn, Column } from "@paxeer-network/ui";
import { Decimal } from "@paxeer-network/utils";
import { DeleteIcon, TopIcon } from "../../icons";
import { FavoriteInstance } from "../../type";
import {
  get24hPercentageColumn,
  getLastColumn,
  getSymbolColumn,
} from "../shared/column";

export const useDropDownMarketsColumns = () => {
  const { t } = useTranslation();

  return useCallback(
    (favorite: FavoriteInstance, isFavoriteList = false) => {
      return [
        getSymbolColumn(favorite, isFavoriteList, {
          symbolColumnWidth: 190,
        }),
        getLastColumn(),
        get24hPercentageColumn(),
        {
          title: t("markets.column.24hVol"),
          dataIndex: "24h_amount",
          align: "right",
          onSort: true,
          className: "oui-relative",

          render: (value, record) => {
            const onDelSymbol: MouseEventHandler = (e) => {
              favorite.updateSymbolFavoriteState(
                record,
                favorite.selectedFavoriteTab,
                true,
              );
              e.stopPropagation();
            };

            const iconCls =
              "oui-w-4 oui-h-4 oui-text-base-contrast-80 hover:oui-text-base-contrast";

            const actions = (
              <div className={cn("oui-absolute oui-end-1 oui-top-0.5")}>
                <Flex
                  className={cn(
                    "oui-bg-primary-darken oui-py-1.5",
                    "oui-hidden group-hover:oui-inline-flex",
                  )}
                  r="base"
                  width="var(--oui-space-12)"
                  justify="center"
                  itemAlign="end"
                  gapX={2}
                >
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      favorite.pinToTop(record);
                    }}
                    aria-label="Pin market"
                    className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
                  >
                    <TopIcon className={iconCls} />
                  </button>
                  <button
                    type="button"
                    onClick={onDelSymbol}
                    aria-label="Delete"
                    className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
                  >
                    <DeleteIcon className={iconCls} />
                  </button>
                </Flex>
              </div>
            );
            return (
              <>
                {isFavoriteList && actions}
                <Text.numeral
                  rule="human"
                  dp={2}
                  rm={Decimal.ROUND_DOWN}
                  className={cn(isFavoriteList && "group-hover:oui-invisible")}
                >
                  {value}
                </Text.numeral>
              </>
            );
          },
        },
        {
          title: t("markets.openInterest"),
          dataIndex: "openInterest",
          align: "right",
          onSort: true,

          render: (value) => (
            <Text.numeral
              rule="human"
              dp={2}
              rm={Decimal.ROUND_DOWN}
              size="2xs"
            >
              {value}
            </Text.numeral>
          ),
        },
      ] as Column[];
    },
    [t],
  );
};
