import styles from "../../presentation.module.css";
import React from "react";
import { useTranslation } from "@paxeer-network/i18n";
import {
  cn,
  Flex,
  PopoverRoot,
  PopoverTrigger,
  PopoverContent,
  Text,
  Box,
} from "@paxeer-network/ui";
import { TooltipIcon } from "../icons/tooltipIcon";
import type { LtvScriptReturns } from "./ltv.script";

const calculateTextColor = (val: number): string => {
  if (val >= 0 && val < 50) {
    return "oui-text-success";
  } else if (val >= 50 && val < 80) {
    return "oui-text-warning";
  } else if (val >= 80) {
    return "oui-text-danger";
  } else {
    return "";
  }
};

const TooltipContent: React.FC<{
  isLoading: boolean;
  ltv_threshold: string;
}> = (props) => {
  const { isLoading, ltv_threshold } = props;
  const { t } = useTranslation();
  return (
    <Box className="oui-w-72 oui-max-w-full">
      <Text size="2xs" intensity={80}>
        {t("transfer.LTV.description", {
          threshold: isLoading ? "-" : ltv_threshold,
        })}
      </Text>{" "}
      <a
        href="https://paxeer-network.network/docs/introduction/trade-on-paxeer-network/multi-collateral"
        target="_blank"
        rel="noreferrer"
        className={
          "oui-border-b oui-border-dashed oui-border-line-12 oui-text-2xs oui-text-primary"
        }
      >
        {t("tradingLeaderboard.learnMore")}
      </a>
    </Box>
  );
};

export const LtvUI: React.FC<
  Readonly<
    LtvScriptReturns & {
      currentLtv: number;
      nextLTV: number;
      showDiff?: boolean;
    }
  >
> = (props) => {
  const { t } = useTranslation();
  const { currentLtv, nextLTV, showDiff, ltv_threshold, isLoading } = props;
  return (
    <Flex
      className={styles.infoRow}
      width="100%"
      itemAlign="center"
      justify="between"
    >
      <Flex justify="start" itemAlign="center">
        <Text size="2xs" intensity={80} className="oui-whitespace-nowrap">
          {t("transfer.LTV")}
        </Text>
        <PopoverRoot>
          <PopoverTrigger asChild>
            <button
              type="button"
              aria-label={t("transfer.LTV")}
              className="oui-inline-flex oui-items-center oui-justify-center"
            >
              <TooltipIcon className="oui-text-base-contrast-80" />
            </button>
          </PopoverTrigger>
          <PopoverContent
            className={`${styles.surface} ${styles.popover} oui-bg-base-8 oui-p-3`}
          >
            <TooltipContent
              isLoading={isLoading}
              ltv_threshold={ltv_threshold}
            />
          </PopoverContent>
        </PopoverRoot>
      </Flex>
      {showDiff ? (
        <Flex
          className={styles.infoValues}
          itemAlign="center"
          justify="between"
          gap={1}
        >
          <Text
            size="2xs"
            className={cn("oui-font-semibold", calculateTextColor(currentLtv))}
          >
            {currentLtv}%
          </Text>
          →
          <Text
            size="2xs"
            className={cn("oui-font-semibold", calculateTextColor(nextLTV))}
          >
            {nextLTV}%
          </Text>
        </Flex>
      ) : (
        <Text
          size="2xs"
          className={cn("oui-font-semibold", calculateTextColor(currentLtv))}
        >
          {currentLtv}%
        </Text>
      )}
    </Flex>
  );
};
