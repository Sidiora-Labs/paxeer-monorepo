import { FC, useId, useMemo, useState } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { AnnouncementType, type API } from "@paxeer-network/types";
import {
  ChevronDownIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  Divider,
  Flex,
  Text,
} from "@paxeer-network/ui";
import {
  AnnouncementIcon,
  BattleIcon,
  FundIcon,
  SecurityIcon,
} from "../announcementCenter/icons";
import {
  CampaignContentCard,
  CommunityVoteContentCard,
  DelistingContentCard,
  ListingContentCard,
  MaintenanceContentCard,
} from "./contentCard";

export interface NotificationProps {
  // Define your notification props here
  className?: string;
  // children?: React.ReactNode;
  dataSource: API.AnnouncementRow[];
  onClose?: () => void;
}

export interface NotificationItemProps {
  id: string;
  type?: "success" | "error" | "warning" | "info";
  title?: string;
  message: string;
  onClose?: () => void;
}

export interface NotificationListProps {
  notifications: NotificationItemProps[];
  onRemove?: (id: string) => void;
}

const NotificationHeader: FC<{
  // type: AnnouncementType;
  dataSource: API.AnnouncementRow[];
  current: number;
  expanded?: boolean;
  onExpandToggle?: () => void;
  contentId: string;
}> = (props) => {
  const { t } = useTranslation();
  const { expanded } = props;
  const { type } = props.dataSource[props.current];

  const title = useMemo(() => {
    switch (type) {
      case AnnouncementType.Campaign:
        return (
          <Text
            size="sm"
            className="oui-text-transparent oui-bg-clip-text oui-gradient-brand"
          >
            {t("notification.campaign")}
          </Text>
        );
      case AnnouncementType.Delisting:
        return <Text size="sm">{t("notification.delistingTitle")}</Text>;
      case AnnouncementType.Listing:
        return (
          <Text size="sm" color="buy">
            {t("notification.listing")}
          </Text>
        );
      case AnnouncementType.Maintenance:
        return (
          <Text size="sm" color="warning">
            {t("notification.maintenanceTitle")}
          </Text>
        );
      case AnnouncementType.Vote:
        return (
          <Text size="sm" color="inherit">
            {t("notification.vote")}
          </Text>
        );
      default:
        return (
          <Text size="sm" color="inherit">
            {t("notification.generalTitle")}
          </Text>
        );
    }
  }, [type, t]);
  const icon = useMemo(() => {
    switch (type) {
      case AnnouncementType.Campaign:
        return <BattleIcon color="white" />;

      case AnnouncementType.Listing:
        return <FundIcon color="success" />;
      case AnnouncementType.Maintenance:
        return <SecurityIcon color="warning" />;
      case AnnouncementType.Delisting:
      default:
        return <AnnouncementIcon color="white" />;
    }
  }, [type]);
  return (
    <Flex itemAlign="center" justify="between" className="oui-px-4 oui-py-3">
      <div className="paxeer-network-notification-header oui-flex oui-items-center oui-gap-2">
        {icon}
        {title}
      </div>
      <button
        type="button"
        aria-label={t("notification.generalTitle")}
        aria-expanded={!!expanded}
        aria-controls={props.contentId}
        onClick={props.onExpandToggle}
        className="oui-shell-icon-button oui-transition-transform oui-[transition-duration:var(--oui-duration-normal)]"
        style={{
          transform: !expanded ? "rotate(180deg)" : "rotate(0deg)",
        }}
      >
        <ChevronDownIcon size={18} color="white" />
      </button>
    </Flex>
  );
};

