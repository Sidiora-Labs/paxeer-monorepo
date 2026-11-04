import React from "react";
import { Trophy } from "lucide-react";
import type {
  ReferralTierView,
  UserReferralData,
} from "@/hooks/perps/usePerfdbReferrals";

export function ReferralTierStatus({
  data,
  view,
}: {
  data: UserReferralData;
  view: ReferralTierView;
}) {
  const { nextTier, progressPct } = view;
  return (
    <>
      <div className="flex items-center gap-2 mb-2">
        <Trophy className="h-4 w-4 text-amber-400" />
        <span className="text-lg font-semibold capitalize">{view.label}</span>
        <span className="text-xs text-muted-foreground ml-auto">
          {data.referralCount} referrals
        </span>
      </div>
      {nextTier && progressPct !== null ? (
        <>
          <div className="h-2 bg-surface-2 rounded-full overflow-hidden mb-1.5">
            <div
              className="h-full bg-primary transition-all duration-300"
              style={{ width: `${progressPct}%` }}
            />
          </div>
          <p className="text-xs text-muted-foreground">
            {data.referralCount} / {nextTier.requiredReferrals} referrals to{" "}
            <span className="capitalize">{nextTier.name}</span>
          </p>
        </>
      ) : view.isMaxTier ? (
        <p className="text-xs text-emerald-400">Max tier unlocked 🎉</p>
      ) : null}
    </>
  );
}
