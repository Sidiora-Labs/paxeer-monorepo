import { useCallback, useEffect, useSyncExternalStore, useState } from "react";
import type { CollateralIdentity } from "@/hooks/perps/useAccount";
import {
  getPendingTransactions,
  hydratePendingTransactions,
  subscribePendingTransactions,
  waitForTx,
} from "@/hooks/perps/useContract";
import { classifyError } from "@/core/errors/classify";

let activeIdentity: CollateralIdentity | null = null;
const listeners = new Set<() => void>();
function notify() {
  for (const listener of listeners) listener();
}
function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function useCollateralRecovery(onSettled: () => void) {
  const active = useSyncExternalStore(
    subscribe,
    () => activeIdentity,
    () => null,
  );
  const pending = useSyncExternalStore(
    subscribePendingTransactions,
    getPendingTransactions,
    getPendingTransactions,
  );
  const [ready, setReady] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  const retryRecovery = useCallback(() => {
    setRetry((value) => value + 1);
  }, []);

  useEffect(() => {
    let alive = true;
    setReady(false);
    setFailure(null);
    const settle = async () => {
      try {
        hydratePendingTransactions();
        if (activeIdentity) {
          if (alive) setReady(true);
          return;
        }
        const records = getPendingTransactions();
        const results = await Promise.allSettled(records.map((record) => waitForTx(record.hash)));
        const rejected = results.find((result) => result.status === "rejected");
        if (rejected?.status === "rejected") throw rejected.reason;
        if (alive && records.length > 0) onSettled();
        if (alive) setReady(true);
      } catch (error) {
        if (alive) {
          const classified = classifyError(error, "account/recovery");
          setFailure(`${classified.message} ${classified.suggestedFix ?? ""}`);
        }
      }
    };
    void settle();
    const onStorage = () => {
      if (alive) retryRecovery();
    };
    window.addEventListener("storage", onStorage);
    return () => {
      alive = false;
      window.removeEventListener("storage", onStorage);
    };
  }, [active, retry, retryRecovery, onSettled]);

  function begin(identity: CollateralIdentity) {
    // Synchronous guards cover both handlers, remounts, and multiple mounted forms.
    hydratePendingTransactions();
    if (!ready || activeIdentity || getPendingTransactions().length > 0) return false;
    activeIdentity = identity;
    notify();
    return true;
  }
  function end() {
    activeIdentity = null;
    notify();
  }

  return {
    busy: !ready || active !== null || pending.length > 0,
    active,
    pending,
    failure,
    retryRecovery,
    begin,
    end,
  };
}