const NotificationFooter: FC<{
  total: number;
  current: number;
  onCloseAll: () => void;
  onPrev: () => void;
  onNext: () => void;
}> = (props) => {
  const { total, current, onCloseAll, onPrev, onNext } = props;
  const { t } = useTranslation();
  return (
    <Flex
      className="paxeer-network-notification-footer oui-px-4 oui-py-2"
      itemAlign="center"
      justify="between"
    >
      <Flex gap={2}>
        <button
          type="button"
          aria-label="Previous announcement"
          disabled={current - 1 < 0}
          onClick={onPrev}
          className="oui-shell-icon-button oui-rounded-full oui-bg-base-6 hover:oui-bg-base-5 disabled:oui-opacity-50"
        >
          <ChevronLeftIcon size={14} color="white" />
        </button>
        <Text intensity={80} size="sm">
          {current + 1}/{total}
        </Text>
        <button
          type="button"
          aria-label="Next announcement"
          disabled={current + 1 >= total}
          onClick={onNext}
          className="oui-shell-icon-button oui-rounded-full oui-bg-base-6 hover:oui-bg-base-5 disabled:oui-opacity-50"
        >
          <ChevronRightIcon size={14} color="white" />
        </button>
      </Flex>
      <button type="button" className="oui-shell-action" onClick={onCloseAll}>
        <Text size="xs" color="primary">
          {t("notification.closeAll", { total })}
        </Text>
      </button>
    </Flex>
  );
};

const NotificationContent: FC<{
  dataSource: (API.AnnouncementRow & {
    startTime?: number;
    endTime?: number;
  })[];
  current: number;
  onItemClick: (url: string) => void;
}> = (props) => {
  const elements = useMemo(() => {
    const message = props.dataSource[props.current];
    if (!message) return null;
    return (() => {
      const { type } = message;
      switch (type) {
        case AnnouncementType.Campaign:
          return (
            <CampaignContentCard
              message={message.message}
              coverImage={message.coverImage ?? ""}
              updateTime={message.updated_time ?? 0}
              url={message.url ?? ""}
              onItemClick={props.onItemClick}
            />
          );
        case AnnouncementType.Maintenance:
          return (
            <MaintenanceContentCard
              message={message.message}
              startTime={message.startTime ?? 0}
              endTime={message.endTime ?? 0}
            />
          );
        case AnnouncementType.Delisting:
          return (
            <DelistingContentCard
              message={message.message}
              updateTime={message.updated_time ?? 0}
            />
          );
        case AnnouncementType.Vote:
          return (
            <CommunityVoteContentCard
              message={message.message}
              url={message.url ?? ""}
              onItemClick={props.onItemClick}
            />
          );
        default:
          return (
            <ListingContentCard
              message={message.message}
              updateTime={message.updated_time ?? 0}
            />
          );
      }
    })();
  }, [props.dataSource, props.current, props.onItemClick]);
  return <>{elements}</>;
};

export const NotificationUI: FC<
  NotificationProps & {
    maintenanceMessage?: string;
    onItemClick: (url: string) => void;
  }
> = (props) => {
  const [expanded, setExpanded] = useState(true);

  const [selectedIndex, setCurrent] = useState(0);
  const current = Math.min(
    selectedIndex,
    Math.max(0, props.dataSource.length - 1),
  );
  const contentId = useId();
  const len = useMemo(() => props.dataSource?.length ?? 0, [props.dataSource]);

  if (len === 0) {
    return null;
  }

  return (
    <div className="paxeer-network-notification oui-w-full ">
      <NotificationHeader
        contentId={contentId}
        dataSource={props.dataSource ?? []}
        current={current}
        expanded={expanded}
        onExpandToggle={() => {
          setExpanded(!expanded);
        }}
      />
      <Divider className="oui-mx-4" />
      <div id={contentId} hidden={!expanded}>
        <div className="oui-px-4 oui-py-3 oui-break-words">
          <NotificationContent
            dataSource={props.dataSource}
            current={current}
            onItemClick={props.onItemClick}
          />
        </div>
      </div>

      <NotificationFooter
        total={len}
        current={current}
        onCloseAll={props.onClose ?? (() => {})}
        onPrev={() => {
          if (current - 1 < 0) {
            return;
          }
          setCurrent(current - 1);
        }}
        onNext={() => {
          if (current + 1 >= len) {
            return;
          }
          setCurrent(current + 1);
        }}
      />
    </div>
  );
};
