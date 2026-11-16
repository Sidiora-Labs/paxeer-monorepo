import { Flex, cn } from "@paxeer-network/ui";
import { SummaryWidget } from "./summary";
import { ReferralLinkWidget } from "./referralLink";
import { TitleStatisticWidget } from "./titleStatistic";
import { ReferralCodesWidget } from "./referralCodes";
import { CommissionAndRefereesWidget } from "./commissionAndReferees";
import surface from "../../surface.module.css";

export const AffiliatePage = () => (
  <Flex
    id="oui-affiliate-affiliate-page"
    className={cn(surface.surface, "oui-font-semibold")}
    direction="column"
    gap={4}
  >
    <div className={surface.columns}>
      <Flex direction="column" gap={4} className="oui-w-full">
        <SummaryWidget />
        <TitleStatisticWidget />
      </Flex>
      <Flex direction="column" gap={4} className="oui-w-full">
        <ReferralLinkWidget />
        <ReferralCodesWidget />
      </Flex>
    </div>
    <CommissionAndRefereesWidget />
  </Flex>
);
