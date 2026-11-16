import surface from "../../surface.module.css";
import { Flex, Box, cn } from "@paxeer-network/ui";
import { SummaryWidget } from "./summary";
import { TitleStatisticWidget } from "./titleStatistic";
import { RebatesWidget } from "./rebates";
export const TraderPage = () => {
  return (
    <Flex
      id="oui-affiliate-trader-page"
      className={cn(
        surface.surface,
        "oui-min-w-0 oui-w-full",
        // padding
        // "oui-p-4 lg:oui-p-6 xl:oui-p-3",
        "oui-font-semibold",
      )}
      direction={"column"}
      gap={4}
    >
      <Flex width={"100%"} gap={4} className={surface.columns}>
        <SummaryWidget />
        <TitleStatisticWidget />
      </Flex>
      <RebatesWidget />
    </Flex>
  );
};
