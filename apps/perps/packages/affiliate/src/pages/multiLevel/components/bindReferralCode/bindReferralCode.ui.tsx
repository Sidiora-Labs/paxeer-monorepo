import surface from "../../../../surface.module.css";
import { REFERRAL_CODE_MIN_LENGTH } from "@paxeer-network/hooks";
import { useTranslation } from "@paxeer-network/i18n";
import { Button, Divider, Flex, Text } from "@paxeer-network/ui";
import { ReferralCodeInput } from "../../affiliate/referralCodeForm/referralCodeInput";
import { WarningBox } from "../warningBox";
import { BindReferralCodeScriptReturns } from "./bindReferralCode.script";
import { BindReferralCodeWidgetProps } from "./bindReferralCode.widget";

export type BindReferralCodeProps = BindReferralCodeScriptReturns &
  BindReferralCodeWidgetProps;

export const BindReferralCode = (props: BindReferralCodeProps) => {
  const { t } = useTranslation();

  const bindCodeInvalid =
    !props.skipBinding &&
    props.formattedBindCode.length >= REFERRAL_CODE_MIN_LENGTH &&
    !props.isBindCodeChecking &&
    props.isBindCodeExist === false;

  const titleView = (
    <Flex
      width={"100%"}
      direction="column"
      itemAlign="start"
      gap={3}
      className="oui-bindReferralCode-header"
    >
      <Text size="base" intensity={98}>
        {t("affiliate.referralCode.bind.modal.title")}
      </Text>
      <Divider intensity={8} className="oui-w-full" />
    </Flex>
  );

  const descriptionView = (
    <WarningBox>
      <Text
        size="2xs"
        intensity={54}
        className="oui-leading-normal oui-text-warning-darken"
      >
        {t("affiliate.referralCode.bind.modal.description")}
      </Text>
    </WarningBox>
  );

  return (
    <form
      className="oui-w-full"
      onSubmit={(event) => {
        event.preventDefault();
        if (!props.buttonDisabled && !props.confirmButtonLoading)
          void props.onConfirm();
      }}
    >
      <Flex
        direction="column"
        itemAlign="start"
        gap={6}
        className={`${surface.surface} oui-affiliate-bindReferralCode oui-font-semibold`}
      >
        {titleView}
        {descriptionView}

        <Flex width={"100%"} direction="column" itemAlign="start" gap={4}>
          <ReferralCodeInput
            value={props.bindCodeInput}
            onChange={props.setBindCodeInput}
            autoFocus
            disabled={props.skipBinding}
            placeholder={t("affiliate.referralCode.bind.input.placeholder")}
            helpText={
              bindCodeInvalid ? t("affiliate.referralCode.notExist") : undefined
            }
            color={bindCodeInvalid ? "danger" : undefined}
          />

          <label className="oui-inline-flex oui-min-h-12 oui-items-center oui-gap-2 oui-text-2xs oui-font-normal oui-text-content-muted">
            <input
              type="checkbox"
              id="oui-checkbox-bindReferralCode-skip"
              name="skip_referral_binding"
              checked={props.skipBinding}
              onChange={(event) => {
                props.setSkipBinding(event.target.checked);
                if (event.target.checked) props.setBindCodeInput("");
              }}
            />
            <span>{t("affiliate.referralCode.bind.skip")}</span>
          </label>

          <Flex direction={"row"} gap={2} width={"100%"} mt={0} pt={5}>
            <Button
              variant="contained"
              color="gray"
              fullWidth
              type="button"
              onClick={props.close}
              size="md"
              className="oui-bindReferralCode-cancel-btn"
            >
              {t("common.cancel")}
            </Button>
            <Button
              fullWidth
              type="submit"
              disabled={props.buttonDisabled || props.confirmButtonLoading}
              loading={props.confirmButtonLoading}
              size="md"
              className="oui-bindReferralCode-confirm-btn"
            >
              {t("common.confirm")}
            </Button>
          </Flex>
        </Flex>
      </Flex>
    </form>
  );
};
