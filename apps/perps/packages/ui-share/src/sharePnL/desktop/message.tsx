import { FC, useId } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { Input } from "@paxeer-network/ui";

export const Message: FC<{
  message: string;
  setMessage: (value: string) => void;
  check: boolean;
  setCheck: (value: boolean) => void;
}> = ({ message, setMessage, check, setCheck }) => {
  const { t } = useTranslation();
  const id = useId();
  return (
    <div className="oui-my-6 oui-grid oui-w-full oui-gap-2">
      <label
        htmlFor={`${id}-enabled`}
        className="oui-flex oui-min-h-12 oui-items-center oui-gap-2 oui-text-sm oui-text-base-contrast"
      >
        <input
          id={`${id}-enabled`}
          type="checkbox"
          checked={check}
          onChange={(event) => setCheck(event.target.checked)}
          className="oui-size-5 oui-accent-primary"
        />
        {t("share.pnl.optionalInfo.message")}
      </label>
      <label htmlFor={id} className="oui-sr-only">
        {t("share.pnl.optionalInfo.message")}
      </label>
      <Input
        id={id}
        name="message"
        autoComplete="on"
        maxLength={25}
        placeholder={t("share.pnl.optionalInfo.message.placeholder")}
        value={message}
        size="xl"
        fullWidth
        onChange={(event) => {
          setMessage(event.target.value);
          setCheck(event.target.value.length > 0);
        }}
      />
    </div>
  );
};
