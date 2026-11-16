import { InfoPopover as Tips } from "../../../../components/infoPopover";
import surface from "../../../../surface.module.css";
import { type ReactNode, useId, useMemo } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import {
  Box,
  Button,
  Flex,
  Slider,
  Text,
  Divider,
  Tooltip,
  formatAddress,
} from "@paxeer-network/ui";
import { ReferralCodeFormField, ReferralCodeFormType } from "../../../../types";
import { ReferralCodeFormReturns } from "./referralCodeForm.script";
import { ReferralCodeFormWidgetProps } from "./referralCodeForm.widget";
import { ReferralCodeInput } from "./referralCodeInput";

export type ReferralCodeFormProps = ReferralCodeFormReturns &
  Omit<ReferralCodeFormWidgetProps, "type">;

export const ReferralCodeForm = (props: ReferralCodeFormProps) => {
  const { type, isReview } = props;
  const { t } = useTranslation();

  const isReset = type === ReferralCodeFormType.Reset;
  const hasBoundReferee = !!props.directInvites && props.directInvites > 0;

  const isEditingRefereeRebateRate = !!props.accountId;

  const noCommissionAvailable = props.maxRebatePercentage === 0;

  const referralCodeRuleText = `${t(
    "affiliate.referralCode.editCodeModal.helpText.length",
  )}. ${t("affiliate.referralCode.editCodeModal.helpText.format")}`;

  const { title, description, buttonText } = useMemo(() => {
    switch (type) {
      case ReferralCodeFormType.Create:
        return {
          title: isReview
            ? t("affiliate.referralCode.review.modal.title")
            : t("affiliate.referralCode.create.modal.title"),
          description: null,
          buttonText: isReview ? t("common.save") : t("affiliate.review"),
        };
      case ReferralCodeFormType.Edit:
        return {
          title: isEditingRefereeRebateRate
            ? t("affiliate.refereeRebateRate.modal.title", {
                accountId: formatAddress(props.accountId!),
              })
            : isReview
              ? t("affiliate.referralCode.review.modal.title")
              : t("affiliate.referralCode.edit.modal.title"),
          description: null,
          buttonText: isEditingRefereeRebateRate
            ? t("common.save")
            : isReview
              ? t("common.save")
              : t("affiliate.review"),
        };
      case ReferralCodeFormType.Reset:
        return {
          title: t("affiliate.resetRebateRate.modal.title"),
          description: (
            <Text size="2xs" intensity={54}>
              {t("affiliate.resetRebateRate.modal.description", {
                accountId: formatAddress(props.accountId!),
              })}
            </Text>
          ),
          buttonText: t("common.reset"),
        };
      default:
        return {
          title: "",
          description: "",
          buttonText: "",
        };
    }
  }, [t, type, isEditingRefereeRebateRate, isReview, props.accountId]);

  const titleView = (
    <Flex
      width={"100%"}
      direction="column"
      itemAlign="start"
      gap={3}
      className="oui-referralCodeForm-header"
    >
      <Text size="base" intensity={98}>
        {title}
      </Text>
      <Divider intensity={8} className="oui-w-full" />
    </Flex>
  );

  const referralCodeInput = (
    <ReferralCodeInput
      value={props.newCode}
      onChange={props.setNewCode}
      autoFocus={
        !isReview &&
        (type === ReferralCodeFormType.Create ||
          props.focusField === ReferralCodeFormField.ReferralCode)
      }
      disabled={isReview || hasBoundReferee}
      label={
        <span className="oui-inline-flex oui-items-center oui-gap-1">
          {t("affiliate.referralCode.editCodeModal.label")}
          <Tips
            content={referralCodeRuleText}
            title={t("affiliate.referralCode.editCodeModal.label")}
          />
        </span>
      }
      placeholder={t("affiliate.referralCode.create.input.placeholder")}
    />
  );

  const rateEditor = (
    <CommissionRatesCard
      directTradesRate={props.directTradesPercentage}
      indirectTradesRate={props.indirectTradesPercentage}
      totalSplitRate={props.totalSplitPercentage}
      youKeepRate={props.referrerRebatePercentage}
      refereeGetRate={props.refereeRebatePercentage}
      baseRebateRate={props.baseRebatePercentage}
      maxBonusRate={props.maxRebatePercentage}
      onChange={props.setReferrerRebatePercentage}
      showTradeRates={!isEditingRefereeRebateRate}
      showSlider={!isReview && !isReset}
      bordered={isEditingRefereeRebateRate ? false : true}
      rightLabel={
        isEditingRefereeRebateRate
          ? t("affiliate.affiliateGet")
          : t("affiliate.directRefereesGet")
      }
    />
  );

  const refereeInfo = (
    <Flex width={"100%"} justify="between" gap={2}>
      <Text size="2xs" intensity={54}>
        {t("affiliate.directReferee")}
      </Text>

      <Text.formatted rule="address" size="2xs" intensity={98}>
        {props.accountId}
      </Text.formatted>
    </Flex>
  );

  const resetRebateRateLabel = (
    <Text size="2xs" intensity={98} className="oui-text-start">
      {t("affiliate.resetRebateRate.rateAfterReset")}:
    </Text>
  );

  const buttons = (
    <Flex direction={"row"} gap={3} width={"100%"} mt={0} pt={2}>
      <Button
        variant="contained"
        color="gray"
        fullWidth
        type="button"
        onClick={props.close}
        size="md"
        className="oui-referralCodeForm-cancel-btn"
      >
        {t("common.cancel")}
      </Button>
      <Button
        fullWidth
        type="submit"
        disabled={props.buttonDisabled || props.confirmButtonLoading}
        loading={props.confirmButtonLoading}
        size="md"
        className="oui-referralCodeForm-confirm-btn"
      >
        {buttonText}
      </Button>
    </Flex>
  );

  const renderContent = () => {
    switch (type) {
      case ReferralCodeFormType.Create:
        return (
          <Flex
            width={"100%"}
            direction="column"
            itemAlign="start"
            gap={6}
            className={surface.surface}
          >
            {referralCodeInput}
            {!noCommissionAvailable && rateEditor}
            {buttons}
          </Flex>
        );
      case ReferralCodeFormType.Edit:
        return (
          <Flex width={"100%"} direction="column" itemAlign="start" gap={6}>
            {isEditingRefereeRebateRate
              ? refereeInfo
              : !hasBoundReferee && referralCodeInput}
            {!noCommissionAvailable && rateEditor}
            {buttons}
          </Flex>
        );
      case ReferralCodeFormType.Reset:
        return (
          <Flex width={"100%"} direction="column" itemAlign="start" gap={6}>
            {description}
            {resetRebateRateLabel}
            <RateSplitValues
              youKeepRate={props.referrerRebatePercentage}
              refereeGetRate={props.refereeRebatePercentage}
              rightLabel={t("affiliate.directRefereesGet")}
            />
            {buttons}
          </Flex>
        );
      default:
        return null;
    }
  };

  return (
    <form
      className="oui-w-full"
      onSubmit={(event) => {
        event.preventDefault();
        if (!props.buttonDisabled && !props.confirmButtonLoading)
          void props.onClick();
      }}
    >
      <Flex
        direction="column"
        itemAlign="start"
        gap={6}
        className={`${surface.surface} oui-affiliate-referralCodeForm oui-font-semibold`}
      >
        {titleView}
        {renderContent()}
      </Flex>
    </form>
  );
};

