import { FC } from "react";
import { API } from "@paxeer-network/types";
import { cn } from "@paxeer-network/ui";
import { NotificationUI as NotificationUIComponent } from "@paxeer-network/ui-notification";
import { useScaffoldContext } from "../scaffold";

export const NotificationUI: FC<{
  dataSource: API.AnnouncementRow[];
  onClose: () => void;
  showAnnouncement: boolean;
}> = (props) => {
  const { dataSource, showAnnouncement } = props;
  const { routerAdapter } = useScaffoldContext();
  const onItemClick = (url: string) => {
    if (!url) return;
    routerAdapter?.onRouteChange({
      href: url,
      name: url,
      target: "_blank",
    });
  };

  if (!showAnnouncement || dataSource.length === 0) return null;

  return (
    <div
      role="region"
      aria-label="Announcements"
      data-state={showAnnouncement ? "open" : "closed"}
      className={cn(
        "oui-shell-notification oui-rounded-lg oui-border oui-border-line-6 oui-bg-base-8",
        "oui-transition-all oui-[transition-duration:var(--oui-duration-normal)] oui-ease-in-out",
        showAnnouncement ? "oui-visible" : "oui-invisible",
      )}
    >
      <NotificationUIComponent
        dataSource={dataSource}
        onClose={props.onClose}
        onItemClick={onItemClick}
      />
    </div>
  );
};
