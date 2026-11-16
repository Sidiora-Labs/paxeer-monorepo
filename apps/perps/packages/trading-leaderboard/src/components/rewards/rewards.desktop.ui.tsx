import { FC, useMemo, useState } from "react";
import { useTranslation, Trans } from "@paxeer-network/i18n";
import { InfoCircleIcon, Tooltip, Text, Button, cn } from "@paxeer-network/ui";
import { commify } from "@paxeer-network/utils";
import { CampaignConfig, UserData } from "../campaigns/type";
import {
  calculateEstimatedRewards,
  calculateEstimatedTickets,
  calculateUserPoolReward,
  formatRewardAmount,
  formatTicketCount,
  calculateTicketProgress,
} from "./utils";

interface RewardsDesktopUIProps {
  campaign?: CampaignConfig;
  userdata?: UserData;
  onLearnMore: () => void;
  onTradeNow: () => void;
  isMobile?: boolean;
  shouldShowJoinButton?: boolean;
  joinCampaign?: (data: { campaign_id: string | number }) => Promise<any>;
  isJoining?: boolean;
}

export const RewardsDesktopUI: FC<RewardsDesktopUIProps> = ({
  campaign,
  userdata,
  onLearnMore,
  onTradeNow,
  isMobile,
  shouldShowJoinButton,
  joinCampaign,
  isJoining,
}) => {
  const { t } = useTranslation();
  const currentUserData = userdata;

  // Calculate estimated rewards
  const estimatedRewards =
    campaign && currentUserData
      ? calculateEstimatedRewards(currentUserData, campaign)
      : null;

  // Calculate estimated tickets
  const estimatedTickets =
    campaign?.ticket_rules && currentUserData
      ? calculateEstimatedTickets(currentUserData, campaign.ticket_rules)
      : 0;

  const rewardText = estimatedRewards
    ? formatRewardAmount(estimatedRewards.amount, estimatedRewards.currency)
    : "0 USDC";

  const ticketText = formatTicketCount(estimatedTickets);

  const canTrade = useMemo(() => {
    return (
      campaign?.start_time &&
      campaign?.end_time &&
      campaign.start_time < new Date().toISOString() &&
      campaign.end_time > new Date().toISOString()
    );
  }, [campaign, t]);

  const ticketTooltipContent = useMemo(() => {
    const ticketRules = campaign?.ticket_rules;

    if (!ticketRules) {
      return null;
    }

    if (ticketRules.mode === "linear") {
      return (
        <div>
          {t("tradingLeaderboard.earnTickets", {
            ticket: ticketRules?.linear?.tickets,
            amount: commify(ticketRules?.linear?.every || 0),
          })}
        </div>
      );
    }

    return (
      <div className="oui-flex oui-min-w-[calc(var(--oui-space-24)*2)] oui-flex-col oui-gap-1">
        {ticketRules?.tiers?.map((tier) => {
          return (
            <div
              key={tier.value}
              className="oui-min-h-5 oui-flex oui-items-center oui-justify-between oui-text-2xs oui-font-semibold"
            >
              <Text.numeral
                currency="$"
                suffix=" volume"
                dp={0}
                className="oui-text-base-contrast-54"
              >
                {tier.value}
              </Text.numeral>
              <div className="oui-text-base-contrast">
                {tier.tickets} tickets
              </div>
            </div>
          );
        })}
      </div>
    );
  }, [campaign, t]);

  const extraProps = useMemo(() => {
    if (
      !userdata ||
      !campaign?.ticket_rules ||
      campaign.end_time < new Date().toISOString() ||
      campaign.start_time > new Date().toISOString()
    ) {
      return {
        showExtraInfo: false,
        extraInfo: null,
      };
    }

    const progress = calculateTicketProgress(userdata, campaign.ticket_rules);

    if (!progress) {
      return {
        showExtraInfo: false,
        extraInfo: null,
      };
    }

    return {
      showExtraInfo: true,
      extraInfo: progress,
    };
  }, [campaign, userdata]);

  return (
    <div
      className={cn([
        "oui-mx-auto oui-mb-10 oui-flex oui-max-w-screen-xl oui-flex-col oui-pb-10",
        isMobile ? "oui-gap-3" : "oui-gap-6",
      ])}
    >
      <div
        className={cn([
          "oui-flex oui-w-full oui-items-stretch oui-gap-3",
          isMobile ? "oui-px-3" : "",
        ])}
      >
        <RewardItem
          showTooltip={!!campaign?.ticket_rules}
          title={t("tradingLeaderboard.estimatedTicketsEarned")}
          value={ticketText}
          tooltip={ticketTooltipContent}
          {...extraProps}
          isMobile={isMobile}
        />
      </div>
      <div
        className={cn([
          "oui-flex oui-justify-center oui-gap-3",
          isMobile ? "oui-px-3" : "",
        ])}
      >
        {campaign?.rule_url && (
          <Button
            size={isMobile ? "md" : "lg"}
            variant="outlined"
            className={cn([
              "oui-border-primary oui-text-primary hover:oui-bg-primary/[0.08] active:oui-bg-primary/[0.08]",
              isMobile ? "oui-flex-1" : "oui-w-[calc(var(--oui-space-24)*1)]",
            ])}
            onClick={onLearnMore}
          >
            {t("tradingLeaderboard.viewRules")}
          </Button>
        )}
        {shouldShowJoinButton && (
          <Button
            size={isMobile ? "md" : "lg"}
            variant="contained"
            color="primary"
            loading={isJoining}
            disabled={isJoining}
            className={cn([
              isMobile ? "oui-flex-1" : "oui-w-[calc(var(--oui-space-24)*1)]",
            ])}
            onClick={() =>
              joinCampaign?.({ campaign_id: Number(campaign?.campaign_id) })
            }
          >
            {t("tradingLeaderboard.joinNow")}
          </Button>
        )}
        {!shouldShowJoinButton && canTrade && (
          <Button
            size={isMobile ? "md" : "lg"}
            variant="contained"
            color="primary"
            className={cn([
              isMobile ? "oui-flex-1" : "oui-w-[calc(var(--oui-space-24)*1)]",
            ])}
            onClick={onTradeNow}
          >
            {t("tradingLeaderboard.tradeNow")}
          </Button>
        )}
      </div>
    </div>
  );
};

