import * as React from "react";
import { Button } from "./button";
import { useToast } from "./use-toast";

export function Toaster() {
  const { toasts } = useToast();
  return (
    <section
      aria-label="Notifications"
      className="fixed bottom-4 right-4 z-50 w-full max-w-sm space-y-2"
    >
      {toasts
        .filter((toast) => toast.open)
        .map((toast) => (
          <div
            key={toast.id}
            role="alert"
            className={`space-y-2 rounded-[var(--radius-md)] p-4 shadow-2xl ${
              toast.variant === "destructive"
                ? "bg-destructive text-destructive-foreground"
                : "bg-surface-2 text-foreground"
            }`}
          >
            <div className="flex items-start justify-between gap-2">
              <div className="space-y-1">
                <div className="text-sm font-semibold">{toast.title}</div>
                <div className="text-sm">{toast.description}</div>
              </div>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                aria-label="Dismiss notification"
                onClick={() => toast.onOpenChange?.(false)}
              >
                Close
              </Button>
            </div>
            {toast.action}
          </div>
        ))}
    </section>
  );
}
