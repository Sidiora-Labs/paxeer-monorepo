import { MouseEventHandler, useMemo } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { Flex, Text, Box, Tooltip, Column, Badge } from "@paxeer-network/ui";
import { Decimal } from "@paxeer-network/utils";
import {
  FavoritesIcon,
  MoveToTopIcon,
  PaxeerNetworkIcon,
  UnFavoritesIcon,
} from "../../icons";
import { FavoriteInstance } from "../../type";
import { FavoritesDropdownMenuWidget } from "../favoritesDropdownMenu";
import { SymbolDisplay } from "../symbolDisplay";

export const useMarketsListFullColumns = (
  favorite: FavoriteInstance,
  isFavoriteList = false,
) => {
  const { t } = useTranslation();

  const columns = useMemo(() => {
    return [
      {
        title: <UnFavoritesIcon className="oui-mt-1" />,
        dataIndex: "isFavorite",
        align: "center",

        render: (value, record) => {
          const onDelSymbol: MouseEventHandler = (e) => {
            favorite.updateSymbolFavoriteState(
              record,
              favorite.selectedFavoriteTab,
              true,
            );
            e.stopPropagation();
          };

          const button = (
            <Flex
              asChild
              width="100%"
              height="100%"
              mr={3}
              justify="center"
              itemAlign="center"
              data-testid="oui-testid-markets-table-row-favorite-icon"
            >
              <button
                type="button"
                aria-label={t("markets.favorites")}
                onClick={onDelSymbol}
                className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
              >
                {value ? <FavoritesIcon /> : <UnFavoritesIcon />}
              </button>
            </Flex>
          );

          if (isFavoriteList) {
            return button;
          }

          return (
            <FavoritesDropdownMenuWidget row={record} favorite={favorite}>
              <span data-testid="oui-testid-markets-table-row-favorite-icon">
                {value ? <FavoritesIcon /> : <UnFavoritesIcon />}
              </span>
            </FavoritesDropdownMenuWidget>
          );
        },
      },
      {
        title: t("markets.column.market"),
        dataIndex: "symbol",

        onSort: true,
        render: (value, record) => {
          return (
            <Flex gapX={1} itemAlign="center">
              <SymbolDisplay formatString="base" showIcon record={record}>
                {value}
              </SymbolDisplay>
              {typeof record.leverage === "number" && (
                <Badge size="xs" color="primary">
                  {record.leverage}x
                </Badge>
              )}
            </Flex>
          );
        },
      },
      {
        title: t("markets.column.last"),
        dataIndex: "24h_close",

        align: "right",
        onSort: true,
        render: (value, record) => {
          return (
            <Text.numeral dp={record.quote_dp || 2} currency="$">
              {value}
            </Text.numeral>
          );
        },
      },
      {
        title: t("markets.column.24hChange"),
        dataIndex: "change",

        align: "right",
        onSort: true,
        render: (value) => {
          return (
            <Text.numeral
              rule="percentages"
              coloring
              rm={Decimal.ROUND_DOWN}
              showIdentifier
            >
              {value}
            </Text.numeral>
          );
        },
      },
      {
        title: (
          <button type="button">
            <Flex gapX={1}>
              <PaxeerNetworkIcon /> {t("markets.column.24hVolume")}
            </Flex>
          </button>
        ),
        dataIndex: "24h_amount",

        align: "right",
        onSort: true,
        render: (value) => {
          return (
            <Text.numeral currency="$" dp={0} rm={Decimal.ROUND_DOWN}>
              {value}
            </Text.numeral>
          );
        },
      },
      {
        title: (
          <button type="button">
            <Flex gapX={1}>
              <PaxeerNetworkIcon /> {t("markets.openInterest")}
            </Flex>
          </button>
        ),
        dataIndex: "openInterest",

        align: "right",
        onSort: true,
        render: (value) => {
          return (
            <Text.numeral currency="$" dp={0} rm={Decimal.ROUND_DOWN}>
              {value}
            </Text.numeral>
          );
        },
      },
      {
        title: t("markets.dataList.column.8hFunding"),
        dataIndex: "8h_funding",

        align: "right",
        onSort: true,
        render: (value) => {
          if (value === null) {
            return "--";
          }
          return (
            <Text.numeral
              rule="percentages"
              coloring
              dp={4}
              rm={Decimal.ROUND_DOWN}
              showIdentifier
            >
              {value}
            </Text.numeral>
          );
        },
      },
      {
        dataIndex: "action",
        type: "action",

        render: (value, record) => {
          if (isFavoriteList) {
            return (
              <Flex justify="end" mr={4}>
                <Tooltip
                  content={t("markets.dataList.column.moveTop")}
                  align="center"
                  delayDuration={0}
                >
                  <Box
                    asChild
                    className="oui-cursor-pointer"
                    data-testid="oui-markets-favorites-pinned-icon"
                  >
                    <button
                      type="button"
                      aria-label={t("markets.dataList.column.moveTop")}
                      onClick={(e) => {
                        e.stopPropagation();
                        favorite.pinToTop(record);
                      }}
                      className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
                    >
                      <MoveToTopIcon className="oui-text-base-contrast-80 hover:oui-text-base-contrast" />
                    </button>
                  </Box>
                </Tooltip>
              </Flex>
            );
          }
          return null;
        },
      },
    ] as Column[];
  }, [favorite, isFavoriteList, t]);

  return columns;
};
