"use client";

import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState, useSyncExternalStore } from "react";

import { copyEntry } from "../../../copy/runtime";
import { formatCopy } from "../../../copy/format";
import { humanApi, type ActivityFilter, type ActivityPage } from "../../api";
import { ACTIVE_ACCOUNT_STORAGE_KEY } from "../../auth/session";
import { ActivityController } from "./controller";
import { DesktopActivityFeed, MobileActivityFeed, type ActivityFeedGroup } from "../../kit/activity";
import { DesktopDetail, MobileDetail } from "../../kit/pattern-detail";
import { DesktopFilters, MobileFilters } from "../../kit/pattern-filters";
import { InlineNotice, ScreenCard, StateEmpty, StateFrame } from "../../kit/surface";
import { KitButton } from "../../kit/control";
import { SignedWordedAmount, StatusPill } from "../../kit/money";
import { protocolAmount } from "../../kit/model";
import { useShellSelection } from "../../shell/app-shell";
import { PrivateFigure, usePrivacyMode } from "../../settings/privacy";
import { LoadingSurface, OfflineSurface } from "../../states/surfaces";
import {
  emptyFilterValues,
  feedFilterDefs,
  feedGroups,
  filterEchoLines,
  formatEntryDate,
  sameFilterValues,
  toKitDirection,
  toWireFilter,
  type ActivityFailure,
  type FeedFilterValues,
} from "./model";

const AMOUNT_LOCALE = "en";

export function ActivityErrorSurface({ failure, onRetry }: Readonly<{ failure: ActivityFailure; onRetry: () => void }>) {
  const shell = useShellSelection().shell;
  const Detail = shell === "mobile" ? MobileDetail : DesktopDetail;
  const [technicalOpen, setTechnicalOpen] = useState(false);
  const reportPath = `/app/support?code=${encodeURIComponent(failure.code)}${
    failure.trace === undefined ? "" : `&trace=${encodeURIComponent(failure.trace)}`
  }`;
  return (
    <StateFrame
      tone="danger"
      role="alert"
      title={copyEntry("state.error").message}
      description={failure.message}
    >
      <div className="flex flex-wrap gap-3">
        {failure.retriable ? (
          <KitButton type="button" onClick={onRetry}>{copyEntry("action.retry").message}</KitButton>
        ) : null}
        <KitButton type="button" variant="secondary" onClick={() => { window.location.reload(); }}>
          {copyEntry("action.reload").message}
        </KitButton>
        <KitButton asChild variant="secondary"><a href={reportPath}>{copyEntry("action.report").message}</a></KitButton>
      </div>
      <Detail
        open={technicalOpen}
        onOpenChange={setTechnicalOpen}
        title={copyEntry("activity.error.technical").message}
        summary={copyEntry("activity.error.technical").message}
        mobileVariant="sheet"
        desktopVariant="inline"
      >
        <dl className="grid gap-2 text-sm text-foreground-secondary">
          <div><dt className="font-semibold">{copyEntry("activity.error.code").message}</dt><dd>{failure.code}</dd></div>
          {failure.trace === undefined ? null : (
            <div><dt className="font-semibold">{copyEntry("activity.error.trace").message}</dt><dd>{failure.trace}</dd></div>
          )}
        </dl>
      </Detail>
    </StateFrame>
  );
}

function groupViews(page: ActivityPage): readonly ActivityFeedGroup[] {
  return feedGroups(page).map((group) => ({
    id: group.id,
    label: group.label,
    subtotal: (
      <span className="inline-flex flex-wrap justify-end gap-2">
        <span>{copyEntry("activity.feed.subtotal_in").message} <PrivateFigure><SignedWordedAmount
          value={group.subtotalIn}
          currency={group.currency}
          locale={AMOUNT_LOCALE}
          decimals={0}
          direction="inbound"
        /></PrivateFigure></span>
        <span>{copyEntry("activity.feed.subtotal_out").message} <PrivateFigure><SignedWordedAmount
          value={protocolAmount(group.subtotalOut * -1)}
          currency={group.currency}
          locale={AMOUNT_LOCALE}
          decimals={0}
          direction="outbound"
        /></PrivateFigure></span>
      </span>
    ),
    rows: group.items.map((row) => ({
      id: row.id,
      title: row.title,
      subtitle: row.subtitle,
      occurredAt: row.date,
      sortAmount: row.amount,
      amount: row.currency === undefined ? <span aria-label={copyEntry("activity.detail.no_amount").message}>—</span> : (
        <PrivateFigure>
          <SignedWordedAmount
            value={row.amount}
            currency={row.currency}
            locale={AMOUNT_LOCALE}
            decimals={0}
            direction={toKitDirection(row.amount < 0 ? "out" : "in")}
          />
        </PrivateFigure>
      ),
      status: <StatusPill status={row.statusKey} />,
    })),
  }));
}

