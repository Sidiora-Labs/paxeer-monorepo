import { AffiliateDialog as SimpleDialog } from "../../../components/dialog";
import { useEffect, useId, useState } from "react";
import {
  formatReferralCodeInput,
  isReferralCodeLengthValid,
  REFERRAL_CODE_MAX_LENGTH,
  REFERRAL_CODE_MIN_LENGTH,
  useMutation,
} from "@paxeer-network/hooks";
import { useTranslation } from "@paxeer-network/i18n";
import {
  inputFormatter,
  modal,
  Input,
  Text,
  useModal,
  Flex,
  cn,
  toast,
} from "@paxeer-network/ui";
import { ReferralCodeType } from "./referralCodes.script";

export const EditCodeModal = modal.create<{
  code: ReferralCodeType;
  successCallback: () => void;
}>((props) => {
  const { t } = useTranslation();
  const { visible, onOpenChange } = useModal();
  const id = useId();
  const [touched, setTouched] = useState(false);
  const [newCode, setNewCode] = useState<string>("");
  const [filedError, setFiledError] = useState<{
    length: boolean;
    format: boolean;
  }>({
    length: false,
    format: false,
  });
  const [editCode, { error, isMutating }] = useMutation(
    "/v1/referral/edit_referral_code",
    "POST",
  );
  useEffect(() => {
    setNewCode(props.code.code);
    setTouched(false);
  }, [props.code]);

  useEffect(() => {
    const _code = newCode.toUpperCase();
    const _fieldError = {
      length: false,
      format: false,
    };
    if (!isReferralCodeLengthValid(_code)) {
      _fieldError.length = true;
    }
    if (!/^[A-Z0-9]+$/.test(_code)) {
      _fieldError.format = true;
    }
    setFiledError(_fieldError);
  }, [newCode]);

  return (
    <SimpleDialog
      classNames={{
        content: "oui-max-w-[calc(var(--oui-space-24)*4)]",
      }}
      title={<div>{t("affiliate.referralCode.editCodeModal.title")}</div>}
      open={visible}
      onOpenChange={onOpenChange}
      actions={{
        primary: {
          loading: isMutating,
          disabled: filedError.length || filedError.format,
          label: t("common.confirm"),
          onClick: async () => {
            try {
              const res = await editCode({
                current_referral_code: props.code.code,
                new_referral_code: formatReferralCodeInput(newCode),
              });
              if (res.success) {
                toast.success(
                  t("affiliate.referralCode.editCodeModal.success"),
                );
                props.successCallback();
                onOpenChange(false);
                return;
              }
              toast.error(res.message);
            } catch (e) {
              toast.error(
                e instanceof Error ? e.message : t("common.somethingWentWrong"),
              );
            }
          },
        },
      }}
    >
      <div className="oui-mb-6 oui-text-xs oui-text-base-contrast-54">
        {t("affiliate.referralCode.editCodeModal.description")}
      </div>
      <label
        htmlFor={id}
        className="oui-block oui-text-sm oui-text-content-muted"
      >
        {t("affiliate.referralCode.editCodeModal.label")}
      </label>
      <Input
        id={id}
        size="xl"
        required
        pattern="[A-Z0-9]+"
        aria-describedby={`${id}-help`}
        name="referral_code"
        onBlur={() => setTouched(true)}
        onInvalid={() => setTouched(true)}
        aria-invalid={touched && (filedError.length || filedError.format)}
        autoCapitalize="none"
        autoCorrect="off"
        spellCheck={false}
        placeholder=""
        fullWidth
        value={newCode}
        onChange={(e) => {
          setNewCode(formatReferralCodeInput(e.target.value));
        }}
        formatters={[
          inputFormatter.createRegexInputFormatter((value: string | number) => {
            return String(value).replace(/[a-z]/g, (char: string) =>
              char.toUpperCase(),
            );
          }),
          inputFormatter.createRegexInputFormatter(/[^A-Z0-9]/g),
        ]}
        classNames={{
          input:
            "placeholder:oui-text-base-contrast-20 placeholder:oui-text-sm",
        }}
        maxLength={REFERRAL_CODE_MAX_LENGTH}
        minLength={REFERRAL_CODE_MIN_LENGTH}
        autoComplete="on"
      />
      <Flex
        id={`${id}-help`}
        direction={"column"}
        justify={"center"}
        itemAlign={"start"}
        gap={1}
        className="oui-mt-3"
      >
        <Text
          className={cn(
            "oui-ms-4 oui-list-item oui-list-outside oui-list-disc oui-text-xs oui-text-base-contrast-36 marker:oui-text-3xs",
            touched && filedError.length
              ? "oui-text-danger"
              : "oui-text-content-muted",
          )}
        >
          {t("affiliate.referralCode.editCodeModal.helpText.length")}
        </Text>
        <Text
          className={cn(
            "oui-ms-4 oui-list-item oui-list-outside oui-list-disc oui-text-xs oui-text-base-contrast-36 marker:oui-text-3xs",
            "oui-text-content-muted",
          )}
        >
          {t("affiliate.referralCode.editCodeModal.helpText.format")}
        </Text>
      </Flex>
    </SimpleDialog>
  );
});
