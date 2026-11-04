import React, { useState, useCallback, useMemo } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from "@/ui/atoms/dialog";
import { Button } from "@/ui/atoms/button";
import { Input } from "@/ui/atoms/input";
import { usePosition } from "@/hooks/perps/usePosition";
import { toast } from "@/ui/atoms/use-toast";
import { toastError } from "@/widgets/errors";
import { COLLATERAL_TOKENS, POSITION_ACTION_COPY } from "@/core/perps/config";
import { toCollateralUnits } from "@/core/perps/collateralInputs";
import { formatUnits, parseEther, zeroAddress } from "viem";
import { useQuery } from "@tanstack/react-query";
import { useAccount } from "wagmi";
import { getAccount } from "wagmi/actions";
import { wagmiConfig } from "@/core/wagmi";
import { paxeerChain } from "@/core/chain";
import { formatWadFixed } from "@/core/perps/pnlHelpers";
import {
  canRemoveCollateral,
  collateralRemovalQueryOptions,
  prepareCollateralRemoval,
  remainingRemovalEquity,
  type CollateralRemovalRisk,
} from "./collateralRemoval";

type AdjustMode = "add" | "remove" | "addSize";

interface AdjustCollateralModalProps {
  positionId: string;
  mode: AdjustMode;
  currentCollateral: number;
  currentSize: number;
  currentLeverage: number;
  onClose: () => void;
  onSuccess?: () => void;
}

const MODE_TITLES: Record<AdjustMode, string> = {
  add: POSITION_ACTION_COPY.addCollateral,
  remove: POSITION_ACTION_COPY.removeCollateral,
  addSize: POSITION_ACTION_COPY.addSize,
};

export function CollateralRemovalPreview({ risk, amount, loading }: {
  risk: CollateralRemovalRisk | undefined;
  amount: bigint;
  loading: boolean;
}) {
  if (!risk) {
    return <p role="status" className="text-xs text-muted-foreground">
      {loading ? "Loading current margin..." : "Current margin unavailable. Removal is disabled."}
    </p>;
  }
  const allowed = canRemoveCollateral(risk, amount);
  return (
    <div className="space-y-1 text-xs">
      <p>Maximum removal: ${formatUnits(risk.maxRemoval, COLLATERAL_TOKENS.usdl.decimals)}</p>
      <p>Market tier maintenance: ${formatWadFixed(risk.marketMaintenanceUsd, 2)}</p>
      <p>Account maintenance: ${formatWadFixed(risk.maintenanceMarginUsd, 2)}</p>
      <p className={allowed ? "text-emerald-500" : "text-red-500"}>
        Remaining account equity: ${formatWadFixed(remainingRemovalEquity(risk, amount), 2)}
      </p>
      {amount > 0n && !allowed && <p role="alert">{POSITION_ACTION_COPY.errorInsufficientMargin}</p>}
    </div>
  );
}

export function AdjustCollateralModal({
  positionId,
  mode,
  currentCollateral,
  currentSize,
  currentLeverage,
  onClose,
  onSuccess,
}: AdjustCollateralModalProps) {
  const [amount, setAmount] = useState("");
  const [leverage, setLeverage] = useState(currentLeverage.toString());
  const [isPending, setIsPending] = useState(false);
  const positionHook = usePosition();
  const { address, isConnected, chainId } = useAccount();
  const removalEnabled = mode === "remove" && isConnected && !!address && chainId === paxeerChain.id;
  const removalQuery = useQuery({
    ...collateralRemovalQueryOptions(BigInt(positionId), address ?? zeroAddress),
    enabled: removalEnabled,
  });
  const removalRisk = removalEnabled && !removalQuery.isError ? removalQuery.data : undefined;
  const collateralUnits = toCollateralUnits(amount);
  const removalAllowed = canRemoveCollateral(removalRisk, collateralUnits);

  const parsedAmount = useMemo(() => {
    const n = parseFloat(amount);
    return isNaN(n) || n <= 0 ? 0 : n;
  }, [amount]);

  const handleSubmit = useCallback(async () => {
    const collateralUnits = toCollateralUnits(amount);
    if (parsedAmount <= 0 || collateralUnits <= 0n) return;
    const posId = BigInt(positionId);
    setIsPending(true);

    try {
      if (mode === "add") {
        await positionHook.addCollateral(posId, collateralUnits);
        toast({ title: POSITION_ACTION_COPY.addCollateral });
      } else if (mode === "remove") {
        if (!address || !removalEnabled || !removalAllowed) return;
        const call = await prepareCollateralRemoval(posId, collateralUnits, address);
        const session = getAccount(wagmiConfig);
        if (!session.isConnected || session.address?.toLowerCase() !== address.toLowerCase() || session.chainId !== paxeerChain.id) {
          throw new Error("Wallet session changed. Review current margin before retrying.");
        }
        await positionHook.removeCollateral(...call.args);
        toast({ title: POSITION_ACTION_COPY.successRemoveCollateral });
      } else if (mode === "addSize") {
        const lev = parseFloat(leverage);
        if (isNaN(lev) || lev <= 0) return;
        await positionHook.addSize(posId, collateralUnits, parseEther(lev.toString()));
        toast({ title: POSITION_ACTION_COPY.successAddSize });
      }
      onSuccess?.();
      onClose();
    } catch (err) {
      const errorKey = mode === "remove" ? "errorRemoveCollateral" : mode === "addSize" ? "errorAddSize" : "errorAddSize";
      toastError(err, {
        context: `position/${mode}-collateral`,
        titleOverride: POSITION_ACTION_COPY[errorKey],
        onRetry: () => { void handleSubmit(); },
      });
    } finally {
      setIsPending(false);
    }
  }, [positionId, amount, leverage, mode, parsedAmount, address, removalEnabled, removalAllowed, positionHook, onClose, onSuccess]);

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>{MODE_TITLES[mode]}</DialogTitle>
          <DialogDescription className="text-xs text-muted-foreground">
            Current collateral: ${currentCollateral.toLocaleString("en-US", { minimumFractionDigits: 2 })}
            {" | "}Size: ${currentSize.toLocaleString("en-US", { minimumFractionDigits: 2 })}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <div>
            <label className="text-xs text-muted-foreground mb-1 block">
              {mode === "addSize"
                ? POSITION_ACTION_COPY.additionalCollateralLabel
                : POSITION_ACTION_COPY.amountLabel}
            </label>
            <Input
              type="number"
              step="any"
              placeholder="0.00"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              className="text-right font-mono"
            />
          </div>

          {mode === "addSize" && (
            <div>
              <label className="text-xs text-muted-foreground mb-1 block">
                {POSITION_ACTION_COPY.leverageLabel}
              </label>
              <Input
                type="number"
                step="any"
                placeholder={currentLeverage.toString()}
                value={leverage}
                onChange={(e) => setLeverage(e.target.value)}
                className="text-right font-mono"
              />
            </div>
          )}

          {mode === "remove" && (
            <CollateralRemovalPreview
              risk={removalRisk}
              amount={collateralUnits}
              loading={removalEnabled && removalQuery.isPending}
            />
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" onClick={onClose} disabled={isPending}>
            Cancel
          </Button>
          <Button size="sm" onClick={handleSubmit} disabled={isPending || parsedAmount <= 0 || (mode === "remove" && !removalAllowed)}>
            {isPending ? "Submitting..." : MODE_TITLES[mode]}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
