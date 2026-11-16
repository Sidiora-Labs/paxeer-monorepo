import surface from "../../../../surface.module.css";
import { FC, SVGProps } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import {
  Box,
  CloseCircleFillIcon,
  cn,
  DataFilter,
  Flex,
  Input,
  Text,
} from "@paxeer-network/ui";
import {
  FilterDays,
  GeneralLeaderboardScriptReturn,
} from "../generalLeaderboard/generalLeaderboard.script";

export type LeaderboardFilterProps = GeneralLeaderboardScriptReturn;

export const LeaderboardFilter: FC<LeaderboardFilterProps> = (props) => {
  const { t } = useTranslation();
  const { useCampaignDateRange, weeklyRanges, setDateRange } = props;

  const weeklyView = (
    <Flex gap={3} className="oui-flex-wrap">
      {weeklyRanges.map((range) => (
        <button
          type="button"
          aria-pressed={props.dateRange?.label === range.label}
          onClick={() => setDateRange(range)}
          className="oui-relative oui-w-fit oui-whitespace-nowrap oui-px-2 oui-py-0 oui-text-sm"
          key={range.label}
        >
          <div className="oui-z-10">
            <Text.gradient
              color={
                props.dateRange?.label === range.label ? "brand" : undefined
              }
              className={
                props.dateRange?.label !== range.label
                  ? "oui-text-base-contrast-54"
                  : ""
              }
            >
              {`${range.label}`}
            </Text.gradient>
          </div>
          <div className="oui-pointer-events-none oui-absolute oui-inset-0 oui-rounded oui-opacity-10 oui-gradient-primary"></div>
        </button>
      ))}
    </Flex>
  );

  const input = (
    <Input
      value={props.searchValue}
      onValueChange={props.onSearchValueChange}
      placeholder={t("common.address.search.placeholder")}
      className={cn(
        "oui-trading-leaderboard-trading-search-input",
        "oui-w-full",
      )}
      size="sm"
      prefix={
        <Box pl={3} pr={1}>
          <SearchIcon className="oui-text-base-contrast-36" />
        </Box>
      }
      suffix={
        props.searchValue && (
          <Box mr={2}>
            <button
              type="button"
              aria-label={t("common.reset")}
              onClick={props.clearSearchValue}
            >
              <CloseCircleFillIcon
                size={14}
                className="oui-cursor-pointer oui-text-base-contrast-36"
              />
            </button>
          </Box>
        )
      }
      name="address"
      aria-label={t("common.address.search.placeholder")}
      autoComplete="on"
    />
  );

  const dateRangeView = props.filterItems.length > 0 && (
    <DataFilter
      items={props.filterItems}
      onFilter={(value) => {
        props.onFilter(value);
      }}
      className="oui-h-12 oui-border-none"
    />
  );

  const filterDayView = FilterDays.map((value) => {
    return (
      <button
        type="button"
        aria-pressed={props.filterDay === value}
        onClick={() => props.updateFilterDay(value)}
        className="oui-relative oui-px-2 oui-py-0 oui-text-sm"
        key={value}
      >
        <div className="oui-z-10">
          <Text.gradient
            color={props.filterDay === value ? "brand" : undefined}
            className={
              props.filterDay !== value ? "oui-text-base-contrast-54" : ""
            }
          >
            {`${value}D`}
          </Text.gradient>
        </div>
        <div className="oui-pointer-events-none oui-absolute oui-inset-0 oui-rounded oui-opacity-10 oui-gradient-primary"></div>
      </button>
    );
  });

  return (
    <div
      className={`${surface.filters} oui-trading-leaderboard-ranking-filter`}
    >
      <div className={surface.filterChoices}>
        {useCampaignDateRange ? (
          weeklyView
        ) : (
          <>
            {dateRangeView}
            {filterDayView}
          </>
        )}
      </div>
      <div className={surface.search}>{input}</div>
    </div>
  );
};

export const SearchIcon: FC<SVGProps<SVGSVGElement>> = (props) => (
  <svg
    width="14"
    height="14"
    viewBox="0 0 14 14"
    fill="currentColor"
    xmlns="http://www.w3.org/2000/svg"
    {...props}
  >
    <path d="M5.841 1.14a4.667 4.667 0 0 0 0 9.333 4.74 4.74 0 0 0 2.875-.975l2.54 2.56a.6.6 0 0 0 .838 0 .6.6 0 0 0 0-.838L9.537 8.677a4.72 4.72 0 0 0 .971-2.871 4.667 4.667 0 0 0-4.667-4.667m0 1.166a3.5 3.5 0 1 1 0 7 3.5 3.5 0 0 1 0-7" />
  </svg>
);
