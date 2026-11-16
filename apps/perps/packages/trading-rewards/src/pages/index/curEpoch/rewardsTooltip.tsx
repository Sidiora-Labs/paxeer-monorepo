import { FC, ReactElement, useId } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { InfoCircleIcon } from "@paxeer-network/ui";
import { commifyOptional } from "@paxeer-network/utils";
import surface from "../../../surface.module.css";

export type RewardsTooltipProps = {
  brokerName: string | undefined;
  curRewards: number;
  otherRewards: number;
};

export const RewardsTooltip: FC<{
  rewardsTooltip?: RewardsTooltipProps;
  children?: ReactElement;
  className?: string;
  arrowClassName?: string;
  align?: "start" | "center" | "end";
}> = (props) => {
  const { t } = useTranslation();
  const id = useId();
  return (
    <>
      <button
        type="button"
        {...{ popovertarget: id }}
        aria-details={id}
        className={surface.infoTrigger}
        aria-label={
          props.children ? undefined : t("tradingRewards.rewardsEarned")
        }
      >
        {props.children ?? <InfoCircleIcon aria-hidden="true" />}
      </button>
      <div
        id={id}
        {...{ popover: "auto" }}
        className={`${surface.surface} ${surface.infoPopover}`}
      >
        <dl className="oui-grid oui-grid-cols-2 oui-gap-3 oui-text-sm">
          <dt>{props.rewardsTooltip?.brokerName}</dt>
          <dd className="oui-text-end">
            {commifyOptional(props.rewardsTooltip?.curRewards, { fix: 2 })}
          </dd>
          <dt>{t("tradingRewards.otherPaxeerNetworkDex")}</dt>
          <dd className="oui-text-end">
            {commifyOptional(props.rewardsTooltip?.otherRewards, { fix: 2 })}
          </dd>
        </dl>
        <button
          type="button"
          {...{ popovertarget: id, popovertargetaction: "hide" }}
          className="oui-mt-3 oui-text-sm oui-underline"
        >
          {t("common.close")}
        </button>
      </div>
    </>
  );
};