const RewardItem: FC<{
  title: string;
  value: string;
  showTooltip?: boolean;
  tooltip?: any;
  showExtraInfo?: boolean;
  extraInfo?: {
    percent: number;
    value: number;
  } | null;
  isMobile?: boolean;
}> = (props) => {
  const [tooltipOpen, setTooltipOpen] = useState(false);

  const tooltipProps = useMemo(() => {
    if (!props.isMobile) {
      return {};
    }
    return {
      open: tooltipOpen,
      onOpenChange: setTooltipOpen,
    };
  }, [tooltipOpen, props.isMobile, setTooltipOpen]);

  return (
    <div className="oui-flex oui-flex-1 oui-flex-col oui-items-center oui-justify-center oui-rounded-2xl oui-bg-base-9 oui-px-5 oui-py-4">
      <div
        className={cn([
          "oui-text-base-contrast-54",
          props.isMobile
            ? "oui-h-4 oui-text-2xs oui-leading-normal"
            : "oui-text-sm oui-font-semibold",
        ])}
      >
        {props.title}
      </div>
      <div className="oui-flex oui-items-center oui-gap-2">
        <Text.gradient
          weight="bold"
          color="brand"
          className={cn([
            "oui-trading-leaderboard-title",
            props.isMobile
              ? "oui-h-4 oui-text-base oui-leading-normal"
              : "oui-h-10 oui-text-4xl oui-leading-normal",
          ])}
        >
          {props.value}
        </Text.gradient>
        {props.showTooltip && (
          <Tooltip content={props.tooltip} {...tooltipProps}>
            <button
              type="button"
              aria-label="Prize pool details"
              className="oui-flex oui-size-4 oui-items-center oui-justify-center"
              onClick={() => setTooltipOpen(true)}
            >
              <InfoCircleIcon className="oui-cursor-pointer" />
            </button>
          </Tooltip>
        )}
      </div>
      {props.showExtraInfo && (
        <div
          className={cn([
            "oui-flex oui-flex-col oui-items-center oui-justify-end",
            props.isMobile ? "oui-mt-2" : "",
          ])}
        >
          <div className="oui-flex oui-flex-col oui-items-center oui-gap-1 oui-text-2xs oui-font-semibold oui-text-base-contrast-36">
            <div
              className={cn([
                "oui-flex oui-h-4 oui-w-[calc(var(--oui-space-24)*2)] oui-items-center oui-rounded-full oui-bg-base-5 oui-p-0",
                props.isMobile
                  ? "oui-w-full"
                  : "oui-w-[calc(var(--oui-space-24)*2)]",
              ])}
            >
              <div
                className={cn([
                  "oui-h-3 oui-rounded-full",
                  "oui-gradient-brand",
                ])}
                style={{ width: `${props?.extraInfo?.percent}%` }}
              />
            </div>
            <div
              className={cn([
                "oui-text-center oui-font-semibold",
                props.isMobile ? "oui-text-2xs oui-leading-normal" : "",
              ])}
            >
              {/* @ts-ignore */}
              {props.extraInfo.atMax ? (
                <Trans i18nKey="tradingLeaderboard.maxTicketsAchieved" />
              ) : (
                <Trans
                  i18nKey="tradingLeaderboard.tradeForMoreTickets"
                  components={[
                    <span key="0" className="oui-text-base-contrast">
                      ${props?.extraInfo?.value?.toFixed(2)}
                    </span>,
                  ]}
                />
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
