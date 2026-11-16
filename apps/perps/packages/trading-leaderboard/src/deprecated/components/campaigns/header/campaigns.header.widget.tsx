import { CampaignConfig } from "../type";
import { CampaignsHeaderUI } from "./campaigns.header.desktop.ui";

export const CampaignsHeaderWidget = (props: {
  campaigns: CampaignConfig[];
  currentCampaignId: string;
  onCampaignChange: (campaignId: string) => void;
  backgroundSrc?: string;
  totalPrizePool?: { amount: number; currency: string };
}) => {
  return <CampaignsHeaderUI {...props} />;
};
