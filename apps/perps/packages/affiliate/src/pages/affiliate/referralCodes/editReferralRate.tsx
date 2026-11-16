import { useEffect, useId, useState } from "react";
import { cleanStringStyle, useMutation } from "@paxeer-network/hooks";
import { useTranslation } from "@paxeer-network/i18n";
import { Button, Input, modal, toast, useModal } from "@paxeer-network/ui";
import { Decimal } from "@paxeer-network/utils";
import { AffiliateDialog } from "../../../components/dialog";
import { ReferralCodeType } from "./referralCodes.script";

export const EditReferralRate = modal.create<{
  code: ReferralCodeType;
  mutate: () => unknown;
}>(({ code, mutate }) => {
  const { visible, onOpenChange, hide } = useModal();
  const { t } = useTranslation();
  const id = useId();
  const maxRate = new Decimal(code.max_rebate_rate).mul(100);
  const [referrerRate, setReferrerRate] = useState(() =>
    new Decimal(code.referrer_rebate_rate).mul(100).toString(),
  );
  const [refereeRate, setRefereeRate] = useState(() =>
    new Decimal(code.referee_rebate_rate).mul(100).toString(),
  );
  const [touched, setTouched] = useState(false);
  useEffect(() => {
    setReferrerRate(new Decimal(code.referrer_rebate_rate).mul(100).toString());
    setRefereeRate(new Decimal(code.referee_rebate_rate).mul(100).toString());
    setTouched(false);
  }, [code]);
  const [editRate, { isMutating }] = useMutation(
    "/v1/referral/edit_split",
    "POST",
  );
  const valid = [referrerRate, refereeRate].every(
    (value) =>
      value.trim() !== "" &&
      Number.isFinite(Number(value)) &&
      Number(value) >= 0 &&
      Number(value) <= maxRate.toNumber(),
  );
  const showError = touched && !valid;
  const updateRate = (value: string, isReferrer: boolean) => {
    const text = cleanStringStyle(value);
    const setValue = isReferrer ? setReferrerRate : setRefereeRate;
    const setOther = isReferrer ? setRefereeRate : setReferrerRate;
    setValue(text);
    if (text === "" || !Number.isFinite(Number(text))) {
      setOther("");
    } else {
      const remainder = maxRate.sub(text);
      setOther(remainder.isNegative() ? "0" : remainder.toString());
    }
  };
  const confirm = async () => {
    setTouched(true);
    if (!valid) return;
    try {
      await editRate({
        referral_code: code.code,
        referee_rebate_rate: Number(refereeRate) / 100,
        referrer_rebate_rate: Number(referrerRate) / 100,
      });
      toast.success(t("affiliate.referralRate.editRateModal.success"));
      await mutate();
      hide();
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t("common.somethingWentWrong"),
      );
    }
  };
  return (
    <AffiliateDialog
      open={visible}
      onOpenChange={onOpenChange}
      title={t("affiliate.referralRate.editRateModal.title")}
    >
      <form
        className="oui-grid oui-gap-4"
        onSubmit={(event) => {
          event.preventDefault();
          void confirm();
        }}
      >
        <p className="oui-text-sm oui-text-content-muted">
          {t("affiliate.referralRate.editRateModal.description")}
        </p>
        <p className="oui-text-sm">
          {t("affiliate.referralRate.editRateModal.label")}{" "}
          <span className="oui-text-warning-darken">
            {maxRate.toFixed(0, Decimal.ROUND_DOWN)}%
          </span>
        </p>
        {([true, false] as const).map((isReferrer) => {
          const inputId = `${id}-${isReferrer ? "referrer" : "referee"}`;
          return (
            <div key={inputId} className="oui-grid oui-gap-2">
              <label
                htmlFor={inputId}
                className="oui-text-sm oui-text-content-muted"
              >
                {t(
                  isReferrer
                    ? "affiliate.referralRate.editRateModal.label.you"
                    : "affiliate.referralRate.editRateModal.label.referee",
                )}
              </label>
              <Input
                id={inputId}
                name={
                  isReferrer ? "referrer_rebate_rate" : "referee_rebate_rate"
                }
                type="text"
                inputMode="decimal"
                autoComplete="on"
                required
                size="xl"
                fullWidth
                value={isReferrer ? referrerRate : refereeRate}
                onChange={(event) => updateRate(event.target.value, isReferrer)}
                onBlur={() => setTouched(true)}
                onInvalid={() => setTouched(true)}
                aria-invalid={showError}
                aria-describedby={showError ? `${id}-error` : undefined}
                color={showError ? "danger" : undefined}
                suffix={
                  <span className="oui-px-3" aria-hidden="true">
                    %
                  </span>
                }
              />
            </div>
          );
        })}
        {showError && (
          <p
            id={`${id}-error`}
            role="status"
            className="oui-text-sm oui-text-danger"
          >
            {t("affiliate.referralRate.editRateModal.helpText.max")}
          </p>
        )}
        <Button
          type="submit"
          fullWidth
          disabled={!valid || isMutating}
          loading={isMutating}
        >
          {t("common.confirm")}
        </Button>
      </form>
    </AffiliateDialog>
  );
});
