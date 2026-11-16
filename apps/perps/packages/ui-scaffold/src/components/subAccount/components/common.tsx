import { useId } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { inputFormatter, Input, Text } from "@paxeer-network/ui";

export const NickNameDescriptionText = "";

export const NickNameTextField = (props: {
  nickName: string | undefined;
  setNickName: (nickName: string | undefined) => void;
  subAccountCount?: number;
  invalid?: boolean;
  onBlur?: () => void;
}) => {
  const { t } = useTranslation();
  const id = useId();
  return (
    <div className="oui-mb-4 oui-flex oui-flex-col oui-gap-2">
      <label htmlFor={id} className="oui-text-xs oui-text-base-contrast-80">
        {t("subAccount.modal.nickName.label")}
      </label>
      <Input
        id={id}
        ref={(input) => input?.setAttribute("autofocus", "")}
        name="account-nickname"
        size="xl"
        required
        placeholder={`Sub-account ${(props.subAccountCount ?? 0) + 1}`}
        value={props.nickName}
        onChange={(event) =>
          props.setNickName(
            event.target.value.replace(/[^a-zA-Z0-9@,\s_-]/g, ""),
          )
        }
        onBlur={props.onBlur}
        onInvalid={() => props.onBlur?.()}
        aria-invalid={!!props.invalid}
        aria-describedby={`${id}-hint`}
        formatters={[
          inputFormatter.createRegexInputFormatter(/[^a-zA-Z0-9@,\s_-]/g),
        ]}
        maxLength={20}
        minLength={1}
        autoComplete="off"
        color={props.invalid ? "danger" : undefined}
      />
      <Text id={`${id}-hint`} size="2xs" className="oui-text-base-contrast-80">
        {t("subAccount.modal.create.nickname.role")}
      </Text>
    </div>
  );
};
