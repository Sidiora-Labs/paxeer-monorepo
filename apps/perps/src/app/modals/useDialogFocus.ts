import { useRef } from "react";

type FocusTarget = Pick<HTMLElement, "focus" | "isConnected">;

export function createDialogFocusLifecycle(
  getActiveElement: () => FocusTarget | null,
) {
  let opener: FocusTarget | null = null;
  return {
    onOpenAutoFocus() {
      opener = getActiveElement();
    },
    onCloseAutoFocus(event: Event) {
      if (opener?.isConnected) {
        event.preventDefault();
        opener.focus();
      }
      opener = null;
    },
  };
}

// Controlled dialogs are launched outside Dialog.Trigger, so Radix has no trigger ref.
export function useDialogFocus() {
  const lifecycle = useRef<ReturnType<
    typeof createDialogFocusLifecycle
  > | null>(null);
  if (!lifecycle.current) {
    lifecycle.current = createDialogFocusLifecycle(() =>
      document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null,
    );
  }
  return lifecycle.current;
}
