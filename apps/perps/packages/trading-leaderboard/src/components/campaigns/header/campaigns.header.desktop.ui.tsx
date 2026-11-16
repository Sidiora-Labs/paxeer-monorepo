import { FC, useEffect, useId, useRef } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { DefaultCampaign } from "../DefaultCampaign";
import { CampaignItemUI } from "../campaign.item.ui";
import { CampaignConfig } from "../type";
import { getCampaignTag } from "../utils";
import surface from "../../../surface.module.css";

export const CampaignsHeaderUI: FC<{
  campaigns: CampaignConfig[];
  currentCampaignId: string;
  onCampaignChange: (campaignId: string) => void;
  backgroundSrc?: string;
}> = ({ campaigns, currentCampaignId, onCampaignChange, backgroundSrc }) => {
  const { t } = useTranslation();
  const id = useId();
  const track = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const active = track.current?.querySelector<HTMLElement>(
      '[aria-pressed="true"]',
    );
    const container = track.current;
    if (!active || !container) return;
    const activeRect = active.getBoundingClientRect();
    const trackRect = container.getBoundingClientRect();
    if (activeRect.left < trackRect.left)
      container.scrollBy({ left: activeRect.left - trackRect.left });
    else if (activeRect.right > trackRect.right)
      container.scrollBy({ left: activeRect.right - trackRect.right });
  }, [currentCampaignId]);
  const scroll = (direction: number) =>
    track.current?.scrollBy({ left: direction * track.current.clientWidth });
  return (
    <section
      className={`${surface.surface} ${surface.campaignHeader}`}
      aria-label={t("tradingLeaderboard.generalLeaderboard")}
    >
      {campaigns.length > 0 && (
        <button
          type="button"
          aria-label="Previous campaigns"
          aria-controls={id}
          onClick={() => scroll(-1)}
          className="oui-rounded-lg oui-p-2 hover:oui-bg-base-7"
        >
          ‹
        </button>
      )}
      <div id={id} ref={track} className={surface.campaignTrack}>
        <DefaultCampaign
          currentCampaignId={currentCampaignId}
          onCampaignChange={onCampaignChange}
          campaigns={campaigns}
        />
        {campaigns.map((campaign) => (
          <CampaignItemUI
            key={campaign.campaign_id}
            backgroundSrc={backgroundSrc}
            campaign={campaign}
            tag={getCampaignTag(campaign)}
            active={currentCampaignId === String(campaign.campaign_id)}
            onCampaignChange={onCampaignChange}
            classNames={{ tag: { container: "oui-py-1 oui-px-2" } }}
          />
        ))}
      </div>
      {campaigns.length > 0 && (
        <button
          type="button"
          aria-label="Next campaigns"
          aria-controls={id}
          onClick={() => scroll(1)}
          className="oui-rounded-lg oui-p-2 hover:oui-bg-base-7"
        >
          ›
        </button>
      )}
    </section>
  );
};
