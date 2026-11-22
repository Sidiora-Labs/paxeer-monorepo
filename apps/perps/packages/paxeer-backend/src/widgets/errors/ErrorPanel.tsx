import * as React from "react";
import { Button } from "@paxeer-network/paxeer-backend/ui/atoms/button";
import type { UserFacingError } from "@paxeer-network/paxeer-backend/core/errors/types";
import { openReportModal } from "./ReportFeedbackModal";

export interface ErrorPanelProps {
  error: UserFacingError;
  onRetry?: () => void | Promise<void>;
}

export function ErrorPanel({ error, onRetry }: ErrorPanelProps) {
  return (
    <div
      role="alert"
      className="space-y-3 rounded-[var(--radius-md)] bg-surface-2 p-4"
    >
      <div className="space-y-1">
        <p className="text-sm font-semibold">{error.title}</p>
        <p className="text-sm text-muted-foreground">{error.message}</p>
        {error.suggestedFix && (
          <p className="text-xs text-muted-foreground">
            <span className="font-medium text-foreground/80">Try this: </span>
            {error.suggestedFix}
          </p>
        )}
      </div>
      {(error.isRetryable && onRetry) || error.isReportable ? (
        <div className="flex items-center gap-2">
          {error.isRetryable && onRetry && (
            <Button type="button" variant="outline" size="sm" onClick={onRetry}>
              Retry
            </Button>
          )}
          {error.isReportable && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => openReportModal(error)}
            >
              Report
            </Button>
          )}
        </div>
      ) : null}
    </div>
  );
}
