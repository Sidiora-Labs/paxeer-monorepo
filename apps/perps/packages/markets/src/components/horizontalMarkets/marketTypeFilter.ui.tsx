import styles from "../../presentation.module.css";
import React, { useRef, useId, useCallback } from "react";
import { useMarkets, MarketsType } from "@paxeer-network/hooks";
import type { MarketTabConfig } from "@paxeer-network/hooks";
import { useTranslation } from "@paxeer-network/i18n";
import { Box, Flex, Text, cn, Checkbox, Divider } from "@paxeer-network/ui";
import { FilterIcon } from "../../icons";
import {
  isBuiltInMarketTab,
  isCustomMarketTab,
  resolveTabTitle,
} from "../shared/tabUtils";
import type { MarketType } from "./horizontalMarkets.script";

export type DropdownPos = "top" | "bottom";

export type MarketTypeFilterProps = {
  selectedMarketType: MarketType;
  onMarketTypeChange: (marketType: MarketType) => void;
  className?: string;
  position?: DropdownPos;
  tabs?: MarketTabConfig[];
};

export const MarketTypeFilter: React.FC<MarketTypeFilterProps> = (props) => {
  const {
    selectedMarketType,
    onMarketTypeChange,
    className,
    position = "bottom" as DropdownPos,
    tabs,
  } = props;
  const { t } = useTranslation();

  const orderedOptions = React.useMemo(() => {
    if (!tabs) return [];
    return tabs.filter(
      (tab) =>
        isCustomMarketTab(tab) ||
        (isBuiltInMarketTab(tab) && tab.type !== "favorites"),
    );
  }, [tabs]);
  const hasFavoritesTab = React.useMemo(() => {
    return tabs?.some(
      (tab) => isBuiltInMarketTab(tab) && tab.type === "favorites",
    );
  }, [tabs]);
  const popover = useRef<HTMLDivElement>(null);
  const instanceId = useId();

  // Access shared favorites store for tabs rendering
  const [, favorite] = useMarkets(MarketsType.FAVORITES);
  const hasFavorites = (favorite?.favorites?.length || 0) > 0;

  const titleOverrides = React.useMemo<Record<string, string>>(
    () => ({
      all: t("markets.allMarkets"),
      crypto: t("markets.crypto"),
      newListing: t("markets.newListings"),
      recent: t("markets.recent"),
      rwa: t("markets.tradFi"),
      preTge: t("markets.preTge"),
    }),
    [t],
  );

  const getOptionLabel = useCallback(
    (tab: MarketTabConfig) =>
      resolveTabTitle(tab, titleOverrides, titleOverrides.rwa),
    [titleOverrides],
  );

  // Handle option click
  const handleOptionClick = useCallback(
    (marketType: MarketType) => {
      onMarketTypeChange(marketType);
      popover.current?.hidePopover();
    },
    [onMarketTypeChange],
  );

  return (
    <div className={styles.filterAnchor}>
      <button
        type="button"
        popoverTarget={instanceId}
        aria-label={t("markets.allMarkets")}
        className={cn(
          styles.filterTarget,
          "oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4",
          className,
        )}
      >
        <FilterIcon className="oui-size-4 oui-text-base-contrast-80" />
      </button>
      <div
        ref={popover}
        id={instanceId}
        popover="auto"
        data-side={position}
        aria-label={t("markets.allMarkets")}
        className={cn(styles.surface, styles.filter)}
      >
        <Flex direction="column" gapY={3} itemAlign="start">
          {/* Configured Market Types */}
          <Flex
            direction="row"
            gapX={3}
            wrap="wrap"
            gapY={1}
            className="oui-gap-x-2 md:oui-gap-x-3"
          >
            {orderedOptions.map((tab) => {
              const optionValue = isCustomMarketTab(tab) ? tab.id : tab.type;
              const isActive = selectedMarketType === optionValue;
              const htmlId = `${instanceId}_toggle_market_type_${optionValue}`;
              return (
                <Flex
                  key={optionValue}
                  className={cn("oui-cursor-pointer")}
                  itemAlign="center"
                >
                  <Checkbox
                    data-testid={`oui-testid-marketType-${optionValue}-checkBox`}
                    id={htmlId}
                    className="oui-peer"
                    color="primary"
                    checked={isActive}
                    onCheckedChange={() =>
                      handleOptionClick(optionValue as MarketType)
                    }
                  />
                  <label
                    htmlFor={htmlId}
                    className={cn(
                      "oui-text-2xs oui-ms-1",
                      "oui-break-normal oui-whitespace-nowrap oui-cursor-pointer",
                    )}
                  >
                    {getOptionLabel(tab)}
                  </label>
                </Flex>
              );
            })}
          </Flex>

          {hasFavoritesTab && hasFavorites && (
            <>
              {/* Divider */}
              <Divider
                className="oui-w-full"
                direction="horizontal"
                intensity={16}
              />

              {/* Favorites Title */}
              <Text className="oui-text-xs oui-text-base-contrast-80 oui-font-medium">
                Favorites
              </Text>

              {/* Favorites Tabs as checkbox list */}
              <Flex
                direction="row"
                gapX={3}
                wrap="wrap"
                gapY={1}
                className="oui-gap-x-2 md:oui-gap-x-3"
              >
                {favorite.favoriteTabs?.slice(0, 10)?.map((tab) => {
                  const isActiveTab =
                    selectedMarketType === "favorites" &&
                    favorite.selectedFavoriteTab?.id === tab.id;
                  const htmlId = `${instanceId}_toggle_market_type_favorites_${tab.id}`;
                  return (
                    <Flex
                      key={tab.id}
                      className={cn("oui-cursor-pointer")}
                      itemAlign="center"
                    >
                      <Checkbox
                        data-testid={`oui-testid-marketType-favorites-${tab.id}-checkBox`}
                        id={htmlId}
                        className="oui-peer"
                        color="primary"
                        checked={isActiveTab}
                        onCheckedChange={() => {
                          favorite.updateSelectedFavoriteTab(tab);
                          onMarketTypeChange("favorites");
                          popover.current?.hidePopover();
                        }}
                      />

                      <label
                        htmlFor={htmlId}
                        className={cn(
                          "oui-text-2xs oui-ms-1",
                          "oui-break-normal oui-whitespace-nowrap oui-cursor-pointer",
                        )}
                      >
                        {tab.name}
                      </label>
                    </Flex>
                  );
                })}
              </Flex>
            </>
          )}
        </Flex>
      </div>
    </div>
  );
};