const formatPercent = (value: number) => {
  if (Number.isInteger(value)) {
    return `${value}%`;
  }
  return `${Number(value.toFixed(2))}%`;
};

type CommissionRatesCardProps = {
  directTradesRate: number;
  indirectTradesRate: number;
  totalSplitRate: number;
  youKeepRate: number;
  refereeGetRate: number;
  baseRebateRate: number;
  maxBonusRate: number;
  onChange: (value: number) => void;
  showTradeRates: boolean;
  showSlider: boolean;
  bordered: boolean;
  rightLabel: string;
};

const CommissionRatesCard = (props: CommissionRatesCardProps) => {
  const { t } = useTranslation();
  const id = useId();

  return (
    <Flex width={"100%"} direction="column" itemAlign="start" gap={2}>
      {props.showTradeRates && (
        <>
          <Text size="2xs" intensity={54}>
            {t("affiliate.yourCommissionRates")}
          </Text>
          <RateRow
            label={t("affiliate.directTrades")}
            tooltip={t("affiliate.directTradesDescription")}
            value={props.directTradesRate}
          />
        </>
      )}

      <Flex
        width={"100%"}
        direction="column"
        itemAlign="start"
        gap={2}
        p={props.bordered ? 2 : 0}
        r="lg"
        className={props.bordered ? "oui-border oui-border-line-6" : ""}
      >
        {props.showTradeRates && (
          <>
            <RateRow
              label={t("affiliate.indirectTrades")}
              tooltip={t("affiliate.indirectTradesDescription")}
              value={props.indirectTradesRate}
              bordered={false}
            />
            <Divider intensity={8} className="oui-w-full" />
          </>
        )}

        <Text size="2xs" intensity={54} className="oui-w-full oui-text-start">
          {t("affiliate.totalToSplit")}:{" "}
          <Text as="span" intensity={98}>
            {formatPercent(props.totalSplitRate)}
          </Text>
        </Text>

        {props.showSlider && (
          <Box width={"100%"} my={1} className="oui-relative">
            <label htmlFor={id} className="oui-sr-only">
              {t("affiliate.youKeep")}
            </label>
            <input
              id={id}
              name="referrer_rebate_percentage"
              type="range"
              min={0}
              max={Math.min(props.totalSplitRate, props.maxBonusRate)}
              step={1}
              value={props.youKeepRate}
              aria-valuetext={formatPercent(props.youKeepRate)}
              onChange={(event) => props.onChange(Number(event.target.value))}
              className="oui-w-full oui-accent-success"
            />
          </Box>
        )}

        <RateSplitValues
          youKeepRate={props.youKeepRate}
          refereeGetRate={props.refereeGetRate}
          rightLabel={props.rightLabel}
        />

        <PassDownHint amount={props.baseRebateRate} />
      </Flex>
    </Flex>
  );
};

