import { FC, useId } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { PnLDisplayFormat } from "../../types/types";

export const PnlFormatView: FC<{
  type: PnLDisplayFormat;
  curType?: PnLDisplayFormat;
  setPnlFormat: (format: PnLDisplayFormat) => void;
  name?: string;
}> = ({ type, curType, setPnlFormat, name = "pnl-format" }) => {
  const { t } = useTranslation();
  const id = useId();
  const text = t(
    type === "roi_pnl"
      ? "share.pnl.displayFormat.roi&Pnl"
      : type === "roi"
        ? "share.pnl.displayFormat.roi"
        : "share.pnl.displayFormat.pnl",
  );
  return (
    <label
      htmlFor={id}
      className="oui-flex oui-min-h-12 oui-items-center oui-gap-2 oui-text-sm oui-text-base-contrast oui-cursor-pointer"
    >
      <input
        id={id}
        type="radio"
        name={name}
        value={type}
        checked={type === curType}
        onChange={() => setPnlFormat(type)}
        className="oui-size-5 oui-shrink-0 oui-accent-primary"
      />
      {text}
    </label>
  );
};
