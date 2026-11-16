import * as React from "react";
import { Button } from "@paxeer-network/paxeer-backend/ui/atoms/button";
import { Input } from "@paxeer-network/paxeer-backend/ui/atoms/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@paxeer-network/paxeer-backend/ui/atoms/dialog";
import { reportError } from "@paxeer-network/paxeer-backend/core/errors/report";
import type { UserFacingError } from "@paxeer-network/paxeer-backend/core/errors/types";

interface ReportModalState {
  error: UserFacingError;
  returnFocus: HTMLElement | null;
}

let state: ReportModalState | null = null;
const listeners = new Set<() => void>();

export const reportModalStore = {
  getSnapshot: () => state,
  subscribe(listener: () => void) {
    listeners.add(listener);
    return () => {
      listeners.delete(listener);
    };
  },
};

function notify() {
  listeners.forEach((listener) => listener());
}

export function openReportModal(error: UserFacingError): void {
  const activeElement =
    typeof document === "undefined" ? null : document.activeElement;
  state = {
    error,
    returnFocus: state
      ? state.returnFocus
      : typeof HTMLElement !== "undefined" &&
          activeElement instanceof HTMLElement
        ? activeElement
        : null,
  };
  notify();
}

export function closeReportModal(): void {
  state = null;
  notify();
}

function getServerSnapshot() {
  return null;
}

function ReportFeedbackDialog({ error, returnFocus }: ReportModalState) {
  const [note, setNote] = React.useState("");
  const noteId = React.useId();

  React.useEffect(() => {
    setNote("");
  }, [error]);

  function sendReport(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void reportError(error, { feedback: { comments: note.trim() } });
    closeReportModal();
  }

  return (
    <Dialog open onOpenChange={(open) => !open && closeReportModal()}>
      <DialogContent
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          if (returnFocus?.isConnected) returnFocus.focus();
        }}
      >
        <DialogHeader>
          <DialogTitle>{error.title}</DialogTitle>
          <DialogDescription>{error.message}</DialogDescription>
        </DialogHeader>
        <form onSubmit={sendReport} className="space-y-4">
          <div className="space-y-1.5">
            <label htmlFor={noteId} className="text-sm font-medium">
              Note (optional)
            </label>
            <Input
              id={noteId}
              value={note}
              onChange={(event) => setNote(event.target.value)}
              placeholder="What were you doing when this happened?"
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={closeReportModal}>
              Cancel
            </Button>
            <Button type="submit">Send report</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export function ReportFeedbackModalHost() {
  const current = React.useSyncExternalStore(
    reportModalStore.subscribe,
    reportModalStore.getSnapshot,
    getServerSnapshot,
  );
  return current ? <ReportFeedbackDialog {...current} /> : null;
}
