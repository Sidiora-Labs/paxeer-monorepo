import surface from "../../../surface.module.css";
import { AffiliateDialog } from "../../../components/dialog";
import { ReferralCodeInput } from "../../multiLevel/affiliate/referralCodeForm/referralCodeInput";
import { FC } from "react";
import {
  formatReferralCodeInput,
  isReferralCodeLengthValid,
  REFERRAL_CODE_MAX_LENGTH,
  REFERRAL_CODE_MIN_LENGTH,
} from "@paxeer-network/hooks";
import { useTranslation } from "@paxeer-network/i18n";
import { AccountStatusEnum } from "@paxeer-network/types";
import {
  Button,
  cn,
  Dialog,
  DialogBody,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  Divider,
  Flex,
  inputFormatter,
  Text,
  TextField,
} from "@paxeer-network/ui";
import { AuthGuard } from "@paxeer-network/ui-connector";
import { commifyOptional } from "@paxeer-network/utils";
import { ArrowRightIcon } from "../../../icons/arrowRightIcon";
import { USDCIcon } from "../../../icons/usdcIcon";
import { AsTraderReturns } from "./asTrader.script";

export const AsTrader: FC<AsTraderReturns> = (props) => {
  const { t } = useTranslation();
  return (
    <Flex
      id="oui-affiliate-home-asTrader"
      gradient="success"
      r={"2xl"}
      p={6}
      gap={6}
      direction={"column"}
      angle={180}
      width={"100%"}
      justify={"between"}
      className={surface.surface}
    >
      <Flex
        height={"var(--oui-space-20)"}
        direction={"row"}
        gap={3}
        itemAlign={"start"}
        width={"100%"}
        justify={"between"}
      >
        <Flex
          direction={"column"}
          itemAlign={"start"}
          justify={"between"}
          className="oui-h-full"
        >
          <Text className="oui-text-lg md:oui-text-xl lg:oui-text-2xl xl:oui-text-3xl">
            {props.isTrader
              ? t("affiliate.trader")
              : t("affiliate.asTrader.title")}
          </Text>
          <Text
            className={cn(
              "oui-text-xs oui-text-base-contrast-54 md:oui-text-sm 2xl:oui-text-base",
              props.isTrader && "oui-hidden",
            )}
          >
            {t("affiliate.asTrader.description")}
          </Text>
        </Flex>
        <div className="oui-shrink-0">
          <Icon />
        </div>
      </Flex>
      <Bottom {...props} />
    </Flex>
  );
};

const Icon = () => {
  return (
    <svg
      width="72"
      height="72"
      viewBox="0 0 90 90"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className="xl:oui-size-24 "
    >
      <path
        d="M44.996 7.324c-20.71 0-37.43 16.79-37.5 37.5-.07 20.682 16.806 37.575 37.5 37.617 20.694.04 37.537-17.082 37.5-37.617-.037-20.71-16.79-37.5-37.5-37.5m0 7.5c16.569 0 30 13.433 30 30 0 8.438-3.514 16.043-9.12 21.495-2.762-5.973-8.694-10.245-15.49-10.245h-10.78c-6.791 0-12.706 4.22-15.47 10.197-5.606-5.453-9.14-13.01-9.14-21.447 0-16.567 13.431-30 30-30m0 7.5c-8.284 0-15 6.717-15 15s6.716 15 15 15 15-6.716 15-15-6.716-15-15-15"
        fill="rgb(var(--oui-color-base-foreground))"
        fillOpacity=".2"
      />
      <path
        d="M90 71.25C90 81.605 81.605 90 71.25 90S52.5 81.605 52.5 71.25 60.895 52.5 71.25 52.5 90 60.895 90 71.25"
        fill="rgb(var(--oui-color-success-darken))"
      />
      <path
        d="M80.62 75.007c0-.24-.08-.49-.263-.675l-3.487-3.456-1.318 1.318 1.845 1.875H63.745a.938.938 0 0 0 0 1.875h13.652l-1.845 1.875 1.318 1.318 3.487-3.456a.95.95 0 0 0 .263-.674m-.937-7.5a.94.94 0 0 0-.938-.938H65.093l1.846-1.875-1.319-1.318-3.486 3.456a.966.966 0 0 0 0 1.349l3.486 3.456 1.319-1.318-1.846-1.875h13.652c.518 0 .938-.42.938-.937"
        fill="rgb(var(--oui-color-base-foreground))"
        fillOpacity=".98"
      />
    </svg>
  );
};

