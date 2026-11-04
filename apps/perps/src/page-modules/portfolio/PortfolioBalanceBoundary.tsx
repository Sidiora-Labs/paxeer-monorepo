import React from "react";
import type { PortfolioBalanceSnapshot } from "./balances";

export function PortfolioBalanceBoundary({
  snapshot,
  onRetry,
  children,
}: {
  snapshot: PortfolioBalanceSnapshot;
  onRetry: () => void;
  children: React.ReactNode;
}) {
  if (!snapshot.address || (snapshot.account.status === "ready" && snapshot.wallet.status === "ready")) {
    return <>{children}</>;
  }
  const failed = snapshot.account.status === "error" || snapshot.wallet.status === "error";
  const loading = snapshot.account.status === "loading" || snapshot.wallet.status === "loading";
  return (
    <section aria-label="Portfolio balances" aria-busy={loading} className="space-y-3 rounded-[var(--radius-lg)] bg-surface-1 p-4">
      <div role={failed ? "alert" : "status"}>
        <p>{failed ? "Portfolio balances are unavailable." : "Loading portfolio balances…"}</p>
        <p>
          {snapshot.account.status === "error" ? "Account balance could not be loaded." :
            snapshot.account.status === "loading" ? "Loading account balance…" : "Account balance loaded."}
        </p>
        <p>
          {snapshot.wallet.status === "error" ? "Wallet balance could not be loaded." :
            snapshot.wallet.status === "loading" ? "Loading wallet balance…" : "Wallet balance loaded."}
        </p>
      </div>
      {failed && (
        <button type="button" onClick={onRetry} disabled={loading} className="rounded-md border px-3 py-2">
          Retry balances
        </button>
      )}
    </section>
  );
}
