import { tokenDisplayName } from "@paxeer-network/utils";
import React, { useContext } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { cn, Flex, Text } from "@paxeer-network/ui";
import { Decimal } from "@paxeer-network/utils";

import { DepositContext } from "../provider/DepositProvider";

export const CollateralContribution: React.FC<{
  value: number;
  precision?: number;
}> = (props) => {
  const { t } = useTranslation();
  const { collateralUnit } = useContext(DepositContext);
  const { value } = props;
  const precision =
    collateralUnit?.precision ??
    collateralUnit?.decimals ??
    props.precision ??
    6;
  return (
    <Flex width="100%" itemAlign="center" justify="between">
      <Flex itemAlign="center" justify="start">
        <Text size="2xs" intensity={36}>
          {t("transfer.deposit.collateralContribution")}
        </Text>
      </Flex>
      <Flex itemAlign="center" justify="end" gap={1}>
        <Text.numeral
          dp={precision}
          size="2xs"
          rm={Decimal.ROUND_DOWN}
          className={cn("oui-font-semibold")}
          padding={false}
        >
          {value}
        </Text.numeral>
        <Text size="2xs" intensity={36} className="oui-select-none">
          {tokenDisplayName(collateralUnit?.symbol ?? "USDC")}
        </Text>
      </Flex>
    </Flex>
  );
};
