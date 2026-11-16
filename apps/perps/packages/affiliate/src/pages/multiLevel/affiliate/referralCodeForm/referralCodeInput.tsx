import { type ReactNode, useId, useState } from "react";
import {
  REFERRAL_CODE_MAX_LENGTH,
  REFERRAL_CODE_MIN_LENGTH,
} from "@paxeer-network/hooks";
import { useTranslation } from "@paxeer-network/i18n";
import { inputFormatter, Input } from "@paxeer-network/ui";

export type ReferralCodeInputProps = {
  value: string;
  onChange: (value: string) => void;
  autoFocus: boolean;
  disabled: boolean;
  label?: ReactNode;
  placeholder?: string;
  helpText?: string;
  color?: "danger";
};

export const ReferralCodeInput = (props: ReferralCodeInputProps) => {
  const id = useId();
  const { t } = useTranslation();
  const [touched, setTouched] = useState(false);
  const valid =
    props.value.length >= REFERRAL_CODE_MIN_LENGTH &&
    props.value.length <= REFERRAL_CODE_MAX_LENGTH &&
    /^[A-Z0-9]+$/.test(props.value);
  const invalid =
    touched && !props.disabled && (!valid || props.color === "danger");
  return (
    <div className="oui-grid oui-w-full oui-gap-2">
      <div id={`${id}-label`} className="oui-text-sm oui-text-content-muted">
        {props.label ?? (
          <label htmlFor={id}>{t("affiliate.referralCode")}</label>
        )}
      </div>
      <Input
        id={id}
        name="referral_code"
        aria-labelledby={`${id}-label`}
        type="text"
        fullWidth
        size="xl"
        required
        pattern="[A-Z0-9]+"
        placeholder={props.placeholder}
        value={props.value}
        onChange={(event) => props.onChange(event.target.value)}
        onBlur={() => setTouched(true)}
        onInvalid={() => setTouched(true)}
        formatters={[
          inputFormatter.createRegexInputFormatter((value: string | number) =>
            String(value).replace(/[a-z]/g, (char) => char.toUpperCase()),
          ),
          inputFormatter.createRegexInputFormatter(/[^A-Z0-9]/g),
        ]}
        aria-invalid={invalid}
        aria-describedby={invalid ? `${id}-error` : undefined}
        color={invalid ? "danger" : undefined}
        maxLength={REFERRAL_CODE_MAX_LENGTH}
        minLength={REFERRAL_CODE_MIN_LENGTH}
        autoComplete="on"
        autoCapitalize="none"
        autoCorrect="off"
        spellCheck={false}
        disabled={props.disabled}
        autoFocus={props.autoFocus}
      />
      {invalid && (
        <p
          id={`${id}-error`}
          className="oui-text-sm oui-text-danger"
          role="status"
        >
          {props.helpText ??
            t("affiliate.referralCode.editCodeModal.helpText.length")}
        </p>
      )}
    </div>
  );
};
