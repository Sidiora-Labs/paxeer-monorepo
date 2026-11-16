import { FC, PropsWithChildren, useEffect, useId, useRef } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { cn, SimpleDialogFooter, SimpleDialogProps } from "@paxeer-network/ui";
import styles from "../../../presentation.module.css";

export const ApiDialog: FC<PropsWithChildren<SimpleDialogProps>> = (props) => {
  const dialog = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  const { t } = useTranslation();
  useEffect(() => {
    const element = dialog.current;
    if (!element) return;
    if (props.open && !element.open) element.showModal();
    if (!props.open && element.open) element.close();
  }, [props.open]);
  const title = typeof props.title === "function" ? props.title() : props.title;
  return (
    <dialog
      ref={dialog}
      aria-labelledby={titleId}
      onClose={() => props.onOpenChange?.(false)}
      onClick={(event) => {
        if (event.target !== event.currentTarget) return;
        const rect = event.currentTarget.getBoundingClientRect();
        if (
          event.clientX < rect.left ||
          event.clientX > rect.right ||
          event.clientY < rect.top ||
          event.clientY > rect.bottom
        ) {
          props.onOpenChange?.(false);
        }
      }}
      className={cn(styles.surface, styles.dialog, props.classNames?.content)}
    >
      <header className="oui-flex oui-items-center oui-justify-between oui-gap-4 oui-border-b oui-border-line-12 oui-pb-4">
        <h2 id={titleId} className="oui-text-lg oui-font-semibold">
          {title}
        </h2>
        {props.closable !== false && (
          <button
            type="button"
            aria-label={t("common.close")}
            onClick={() => props.onOpenChange?.(false)}
            className="oui-min-h-12 oui-min-w-12 oui-rounded hover:oui-bg-base-contrast-12"
          >
            <span aria-hidden="true">×</span>
          </button>
        )}
      </header>
      <div className={cn("oui-py-5", props.classNames?.body)}>
        {props.children}
      </div>
      {props.description}
      <SimpleDialogFooter
        actions={props.actions}
        className={cn("oui-flex-wrap oui-gap-3", props.classNames?.footer)}
      />
    </dialog>
  );
};
