import { useId } from "react";
import { useTranslation } from "@paxeer-network/i18n";

export function PeriodSelect(props: {
  value?: string;
  onValueChange?: (value: string) => void;
  options: { label: string; value: string }[];
  size?: "xs";
  label?: string;
}) {
  const id = useId();
  const { t } = useTranslation();
  return (
    <div className="oui-grid oui-gap-1">
      <label htmlFor={id} className="oui-text-2xs oui-text-content-muted">
        {props.label ?? t("common.date")}
      </label>
      <select
        id={id}
        name={id}
        value={props.value}
        onChange={(event) => props.onValueChange?.(event.target.value)}
        className="oui-w-full oui-rounded-md oui-border oui-border-line oui-bg-base-8 oui-px-3 oui-py-2 oui-text-sm oui-text-content"
      >
        {props.options.map(({ label, value }) => (
          <option key={value} value={value}>
            {label}
          </option>
        ))}
      </select>
    </div>
  );
}
