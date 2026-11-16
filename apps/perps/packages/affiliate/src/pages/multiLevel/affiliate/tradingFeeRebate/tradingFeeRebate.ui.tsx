import { useId, useState } from "react";
import surface from "../../../../surface.module.css";
import { useTranslation } from "@paxeer-network/i18n";
import { Button, Flex, Input, Text, Tooltip, cn } from "@paxeer-network/ui";
import type { TradingFeeRebateReturns } from "./tradingFeeRebate.script";
import type { TradingFeeRebateWidgetProps } from "./tradingFeeRebate.widget";

export type TradingFeeRebateProps = TradingFeeRebateReturns &
  TradingFeeRebateWidgetProps;

export const TradingFeeRebate = (props: TradingFeeRebateProps) => {
  const { t } = useTranslation();
  const id = useId();
  const [touched, setTouched] = useState(false);
  const showError = touched && props.showError;

  return (
    <form
      className={surface.surface}
      onSubmit={(event) => {
        event.preventDefault();
        setTouched(true);
        if (!props.buttonDisabled) void props.onSave();
      }}
    >
      <Flex
        direction="column"
        itemAlign="start"
        className={`${surface.surface} oui-affiliate-tradingFeeRebate oui-font-semibold`}
      >
        <Flex
          direction="column"
          itemAlign="start"
          gap={6}
          width="100%"
          className="oui-py-5"
        >
          <Text
            size="2xs"
            intensity={54}
            className="oui-w-full oui-leading-normal"
          >
            {t("affiliate.tradingFeeRebate.modal.description")}
          </Text>

          <Flex direction="column" itemAlign="start" gap={1} width="100%">
            <Text
              as="label"
              htmlFor={id}
              size="2xs"
              intensity={54}
              className="oui-w-full oui-break-words oui-leading-normal"
            >
              {t("affiliate.tradingFeeRebate.rate")}
            </Text>
            <Input
              id={id}
              name="trading_fee_rebate_rate"
              required
              pattern="[0-9]+"
              onBlur={() => setTouched(true)}
              onInvalid={() => setTouched(true)}
              aria-invalid={showError}
              aria-describedby={`${id}-help`}
              type="text"
              inputMode="numeric"
              autoComplete="on"
              fullWidth
              size="xl"
              value={props.percentage}
              onChange={(event) => props.onChange(event.target.value)}
              color={showError ? "danger" : undefined}
              className="oui-border oui-border-line"
              suffix={
                <Text intensity={54} className="oui-pe-3">
                  %
                </Text>
              }
            />
            <Text
              id={`${id}-help`}
              role="status"
              size="2xs"
              intensity={54}
              className={cn(
                "oui-w-full oui-break-words oui-leading-normal",
                showError && "oui-text-danger",
              )}
            >
              {t("affiliate.tradingFeeRebate.maximumAllowed", {
                rate: props.maxPercentage,
              })}
            </Text>
            <Flex
              direction="column"
              itemAlign="start"
              className="oui-affiliate-tradingFeeRebate-preview oui-mt-2 oui-w-full oui-gap-0 oui-break-words oui-leading-normal"
              aria-live="polite"
            >
              <Text size="2xs" intensity={54}>
                {t("affiliate.tradingFeeRebate.preview.receive")}
              </Text>
              <Tooltip
                className="oui-max-w-[calc(var(--oui-space-24)*3)]"
                content={t("affiliate.tradingFeeRebate.preview.basis", {
                  commissionRate: props.directTradesRate,
                })}
              >
                <Text
                  size="2xs"
                  intensity={98}
                  tabIndex={0}
                  className="oui-cursor-help oui-underline oui-decoration-dotted oui-underline-offset-1"
                >
                  {t("affiliate.tradingFeeRebate.preview.rebate", {
                    rate: props.tradingFeeRebateRate ?? "-",
                  })}
                </Text>
              </Tooltip>
            </Flex>
          </Flex>
        </Flex>

        <Flex width="100%" gap={3} className="oui-pb-5 oui-pt-3">
          <Button
            size="md"
            color="gray"
            fullWidth
            className="oui-rounded-md"
            type="button"
            onClick={props.close}
          >
            {t("common.cancel")}
          </Button>
          <Button
            size="md"
            fullWidth
            className="oui-rounded-md"
            disabled={props.buttonDisabled}
            loading={props.confirmButtonLoading}
            type="submit"
          >
            {t("common.save")}
          </Button>
        </Flex>
      </Flex>
    </form>
  );
};
