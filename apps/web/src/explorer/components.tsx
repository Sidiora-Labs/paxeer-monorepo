import type { ReactNode } from "react";

import { copyEntry } from "../../copy/runtime";
import { ExplorerFreshness as ExplorerFreshnessView, ExplorerNavigation } from "../kit/explorer";
import { verificationWord } from "../kit/model";
import { ScreenCard } from "../kit/surface";
import { EXPLORER_NAVIGATION } from "./links";
import {
  validExplorerIdentifier,
  type ExplorerFreshness,
  type NativeExplorerFreshness,
} from "./model";

export { EXPLORER_NAVIGATION };

export function ExplorerFrame({
  title,
  description,
  children,
}: Readonly<{ title: ReactNode; description?: ReactNode; children: ReactNode }>) {
  return (
    <ScreenCard title={title} description={description} dataApplication="explorer">
      <div className="mt-4 flex flex-col gap-4">
        <ExplorerNavigation
          label={copyEntry("explorer.navigation.label").message}
          items={EXPLORER_NAVIGATION.map((item) => ({
            href: item.href,
            label: copyEntry(item.copyKey).message,
          }))}
        />
        {children}
      </div>
    </ScreenCard>
  );
}

export function verificationLabel(level: string): string {
  return verificationWord(level);
}

export const NATIVE_FRESHNESS_COPY = Object.freeze({
  current: "The explorer has indexed the committed head",
  stale: "The explorer index is behind the committed head",
  unavailable: "Explorer freshness is unavailable",
  unavailableBody: "Current committed and indexed block positions are unavailable.",
  unavailablePosition: "unavailable",
});

export function FreshnessDisplay({
  freshness,
  position,
}: Readonly<{ freshness?: ExplorerFreshness; position?: NativeExplorerFreshness }>) {
  // Old page props are historical evidence, never a source for current network freshness.
  if (freshness !== undefined || position === undefined) {
    return (
      <ExplorerFreshnessView
        title={NATIVE_FRESHNESS_COPY.unavailable}
        description={NATIVE_FRESHNESS_COPY.unavailableBody}
        current={false}
      />
    );
  }
  if (position.state === "unavailable") {
    return (
      <ExplorerFreshnessView
        title={NATIVE_FRESHNESS_COPY.unavailable}
        description={`Committed block ${String(position.committedHeight ?? NATIVE_FRESHNESS_COPY.unavailablePosition)}; indexed block ${String(position.indexedHeight ?? NATIVE_FRESHNESS_COPY.unavailablePosition)}.`}
        current={false}
      />
    );
  }
  return (
    <ExplorerFreshnessView
      title={NATIVE_FRESHNESS_COPY[position.state]}
      description={`Committed block ${String(position.committedHeight)}; indexed block ${String(position.indexedHeight)}.`}
      current={position.state === "current"}
    />
  );
}

export function ExplorerUnavailable() {
  return (
    <ExplorerFrame
      title={copyEntry("explorer.title").message}
      description={copyEntry("explorer.summary").message}
    >
      <FreshnessDisplay />
    </ExplorerFrame>
  );
}

export function ExplorerNotFound({
  title,
  freshness,
}: Readonly<{ title: string; freshness: ExplorerFreshness }>) {
  return (
    <ExplorerFrame title={title} description={copyEntry("explorer.not_found").message}>
      <FreshnessDisplay freshness={freshness} />
      <p className="text-sm text-foreground-secondary">{copyEntry("explorer.not_found.body").message}</p>
    </ExplorerFrame>
  );
}

export function explorerReceiptPath(identifier: string): string {
  if (!validExplorerIdentifier(identifier)) {
    throw new TypeError("Invalid receipt identifier");
  }
  return `/explorer/receipts/${identifier.toLowerCase()}`;
}
