import surface from "../../../surface.module.css";
import { FC, ReactNode } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { Flex, Text } from "@paxeer-network/ui";
import { commify, commifyOptional } from "@paxeer-network/utils";
import { EsPaxeerNetworkIcon } from "../components/esPaxeerNetworkIcon";
import { JumpIcon } from "../components/jumpIcon";
import { PaxeerNetworkIcon } from "../components/paxeerNetworkIcon";
import { RocketIcon } from "../components/rocket";
import { StakeBoosterReturns } from "./stakeBooster.script";

export const StakeBooster: FC<StakeBoosterReturns> = (props) => {
  const { t } = useTranslation();

  return (
    <Flex
      id="oui-tradingRewards-home-stakeBooster"
      p={6}
      r="2xl"
      direction={"column"}
      gap={4}
      width={"100%"}
      className={`${surface.surface}  oui-font-semibold oui-bg-base-9 `}
    >
      <Flex direction={"row"} justify={"between"} width={"100%"}>
        <Text className="oui-text-lg">{t("tradingRewards.stakeBooster")}</Text>
        <button
          type="button"
          onClick={props.stakeNow}
          className="oui-inline-flex oui-items-center oui-gap-1 oui-text-primary-light oui-underline"
        >
          <Text size="sm">{t("tradingRewards.stake")}</Text>
          <JumpIcon />
        </button>
      </Flex>
      <Flex direction={"row"} gap={3} width={"100%"}>
        <Statics
          title={t("tradingRewards.avgStakedAmount")}
          value={props.curEpochEstimate?.est_avg_stake}
          icon={
            <div className="oui-flex oui-w-8 oui-h-5 oui-relative">
              <div className="oui-absolute oui-end-0 oui-top-0">
                <EsPaxeerNetworkIcon />
              </div>
              <div className="oui-absolute oui-start-0 oui-top-0 ">
                <PaxeerNetworkIcon />
              </div>
            </div>
          }
        />
        <Statics
          title={t("tradingRewards.booster")}
          value={props.booster}
          icon={<RocketIcon />}
          gradient
        />
      </Flex>
    </Flex>
  );
};

const Statics: FC<{
  title: string;
  icon: ReactNode;
  value?: number | string;
  gradient?: boolean;
}> = (props) => {
  const calcValue = commify(props.value || "--", 2);
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
        {props.icon}
        {props.gradient ? (
          <Text.gradient
            className="oui-text-sm xl:oui-text-base"
            color="brand"
            angle={90}
          >
            {calcValue + (calcValue === "--" ? "" : "x")}
          </Text.gradient>
        ) : (
          <Text className="oui-text-sm xl:oui-text-base">
            {commifyOptional(props.value, { fix: 2 })}
          </Text>
        )}
      </Flex>
    </Flex>
  );
};
