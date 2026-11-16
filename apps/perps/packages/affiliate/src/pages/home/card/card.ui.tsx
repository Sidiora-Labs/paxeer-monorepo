import surface from "../../../surface.module.css";
import { FC } from "react";
import { Flex, Text } from "@paxeer-network/ui";
import { AsTraderWidget } from "../asATrader";
import { AsAnAffiliateWidget } from "../asAnAffilate";
import { CardReturns } from "./card.script";

export const Card: FC<CardReturns> = (props) => {
  if (typeof props.overwrite === "function") {
    return props.overwrite?.(props.state);
  }
  return (
    <Flex
      id="oui-affiliate-home-card"
      className={`${surface.surface} ${surface.columns}`}
    >
      <AsAnAffiliateWidget />
      <AsTraderWidget />
    </Flex>
  );
};
