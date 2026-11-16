import { ComponentType, useEffect, useId, useRef } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { Button, modal, useModal } from "@paxeer-network/ui";
import { SharePnLOptions, SharePnLParams } from "../types/types";
import surface from "../surface.module.css";

type ShareDialogProps = {
  hide?: () => void;
  pnl?: SharePnLOptions & SharePnLParams;
};

export function registerShareDialog(
  id: string,
  Content: ComponentType<ShareDialogProps>,
) {
  const Dialog = modal.create((props: ShareDialogProps) => {
    const { visible, hide, onOpenChange } = useModal();
    const { t } = useTranslation();
    const titleId = useId();
    const ref = useRef<HTMLDialogElement>(null);
    useEffect(() => {
      if (visible && !ref.current?.open) ref.current?.showModal();
      if (!visible && ref.current?.open) ref.current.close();
    }, [visible]);
    return (
      <dialog
        ref={ref}
        className={`${surface.surface} ${surface.dialog}`}
        aria-labelledby={titleId}
        onClose={() => onOpenChange(false)}
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
        <header className="oui-mb-4 oui-flex oui-items-center oui-justify-between oui-gap-3">
          <h2 id={titleId} className="oui-text-lg oui-font-semibold">
            {t("share.pnl.sharePnl")}
          </h2>
          <Button
            type="button"
            variant="text"
            color="secondary"
            aria-label={t("common.close")}
            onClick={() => ref.current?.close()}
          >
            ×
          </Button>
        </header>
        <Content {...props} hide={hide} />
      </dialog>
    );
  });
  modal.register(id, Dialog);
}
