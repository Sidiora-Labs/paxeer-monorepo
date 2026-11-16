import { FC, useMemo } from "react";
import { cn, useScreen } from "@paxeer-network/ui";
import { useCanTrade } from "../../hooks/useCanTrade";
import { CampaignsContentDesktopUI } from "./campaigns.content.desktop.ui";
import { useCampaignsScript } from "./campaigns.script";
import { CampaignsTimeDesktopUI } from "./components/time.desktop.ui";
import { CampaignsHeaderWidget } from "./header";

export type CampaignsWidgetProps = {
  className?: string;
  style?: React.CSSProperties;
  hideCampaignsBanner?: boolean;
};

export const CampaignsWidget: FC<CampaignsWidgetProps> = (props) => {
  const state = useCampaignsScript();
  const { isMobile } = useScreen();

  const canTrade = useCanTrade();

  const contentClassNames = useMemo(() => {
    if (!isMobile) return undefined;
    return {
      container: cn("oui-min-h-[calc(var(--oui-space-24)*4)] oui-gap-5"),
      time: "oui-text-sm",
      title: "oui-text-2xl oui-leading-normal",
      description: "oui-text-2xs oui-leading-normal",
      topContainer: cn("oui-w-full oui-px-3 oui-gap-1"),
    };
  }, [isMobile, canTrade]);

  return (
    <div className={cn(["oui-relative oui-z-[1]"])} style={props.style}>
      {!props.hideCampaignsBanner && (
        <CampaignsHeaderWidget
          backgroundSrc={state.backgroundSrc}
          campaigns={state.campaigns}
          currentCampaignId={state.currentCampaignId.toString()}
          onCampaignChange={state.onCampaignChange}
        />
      )}
      {state.currentCampaign && (
        <CampaignsContentDesktopUI
          {...state}
          classNames={contentClassNames}
          isMobile={isMobile}
          campaign={state.currentCampaign}
        />
      )}
      {state.currentCampaign && (
        <CampaignsTimeDesktopUI
          campaign={state.currentCampaign}
          isMobile={isMobile}
        />
      )}
    </div>
  );
};
