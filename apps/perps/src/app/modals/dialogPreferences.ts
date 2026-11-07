import { useEffect, useSyncExternalStore } from "react";

export const DIALOG_PREFERENCES_KEY = "hyperpax:dialog-preferences";
export interface DialogPreferenceRecord {
  version: 1;
  reduceMotion: boolean;
}
type PreferenceSnapshot = {
  reduceMotion: boolean;
  status: "loading" | "default" | "saved" | "error";
  error: string | null;
};
const initialSnapshot: PreferenceSnapshot = {
  reduceMotion: false,
  status: "loading",
  error: null,
};

export function decodeDialogPreferences(raw: string): DialogPreferenceRecord {
  const value: unknown = JSON.parse(raw);
  if (
    typeof value !== "object" ||
    value === null ||
    !("version" in value) ||
    value.version !== 1 ||
    !("reduceMotion" in value) ||
    typeof value.reduceMotion !== "boolean"
  )
    throw new Error("Unsupported dialog preference record");
  return { version: 1, reduceMotion: value.reduceMotion };
}

export function createDialogPreferenceStore(getStorage: () => Storage) {
  let snapshot = initialSnapshot;
  const listeners = new Set<() => void>();
  const publish = (next: PreferenceSnapshot) => {
    snapshot = next;
    for (const listener of listeners) listener();
  };
  const restore = () => {
    try {
      const raw = getStorage().getItem(DIALOG_PREFERENCES_KEY);
      const record = raw === null ? null : decodeDialogPreferences(raw);
      publish({
        reduceMotion: record?.reduceMotion ?? false,
        status: record ? "saved" : "default",
        error: null,
      });
    } catch {
      publish({
        ...snapshot,
        status: "error",
        error:
          "Dialog motion preference could not be restored. Choose a preference to save it again.",
      });
    }
  };
  return {
    getSnapshot: () => snapshot,
    getServerSnapshot: () => initialSnapshot,
    subscribe(listener: () => void) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    hydrate() {
      if (snapshot.status === "loading") restore();
    },
    restore,
    setReduceMotion(reduceMotion: boolean) {
      try {
        const record: DialogPreferenceRecord = { version: 1, reduceMotion };
        getStorage().setItem(DIALOG_PREFERENCES_KEY, JSON.stringify(record));
        publish({ reduceMotion, status: "saved", error: null });
      } catch {
        publish({
          ...snapshot,
          status: "error",
          error:
            "Dialog motion preference was not saved. Check browser storage and try again.",
        });
      }
    },
  };
}

const dialogPreferenceStore = createDialogPreferenceStore(
  () => window.localStorage,
);

export function useDialogPreferences() {
  const snapshot = useSyncExternalStore(
    dialogPreferenceStore.subscribe,
    dialogPreferenceStore.getSnapshot,
    dialogPreferenceStore.getServerSnapshot,
  );
  useEffect(() => {
    dialogPreferenceStore.hydrate();
    const onStorage = (event: StorageEvent) => {
      if (event.key === DIALOG_PREFERENCES_KEY || event.key === null)
        dialogPreferenceStore.restore();
    };
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, []);
  return {
    ...snapshot,
    setReduceMotion: dialogPreferenceStore.setReduceMotion,
  };
}
