import {
  ComponentType,
  PropsWithChildren,
  useEffect,
  useId,
  useRef,
  useState,
} from "react";
import { useTranslation } from "@paxeer-network/i18n";
import {
  Button,
  cn,
  modal,
  SimpleDialogProps,
  useModal,
} from "@paxeer-network/ui";
import surface from "../surface.module.css";

export const AffiliateDialog = (
  props: PropsWithChildren<
    Pick<
      SimpleDialogProps,
      "open" | "onOpenChange" | "title" | "actions" | "classNames"
    >
  >,
) => {
  const ref = useRef<HTMLDialogElement>(null);
  const id = useId();
  const [submitting, setSubmitting] = useState(false);
  const { t } = useTranslation();
  const title = typeof props.title === "function" ? props.title() : props.title;
  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (props.open && !dialog.open) dialog.showModal();
    if (!props.open && dialog.open) dialog.close();
  }, [props.open]);
  return (
    <dialog
      ref={ref}
      aria-labelledby={title ? id : undefined}
      aria-label={title ? undefined : t("affiliate.referralCode")}
      className={cn(surface.surface, surface.dialog, props.classNames?.content)}
      onClose={() => props.onOpenChange?.(false)}
      onClick={(event) => {
        if (event.target !== event.currentTarget) return;
        const rect = event.currentTarget.getBoundingClientRect();
        if (
          event.clientX < rect.left ||
          event.clientX > rect.right ||
          event.clientY < rect.top ||
          event.clientY > rect.bottom
        )
          event.currentTarget.close();
      }}
    >
      <div className="oui-flex oui-justify-end">
        <Button
          type="button"
          variant="text"
          color="secondary"
          aria-label={t("common.close")}
          onClick={() => ref.current?.close()}
        >
          ×
        </Button>
      </div>
      {title && (
        <h2 id={id} className="oui-mb-4 oui-text-lg oui-font-semibold">
          {title}
        </h2>
      )}
      {props.actions ? (
        <form
          onSubmit={async (event) => {
            event.preventDefault();
            const action = props.actions?.primary;
            if (!action || action.disabled || action.loading || submitting)
              return;
            setSubmitting(true);
            try {
              await action.onClick(event);
            } finally {
              setSubmitting(false);
            }
          }}
        >
          <div className={props.classNames?.body}>{props.children}</div>
          <div
            className={cn(
              "oui-mt-4 oui-flex oui-gap-3",
              props.classNames?.footer,
            )}
          >
            {props.actions.secondary && (
              <Button
                type="button"
                fullWidth
                color="gray"
                disabled={props.actions.secondary.disabled}
                onClick={props.actions.secondary.onClick}
              >
                {props.actions.secondary.label}
              </Button>
            )}
            {props.actions.primary && (
              <Button
                type="submit"
                fullWidth
                disabled={props.actions.primary.disabled || submitting}
                loading={props.actions.primary.loading || submitting}
              >
                {props.actions.primary.label}
              </Button>
            )}
          </div>
        </form>
      ) : (
        <div className={props.classNames?.body}>{props.children}</div>
      )}
    </dialog>
  );
};

export function registerAffiliateDialog<P extends { close?: () => void }>(
  id: string,
  Widget: ComponentType<P>,
  options?: Partial<SimpleDialogProps>,
) {
  const Component = modal.create((props: P) => {
    const { visible, hide, onOpenChange } = useModal();
    return (
      <AffiliateDialog {...options} open={visible} onOpenChange={onOpenChange}>
        <Widget {...props} close={hide} />
      </AffiliateDialog>
    );
  });
  modal.register(id, Component);
}
