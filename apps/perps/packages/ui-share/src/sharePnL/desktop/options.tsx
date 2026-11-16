import { Dispatch, FC, SetStateAction, useId } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { ShareOptions } from "../../types/types";

export const ShareOption: FC<{
  type: ShareOptions;
  curType: Set<ShareOptions>;
  setShareOption: Dispatch<SetStateAction<Set<ShareOptions>>>;
}> = ({ type, curType, setShareOption }) => {
  const { t } = useTranslation();
  const id = useId();
  const text = t(
    type === "markPrice"
      ? "common.markPrice"
      : type === "quantity"
        ? "common.quantity"
        : type === "leverage"
          ? "common.leverage"
          : `share.pnl.optionalInfo.${type}`,
  );
  return (
    <label
      htmlFor={id}
      className="oui-flex oui-min-h-12 oui-items-center oui-gap-2 oui-text-sm oui-text-base-contrast oui-cursor-pointer"
    >
      <input
        id={id}
        type="checkbox"
        name={type}
        checked={curType.has(type)}
        onChange={(event) => {
          const checked = event.target.checked;
          setShareOption((value) => {
            const next = new Set(value);
            if (checked) next.add(type);
            else next.delete(type);
            return next;
          });
        }}
        className="oui-size-5 oui-shrink-0 oui-accent-primary"
      />
      {text}
    </label>
  );
};
