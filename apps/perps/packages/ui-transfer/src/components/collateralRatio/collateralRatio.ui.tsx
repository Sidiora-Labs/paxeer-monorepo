import React from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { Box, Flex, Text, Tooltip } from "@paxeer-network/ui";
import { Decimal } from "@paxeer-network/utils";
import { TooltipIcon } from "../icons/tooltipIcon";
import styles from "../../presentation.module.css";

const TooltipContent: React.FC = () => {
  const { t } = useTranslation();
  return (
    <Box className="oui-w-72 oui-max-w-full">
      <Text size="2xs" intensity={80}>
        {t("portfolio.overview.column.collateralRatio.explain")}
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

export const CollateralRatioUI: React.FC<{ value: number }> = (props) => {
  const { t } = useTranslation();
  const { value } = props;
  return (
    <Flex
      className={styles.infoRow}
      width="100%"
      itemAlign="center"
      justify="between"
    >
      <Flex justify="start" itemAlign="center">
        <Text size="2xs" intensity={80}>
          {t("portfolio.overview.column.collateralRatio")}
        </Text>
        <Tooltip className="oui-p-2" content={<TooltipContent />}>
          <button
            type="button"
            aria-label={t("portfolio.overview.column.collateralRatio")}
            className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center"
          >
            <TooltipIcon className="oui-text-base-contrast-80" />
          </button>
        </Tooltip>
      </Flex>
      <Text.numeral
        dp={2}
        rm={Decimal.ROUND_DOWN}
        coloring
        className="oui-font-semibold"
        rule="percentages"
      >
        {value}
      </Text.numeral>
    </Flex>
  );
};
