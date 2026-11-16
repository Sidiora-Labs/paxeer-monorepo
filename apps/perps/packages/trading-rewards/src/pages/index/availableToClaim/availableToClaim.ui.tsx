import surface from "../../../surface.module.css";
import { FC } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { Flex, Text } from "@paxeer-network/ui";
import { commifyOptional } from "@paxeer-network/utils";
import { EsPaxeerNetworkIcon } from "../components/esPaxeerNetworkIcon";
import { JumpIcon } from "../components/jumpIcon";
import { PaxeerNetworkIcon } from "../components/paxeerNetworkIcon";
import { AvailableReturns } from "./availableToClaim.script";

export const AvailableToClaim: FC<AvailableReturns> = (props) => {
  const { t } = useTranslation();

  return (
    <Flex
      id="oui-tradingRewards-home-availableToClaim"
      p={6}
      r="2xl"
      direction={"column"}
      gap={4}
      width={"100%"}
      className={`${surface.surface}  oui-font-semibold oui-bg-base-9 `}
    >
      <Flex direction={"row"} justify={"between"} width={"100%"}>
        <Text className="oui-text-lg">
          {t("tradingRewards.availableClaim")}
        </Text>
        <button
          type="button"
          onClick={props.goToClaim}
          className="oui-inline-flex oui-items-center oui-gap-1 oui-text-primary-light oui-underline"
        >
          <Text size="sm">{t("tradingRewards.claim")}</Text>
          <JumpIcon />
        </button>
      </Flex>
      <Flex direction={"row"} gap={3} width={"100%"}>
        <Statics title="ORDER" value={props.order} />
        <Statics title="esORDER" value={props.esOrder} isEsOrder />
      </Flex>
    </Flex>
  );
};

const Statics: FC<{
  title: string;
  isEsOrder?: boolean;
  value?: number;
}> = (props) => {
  const { value } = props;
  return (
    <Flex
      className="oui-flex-1 oui-bg-base-8 oui-py-3"
      direction={"column"}
      gap={2}
      r="xl"
      gradient="neutral"
      angle={180}
    >
      <Text className="oui-text-xs xl:oui-text-sm oui-text-base-contrast-54">
        {props.title}
      </Text>
      <Flex direction={"row"} gap={1}>
        {props.isEsOrder ? <EsPaxeerNetworkIcon /> : <PaxeerNetworkIcon />}
        <Text
          className="oui-text-sm xl:oui-text-base"
          children={commifyOptional(value, { fix: 2 })}
        />
      </Flex>
    </Flex>
  );
};