export function ActivityFilterResults({ page, agents, shell, masked, onSelect }: Readonly<{
  page: ActivityPage;
  agents: readonly Readonly<{ value: string; label: string }>[];
  shell: "mobile" | "desktop";
  masked: boolean;
  onSelect: (entryId: string) => void;
}>) {
  const echo = filterEchoLines(page.filter, new Map(agents.map((agent) => [agent.value, agent.label])));
  const groups = groupViews(page);
  const Feed = shell === "mobile" ? MobileActivityFeed : DesktopActivityFeed;
  return (
    <>
      <div role="status" className="flex flex-col gap-1 text-sm text-foreground-secondary">
        {echo.map((line) => <p key={line}>{line}</p>)}
      </div>
      {groups.length === 0 ? (
        <StateEmpty title={copyEntry("activity.feed.empty").message} description={echo.join(" ")} />
      ) : (
        <Feed
          groups={groups}
          onSelect={onSelect}
          dateLabel={(date) => formatEntryDate(date.toISOString())}
          columns={{
            activity: copyEntry("activity.column.activity").message,
            date: copyEntry("activity.column.date").message,
            status: copyEntry("activity.column.status").message,
            amount: copyEntry("activity.column.amount").message,
          }}
          amountSortEnabled={!masked}
        />
      )}
    </>
  );
}

