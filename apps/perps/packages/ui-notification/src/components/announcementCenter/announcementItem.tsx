import { FC, useMemo } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { AnnouncementType } from "@paxeer-network/types";
import { Text, ChevronDownIcon, cn } from "@paxeer-network/ui";
import {
  AnnouncementIcon,
  ArrowRightShortIcon,
  CampaignIcon,
  FundIcon,
  SecurityIcon,
} from "./icons";

export const AnnouncementItem: FC<{
  // title: string;
  expanded: boolean;
  url?: string | null;
  onItemClick: (url: string) => void;
  onExpandToggle?: () => void;
  type: AnnouncementType | undefined | null;
  message: string;
  updatedTime: number;
  className?: string;
  showDivider?: boolean;
}> = (props) => {
  const { t } = useTranslation();
  const Icon = useMemo(() => {
    switch (props.type) {
      case AnnouncementType.Campaign:
        return CampaignIcon;

      case AnnouncementType.Listing:
        return FundIcon;
      case AnnouncementType.Maintenance:
        return SecurityIcon;
      case AnnouncementType.Delisting:
      default:
        return AnnouncementIcon;
    }
  }, [props.type]);

  const title = useMemo(() => {
    switch (props.type) {
      case AnnouncementType.Campaign:
        return t("notification.campaign");
      case AnnouncementType.Delisting:
        return t("notification.delisting");
      case AnnouncementType.Listing:
        return t("notification.listing");
      case AnnouncementType.Maintenance:
        return t("notification.maintenance");
      case AnnouncementType.Vote:
        return t("notification.vote");
      default:
        return t("notification.general");
    }
  }, [props.type, t]);

  const action = useMemo(() => {
    if (
      (props.type === AnnouncementType.Campaign ||
        props.type === AnnouncementType.Vote) &&
      typeof props.url === "string" &&
      props.url !== "" &&
      typeof props.onItemClick === "function"
    ) {
      return (
        <button
          type="button"
          className="oui-shell-action oui-flex oui-items-center oui-gap-1"
          onClick={(event) => {
            event.stopPropagation();
            props.onItemClick(props.url!);
          }}
        >
          <Text size="xs" color="buy" className="oui-text-primary">
            {t("notification.joinNow")}
          </Text>
          <ArrowRightShortIcon size={18} />
        </button>
      );
    }
    return null;
  }, [props.type, props.url, t]);

  const updateTime = useMemo(() => {
    if (props.type === AnnouncementType.Maintenance) {
      return (
        <Text size="2xs" intensity={80}>
          {t("notification.recentlyUpdated")}
        </Text>
      );
    }
    return (
      <Text.formatted
        size="2xs"
        intensity={80}
        rule="date"
        formatString="yyyy-MM-dd HH:mm:ss"
      >
        {props.updatedTime}
      </Text.formatted>
    );
  }, [props.updatedTime, props.type, t]);

  return (
    <details
      open={props.expanded}
      onToggle={(event) => {
        if (event.currentTarget.open !== props.expanded)
          props.onExpandToggle?.();
      }}
      className={cn(
        "oui-announcement-item oui-rounded-md oui-p-2 oui-text-base-contrast-80",
        !props.showDivider && "hover:oui-bg-base-6",
        props.className,
      )}
    >
      <summary className="oui-flex oui-min-h-12 oui-cursor-pointer oui-items-center oui-gap-2">
        <Icon color="white" className="oui-shrink-0" />
        <span className="oui-min-w-0 oui-flex-1">
          <Text as="div" size="xs" intensity={80} weight="bold">
            {title}
          </Text>
          {updateTime}
        </span>
        <ChevronDownIcon
          aria-hidden="true"
          color="white"
          size={20}
          className={props.expanded ? "oui-rotate-180" : ""}
        />
      </summary>
      <div className="oui-flex oui-flex-col oui-gap-2 oui-pt-2 oui-break-words">
        <Text size="2xs" intensity={80} as="div">
          {props.message}
        </Text>
        {action}
      </div>
    </details>
  );
};
