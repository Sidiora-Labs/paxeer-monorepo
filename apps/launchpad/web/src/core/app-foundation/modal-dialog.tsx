'use client';

import { useEffect, useRef, useState, type ComponentPropsWithoutRef } from 'react';
import { createPortal } from 'react-dom';
import { useDialogFocus } from './use-dialog-focus';

interface ModalDialogProps extends Omit<ComponentPropsWithoutRef<'dialog'>, 'open' | 'onClose'> {
  open: boolean;
  onClose?: () => void;
  closeOnScrim?: boolean;
}

/** A modal in the browser's top layer, with focus containment for older engines. */
export function ModalDialog({
  open,
  onClose,
  closeOnScrim = true,
  children,
  className,
  ...props
}: ModalDialogProps) {
  const [mounted, setMounted] = useState(false);
  const ref = useRef<HTMLDialogElement>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;
  const scrimStart = useRef(false);
  const native =
    typeof HTMLDialogElement !== 'undefined' &&
    typeof HTMLDialogElement.prototype.showModal === 'function';
  useEffect(() => {
    setMounted(true);
  }, []);
  useDialogFocus(ref, mounted && open && !native);

  useEffect(() => {
    const dialog = ref.current;
    if (!mounted || !open || !dialog) return;
    const opener = document.activeElement;
    const preferred = dialog.querySelector<HTMLElement>('[data-autofocus]');
    const hadAutofocus = preferred?.hasAttribute('autofocus');
    preferred?.setAttribute('autofocus', '');
    if (native) dialog.showModal();
    else {
      dialog.setAttribute('open', '');
      preferred?.focus();
    }
    if (!hadAutofocus) preferred?.removeAttribute('autofocus');
    return () => {
      if (native) {
        if (dialog.open) dialog.close();
        if (opener instanceof HTMLElement && opener.isConnected) opener.focus();
      }
    };
  }, [mounted, open, native]);

  if (!mounted || !open) return null;
  const outside = (
    event: React.PointerEvent<HTMLDialogElement> | React.MouseEvent<HTMLDialogElement>
  ) => {
    if (event.target !== event.currentTarget) return false;
    const bounds = event.currentTarget.getBoundingClientRect();
    return (
      event.clientX < bounds.left ||
      event.clientX > bounds.right ||
      event.clientY < bounds.top ||
      event.clientY > bounds.bottom
    );
  };
  return createPortal(
    <>
      {!native ? (
        <div
          aria-hidden="true"
          className="fixed inset-0 z-[var(--sl-z-modal)] bg-[var(--sl-tint-scrim)]"
          onClick={() => {
            if (closeOnScrim) closeRef.current?.();
          }}
        />
      ) : null}
      <dialog
        {...props}
        ref={ref}
        open={native ? undefined : true}
        role="dialog"
        aria-modal="true"
        className={`sl-native-dialog ${className ?? ''}`}
        onCancel={(event) => {
          event.preventDefault();
          closeRef.current?.();
        }}
        onKeyDown={(event) => {
          props.onKeyDown?.(event);
          if (!native && event.key === 'Escape') {
            event.preventDefault();
            event.stopPropagation();
            closeRef.current?.();
          }
        }}
        onPointerDown={(event) => {
          scrimStart.current = outside(event);
        }}
        onClick={(event) => {
          if (closeOnScrim && scrimStart.current && outside(event)) closeRef.current?.();
          scrimStart.current = false;
        }}
      >
        {children}
      </dialog>
    </>,
    document.body
  );
}