export function Activity() {
  const router = useRouter();
  const shell = useShellSelection().shell;
  const client = useMemo(() => humanApi(), []);
  const controller = useMemo(() => new ActivityController(client), [client]);
  const { load, paging, pagingFailure, exportState, pendingExports } = useSyncExternalStore(
    controller.subscribe, controller.snapshot, controller.snapshot,
  );
  const { masked } = usePrivacyMode();
  const [draft, setDraft] = useState<FeedFilterValues>(() => emptyFilterValues());
  const [applied, setApplied] = useState<ActivityFilter>({});
  const refresh = (filter: ActivityFilter) => { void controller.refresh(filter); };

  useEffect(() => {
    void controller.refresh({});
    const accountChanged = (event: StorageEvent) => {
      if (event.key !== ACTIVE_ACCOUNT_STORAGE_KEY && event.key !== null) return;
      controller.invalidateAccount();
      setDraft(emptyFilterValues());
      setApplied({});
      void controller.refresh({});
    };
    window.addEventListener("storage", accountChanged);
    return () => {
      window.removeEventListener("storage", accountChanged);
      controller.invalidateAccount();
    };
  }, [controller]);

  const applyFilters = (values: FeedFilterValues) => {
    const filter = toWireFilter(values);
    setDraft(values);
    setApplied(filter);
    refresh(filter);
  };
  const requestExport = (kind: "evidence-bundle" | "statement") => { void controller.requestExport(kind); };
  const loadMore = () => { void controller.loadMore(); };

  if (load.kind === "loading") {
    return <LoadingSurface rows={6} />;
  }
  if (load.kind === "offline") {
    return <OfflineSurface onRetry={() => { refresh(applied); }} />;
  }
  if (load.kind === "error") {
    return <ActivityErrorSurface failure={load.failure} onRetry={() => { refresh(applied); }} />;
  }

  const filters = feedFilterDefs(load.agents);
  const Filter = shell === "mobile" ? MobileFilters : DesktopFilters;

  return (
    <ScreenCard
      landmark="section"
      title={copyEntry("navigation.activity").message}
      description={copyEntry("activity.summary").message}
    >
      <div className="mt-4 flex flex-col gap-5">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <Filter
            filters={filters}
            values={draft}
            onChange={(next) => {
              const values = next as FeedFilterValues;
              if (shell === "mobile") {
                applyFilters(values);
              } else {
                setDraft(values);
              }
            }}
          />
          {shell === "desktop" ? (
            <KitButton
              type="button"
              {...(sameFilterValues(draft, (() => {
                const current: Record<string, string | { from: Date; to?: Date } | undefined> = emptyFilterValues();
                const kind = load.page.filter.kinds?.[0];
                if (kind !== undefined) current["kind"] = kind;
                if (load.page.filter.agent_id !== undefined) current["agent"] = load.page.filter.agent_id;
                if (load.page.filter.from !== undefined) {
                  current["date"] = {
                    from: new Date(load.page.filter.from),
                    ...(load.page.filter.to === undefined ? {} : { to: new Date(new Date(load.page.filter.to).getTime() - 86_400_000) }),
                  };
                }
                return current;
              })()) ? {
                disabled: true,
                disabledReason: copyEntry("activity.filter.no_changes").message,
              } : {})}
              onClick={() => { applyFilters(draft); }}
            >
              {copyEntry("activity.filter.apply").message}
            </KitButton>
          ) : null}
        </div>
        <div className="flex flex-wrap gap-3">
          <KitButton
            type="button"
            variant="secondary"
            {...(exportState.kind === "preparing" ? {
              disabled: true,
              disabledReason: copyEntry("activity.export.preparing").message,
            } : {})}
            onClick={() => { requestExport("statement"); }}
          >
            {copyEntry("activity.export.statement").message}
          </KitButton>
          <KitButton
            type="button"
            variant="secondary"
            {...(exportState.kind === "preparing" ? {
              disabled: true,
              disabledReason: copyEntry("activity.export.preparing").message,
            } : {})}
            onClick={() => { requestExport("evidence-bundle"); }}
          >
            {copyEntry("activity.export.evidence").message}
          </KitButton>
        </div>
        {exportState.kind === "ready" ? (
          <InlineNotice tone="success">
            <span className="flex flex-wrap items-center justify-between gap-3">
              <span>{copyEntry("activity.export.ready").message} {formatCopy("activity.export.evidence_count", { count: exportState.artefact.evidence.length })}</span>
              <KitButton asChild size="sm"><a href={exportState.artefact.download_path} download>{copyEntry("activity.export.download").message}</a></KitButton>
            </span>
          </InlineNotice>
        ) : exportState.kind === "failed" ? (
          <InlineNotice tone="danger" role="alert">
            <span>{copyEntry("activity.export.failed").message}</span>{" "}
            <span>{formatCopy("error.technical.machine_code", { machineCode: exportState.failure.code })}</span>
          </InlineNotice>
        ) : null}
        {pendingExports.map((attempt) => (
          <div key={attempt.key} className="flex flex-wrap items-center gap-3">
            <span>{copyEntry(attempt.kind === "statement" ? "activity.export.statement" : "activity.export.evidence").message}: {filterEchoLines(attempt.filter).join(" ")}</span>
            <KitButton type="button" variant="secondary"
              {...(exportState.kind === "preparing" ? {
                disabled: true, disabledReason: copyEntry("activity.export.preparing").message,
              } : {})}
              onClick={() => { void controller.retryExport(attempt.key); }}>
              {copyEntry("action.retry").message}
            </KitButton>
          </div>
        ))}
        {exportState.kind === "ready" ? (
          <p className="text-sm text-foreground-secondary">{filterEchoLines(exportState.attempt.filter).join(" ")}</p>
        ) : null}
        <ActivityFilterResults page={load.page} agents={load.agents} shell={shell} masked={masked}
          onSelect={(entryId) => { router.push(`/app/activity/${encodeURIComponent(entryId)}`); }} />
        {pagingFailure === undefined ? null : (
          <InlineNotice tone={pagingFailure.kind === "offline" ? "warning" : "danger"} role="alert">
            {pagingFailure.message}
          </InlineNotice>
        )}
        {load.page.next_cursor.length === 0 ? null : (
          <KitButton
            type="button"
            variant="secondary"
            {...(paging ? {
              disabled: true,
              disabledReason: copyEntry("state.loading.body").message,
            } : {})}
            onClick={loadMore}
          >
            {copyEntry("activity.load_more").message}
          </KitButton>
        )}
      </div>
    </ScreenCard>
  );
}

export function human_web_journeys_Activity() {
  return Activity;
}
