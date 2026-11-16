import { PropsWithChildren, ReactNode, useId } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { InfoCircleIcon } from "@paxeer-network/ui";
import surface from "../surface.module.css";

export function InfoPopover({
  children,
  content,
  title,
}: PropsWithChildren<{ content: ReactNode; title?: ReactNode }>) {
  const id = useId();
  const { t } = useTranslation();
  return (
    <>
      <button
        type="button"
        aria-label={
          typeof title === "string"
            ? title
            : children
              ? undefined
              : t("common.details")
        }
        {...{ popovertarget: id }}
        aria-details={id}
        className={surface.infoTrigger}
        onClick={(event) => event.stopPropagation()}
      >
        {children ?? <InfoCircleIcon size={16} aria-hidden="true" />}
      </button>
      <div
        id={id}
        {...{ popover: "auto" }}
        className={`${surface.surface} ${surface.infoPopover}`}
      >
        {title && (
          <h3 className="oui-mb-2 oui-text-base oui-font-semibold">{title}</h3>
        )}
        <div className="oui-text-sm">{content}</div>
        <button
          type="button"
          {...{ popovertarget: id, popovertargetaction: "hide" }}
          className="oui-mt-3 oui-text-sm oui-underline"
        >
          {t("common.close")}
        </button>
      </div>
    </>
  );
}
