import React, { useEffect, useRef } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "@paxeer-network/i18n";

interface DrawerProps {
  children: React.ReactNode;
  isOpen: boolean;
  onClose: () => void;
}

export function Drawer({ children, isOpen, onClose }: DrawerProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const { t } = useTranslation();

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (isOpen && !dialog.open) dialog.showModal();
    else if (!isOpen && dialog.open) dialog.close();
  }, [isOpen]);

  if (typeof document === "undefined") return null;

  return createPortal(
    <dialog
      ref={dialogRef}
      aria-label={t("connector.connectWallet")}
      className="oui-wallet-drawer"
      {...{ closedby: "any" }}
      onClose={() => {
        if (isOpen) onClose();
      }}
    >
      <button
        type="button"
        className="oui-shell-icon-button oui-self-end"
        onClick={() => dialogRef.current?.close()}
        aria-label={t("common.close")}
      >
        <span aria-hidden="true">×</span>
      </button>
      {children}
    </dialog>,
    document.body,
  );
}