const RateRow = (props: {
  label: string;
  tooltip?: ReactNode;
  value: number;
  bordered?: boolean;
}) => {
  const label = (
    <button
      type="button"
      className="oui-text-2xs oui-text-content oui-underline oui-decoration-dotted oui-underline-offset-4"
    >
      {props.label}
    </button>
  );

  return (
    <Flex
      width={"100%"}
      justify="between"
      itemAlign="center"
      gap={2}
      p={props.bordered === false ? 0 : 2}
      r="lg"
      className={props.bordered === false ? "" : "oui-border oui-border-line-6"}
    >
      {props.tooltip ? (
        <Tooltip
          content={props.tooltip}
          className="oui-max-w-[calc(var(--oui-space-24)*2)] oui-bg-base-6"
          arrow={{ className: "oui-fill-base-6" }}
        >
          {label}
        </Tooltip>
      ) : (
        label
      )}
      <Text size="lg" className="oui-text-success-darken">
        {formatPercent(props.value)}
      </Text>
    </Flex>
  );
};

const RateSplitValues = (props: {
  youKeepRate: number;
  refereeGetRate: number;
  rightLabel: string;
}) => {
  const { t } = useTranslation();

  return (
    <Flex width={"100%"} gap={2}>
      <Flex
        direction="column"
        itemAlign="start"
        className="oui-min-w-0 oui-flex-1"
      >
        <Text size="2xs" intensity={54}>
          {t("affiliate.youKeep")}
        </Text>
        <Text size="lg" className="oui-text-success-darken">
          {formatPercent(props.youKeepRate)}
        </Text>
      </Flex>
      <Flex
        direction="column"
        itemAlign="end"
        className="oui-min-w-0 oui-flex-1"
      >
        <Text size="2xs" intensity={54} className="oui-text-right">
          {props.rightLabel}
        </Text>
        <Text size="lg" intensity={54} className="oui-text-right">
          {formatPercent(props.refereeGetRate)}
        </Text>
      </Flex>
    </Flex>
  );
};

const PassDownHint = (props: { amount: number }) => {
  const { t } = useTranslation();

  if (props.amount <= 0) {
    return null;
  }

  return (
    <Flex
      width={"100%"}
      itemAlign="center"
      gap={2}
      className="oui-overflow-hidden"
    >
      <div className="oui-h-3 oui-w-6 oui-shrink-0 oui-rounded-sm oui-border oui-border-line-6 oui-bg-base-contrast/10" />
      <Text size="2xs" intensity={36} className="oui-leading-normal">
        {t("affiliate.atLeastPassesDown", {
          amount: formatPercent(props.amount),
        })}
      </Text>
    </Flex>
  );
};
