import { FC } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { cn } from "@paxeer-network/ui";
import { CampaignConfig } from "../type";
import { generateCampaignTimeline } from "../utils";
import { CampaignsAxis } from "./axis";
import { CampaignsCountdown } from "./countdown";

export const CampaignsTimeDesktopUI: FC<{
  campaign: CampaignConfig;
  isMobile?: boolean;
}> = ({ campaign, isMobile }) => {
  useTranslation();
  const timelineData = generateCampaignTimeline(campaign);

  return (
    <div
      className={cn([
        "oui-max-w-screen-xl oui-mx-auto oui-flex oui-flex-col oui-items-center",
        isMobile ? "oui-pt-4 oui-gap-10 oui-pb-10" : "oui-py-10 oui-gap-4",
      ])}
    >
      <CampaignsCountdown campaign={campaign} isMobile={isMobile} />
      <CampaignsAxis points={timelineData} />
    </div>
  );
};
