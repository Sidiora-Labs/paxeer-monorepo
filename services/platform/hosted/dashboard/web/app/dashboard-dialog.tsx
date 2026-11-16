"use client";

import { useEffect, useId, useRef, type ReactNode } from "react";

export function DashboardDialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  footer,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  children?: ReactNode;
  footer: ReactNode;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const pressedOutside = useRef(false);
  const titleId = useId();
  const descriptionId = useId();

  useEffect(() => {
    const element = dialog.current;
    if (!element) return;
    if (open && !element.open) element.showModal();
    else if (!open && element.open) element.close();
  }, [open]);

  const outside = (clientX: number, clientY: number) => {
    const bounds = dialog.current?.getBoundingClientRect();
    return (
      bounds !== undefined &&
      (clientX < bounds.left ||
        clientX > bounds.right ||
        clientY < bounds.top ||
        clientY > bounds.bottom)
    );
  };

  return (
    <dialog
      ref={dialog}
      className="dashboard-dialog"
      aria-labelledby={titleId}
      aria-describedby={descriptionId}
      onCancel={(event) => {
        event.preventDefault();
        onOpenChange(false);
      }}
      onClose={() => onOpenChange(false)}
      onPointerDown={(event) => {
        pressedOutside.current = outside(event.clientX, event.clientY);
      }}
      onPointerUp={(event) => {
        if (pressedOutside.current && outside(event.clientX, event.clientY)) {
          onOpenChange(false);
        }
        pressedOutside.current = false;
      }}
    >
      <div className="dashboard-dialog-body">
        <h2 id={titleId}>{title}</h2>
        <p id={descriptionId}>{description}</p>
        {children}
      </div>
      <div className="dashboard-dialog-footer">{footer}</div>
    </dialog>
  );
}
