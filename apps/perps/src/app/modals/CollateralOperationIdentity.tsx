import React from "react";
import type { CollateralIdentity } from "@/hooks/perps/useAccount";

export function CollateralOperationIdentity({ identity }: { identity: CollateralIdentity }) {
  return (
    <p className="text-xs text-muted-foreground break-all" role="status">
      Transfer account: {identity.address} · Chain {identity.chainId}
    </p>
  );
}
