"use client";

import * as React from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import { cn } from "../lib/utils";

export interface DrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  children: React.ReactNode;
  portalContainer?: HTMLElement | null;
  /** Width of the right-side panel. Defaults to the surface drawer token. */
  width?: number | string;
}

/**
 * Right-side drawer — the desktop detail/education surface.
 * Esc + overlay dismiss, focus-trapped.
 */
export function Drawer({ open, onOpenChange, children, portalContainer, width = "var(--drawer-width)" }: DrawerProps) {
  const widthClass = typeof width === "number"
    ? ({ 320: "w-[320px]", 400: "w-[400px]", 480: "w-[480px]", 560: "w-[560px]", 640: "w-[640px]", 720: "w-[720px]" } as Record<number, string>)[width] ?? "w-full"
    : "w-[min(var(--drawer-width),100vw)]";
  const contentRef = React.useRef<HTMLDialogElement>(null);
  React.useEffect(() => {
    const content = contentRef.current;
    if (content === null) return;
    if (open) {
      if (content.matches(":modal")) return;
      if (content.open) content.close();
      content.showModal();
    } else if (content.open) {
      content.close();
    }
  }, [open]);
  const setContentRef = React.useCallback(
    (content: HTMLDialogElement | null) => {
      contentRef.current = content;
      if (content !== null && open && !content.matches(":modal")) {
        content.showModal();
      }
    },
    [open],
  );
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal container={portalContainer ?? undefined}>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-overlay-subtle data-[state=open]:animate-fade-in data-[state=closed]:animate-fade-out" />
        <Dialog.Content asChild>
          <dialog
            ref={setContentRef}
            onCancel={(event) => {
              event.preventDefault();
              onOpenChange(false);
            }}
          className={cn(
            "fixed top-0 right-0 z-50 flex h-dvh flex-col overscroll-contain bg-surface lx-safe-top lx-safe-bottom shadow-overlay outline-none",
            widthClass,
            "data-[state=open]:animate-drawer-in data-[state=closed]:animate-drawer-out",
          )}
          >
            {children}
          </dialog>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

export function DrawerHeader({
  title,
  description,
  onClose,
  className,
}: {
  title: React.ReactNode;
  description?: React.ReactNode;
  onClose?: () => void;
  className?: string;
}) {
  return (
    <div className={cn("flex items-start justify-between gap-4 border-b border-border p-5", className)}>
      <div className="flex flex-col gap-1">
        <Dialog.Title className="text-lg font-bold text-foreground">{title}</Dialog.Title>
        {description && (
          <Dialog.Description asChild>
            <p className="text-sm text-muted-foreground">{description}</p>
          </Dialog.Description>
        )}
      </div>
      {onClose && (
        <button
          type="button"
          onClick={onClose}
          aria-label="Close"
          className="inline-flex size-11 shrink-0 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-surface-sunken"
        >
          <X className="size-4" />
        </button>
      )}
    </div>
  );
}

export function DrawerBody({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("lx-scroll flex-1 overflow-y-auto p-5", className)} {...props} />;
}

export function DrawerFooter({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cn("flex items-center justify-end gap-3 border-t border-border p-4", className)} {...props} />
  );
}