const Bottom: FC<AsTraderReturns> = (props) => {
  const { t } = useTranslation();

  const content = () => {
    if (props.isTrader && !props.wrongNetwork) {
      const totalReferrerRebate =
        props.referralInfo?.referee_info?.total_referee_rebate;
      return (
        <>
          <Flex direction={"column"} itemAlign={"start"} gap={2}>
            <Text className="oui-text-2xs md:oui-text-xs xl:oui-text-sm">
              {t("affiliate.commission")} {<Text intensity={36}>(USDC)</Text>}
            </Text>
            <Flex direction={"row"} gap={1}>
              <USDCIcon />
              <Text className="oui-text-base md:oui-text-lg lg:oui-text-xl xl:oui-text-2xl">
                {commifyOptional(totalReferrerRebate, {
                  fix: 2,
                  fallback: "0",
                })}
              </Text>
            </Flex>
          </Flex>
          <button
            type="button"
            className="oui-inline-flex oui-items-center oui-justify-end oui-gap-1 oui-underline"
            onClick={props.onEnterTraderPage}
          >
            <Text className="oui-text-sm md:oui-text-base xl:oui-text-lg">
              {t("affiliate.enter")}
            </Text>
            <ArrowRightIcon className="md:oui-size-4 lg:oui-size-5 xl:oui-size-6" />
          </button>
        </>
      );
    }

    return <EntryCode {...props} />;
  };

  return (
    <Flex
      direction={"row"}
      justify={"between"}
      width={"100%"}
      itemAlign={"end"}
    >
      {content()}
    </Flex>
  );
};

const EntryCode: FC<AsTraderReturns> = (props) => {
  const { t } = useTranslation();
  const invalid =
    isReferralCodeLengthValid(props.code) && !props.isExist && !props.isLoading;
  return (
    <>
      <Button type="button" color="light" onClick={() => props.setOpen(true)}>
        {t("affiliate.asTrader.button")}
      </Button>
      <AffiliateDialog
        open={props.open}
        onOpenChange={props.setOpen}
        title={t("affiliate.referralCode.dialog.title")}
      >
        <form
          className="oui-grid oui-gap-4"
          onSubmit={(event) => {
            event.preventDefault();
            if (isReferralCodeLengthValid(props.code) && props.isExist)
              void props.onClickConfirm();
          }}
        >
          <p className="oui-text-sm oui-text-content-muted">
            {t("affiliate.referralCode.dialog.description")}
          </p>
          <ReferralCodeInput
            value={props.code}
            onChange={(value) => props.setCode(formatReferralCodeInput(value))}
            disabled={false}
            autoFocus
            label={t("affiliate.referralCode.label")}
            placeholder={t("affiliate.referralCode")}
            helpText={
              invalid ? t("affiliate.referralCode.notExist") : undefined
            }
            color={invalid ? "danger" : undefined}
          />
          {props.warningMessage && (
            <p className="oui-text-sm oui-text-warning-darken">
              {props.warningMessage}
            </p>
          )}
          <AuthGuard buttonProps={{ size: "md", fullWidth: true }}>
            <Button
              type="submit"
              fullWidth
              disabled={
                !isReferralCodeLengthValid(props.code) || !props.isExist
              }
            >
              {t("common.confirm")}
            </Button>
          </AuthGuard>
        </form>
      </AffiliateDialog>
    </>
  );
};
