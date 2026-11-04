import {
  decodeActivityFilter,
  encodeActivityFilter,
  type ActivityFilter,
  type ActivityPage,
  type ExportArtefact,
  type HumanApiClient,
} from "../../api/index.ts";
import {
  activityFailure,
  agentFilterOptions,
  feedGroups,
  loadActivity,
  mergePages,
  newExportKey,
  safeExportArtefact,
  type ActivityFailure,
} from "./model.ts";

type ExportKind = "evidence-bundle" | "statement";

export interface ActivityExportAttempt {
  readonly principal: string;
  readonly kind: ExportKind;
  readonly filter: ActivityFilter;
  readonly key: string;
}

type ActivityLoad =
  | Readonly<{ kind: "loading" }>
  | Readonly<{ kind: "loaded"; principal: string; page: ActivityPage; agents: ReturnType<typeof agentFilterOptions> }>
  | Readonly<{ kind: "offline"; failure: ActivityFailure }>
  | Readonly<{ kind: "error"; failure: ActivityFailure }>;

type ExportState =
  | Readonly<{ kind: "idle" }>
  | Readonly<{ kind: "preparing"; attempt: ActivityExportAttempt }>
  | Readonly<{ kind: "ready"; attempt: ActivityExportAttempt; artefact: ExportArtefact }>
  | Readonly<{ kind: "failed"; attempt: ActivityExportAttempt; failure: ActivityFailure }>;

export interface ActivitySnapshot {
  readonly load: ActivityLoad;
  readonly paging: boolean;
  readonly pagingFailure: ActivityFailure | undefined;
  readonly exportState: ExportState;
  readonly pendingExports: readonly ActivityExportAttempt[];
}

function retainedFilter(filter: ActivityFilter): ActivityFilter {
  const retained = decodeActivityFilter(encodeActivityFilter(filter), "retained activity filter");
  if (retained.kinds !== undefined) Object.freeze(retained.kinds);
  return Object.freeze(retained);
}

function exportIdentity(principal: string, kind: ExportKind, filter: ActivityFilter): string {
  return JSON.stringify([principal, kind, encodeActivityFilter(filter)]);
}

export class ActivityController {
  private generation = 0;
  private accountGeneration = 0;
  private exportGeneration = 0;
  private readonly listeners = new Set<() => void>();
  private readonly attempts = new Map<string, ActivityExportAttempt>();
  private readonly inFlight = new Set<string>();
  private state: ActivitySnapshot = {
    load: { kind: "loading" }, paging: false, pagingFailure: undefined,
    exportState: { kind: "idle" }, pendingExports: [],
  };

  constructor(private readonly client: HumanApiClient) {}

  readonly snapshot = (): ActivitySnapshot => this.state;

  readonly subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => { this.listeners.delete(listener); };
  };

  private publish(update: Partial<ActivitySnapshot>): void {
    const next = { ...this.state, ...update };
    const principal = next.load.kind === "loaded" ? next.load.principal : undefined;
    this.state = {
      ...next,
      pendingExports: [...this.attempts.values()].filter((attempt) => attempt.principal === principal),
    };
    for (const listener of this.listeners) listener();
  }

  invalidateAccount(): void {
    this.generation++;
    this.accountGeneration++;
    this.exportGeneration++;
    this.publish({ load: { kind: "loading" }, paging: false, pagingFailure: undefined, exportState: { kind: "idle" } });
  }

  async refresh(filter: ActivityFilter): Promise<void> {
    const generation = ++this.generation;
    this.exportGeneration++;
    const requested = retainedFilter(filter);
    this.publish({ load: { kind: "loading" }, paging: false, pagingFailure: undefined, exportState: { kind: "idle" } });
    try {
      const [result, balance] = await Promise.all([
        loadActivity(this.client, requested), this.client.accountBalance(),
      ]);
      if (generation !== this.generation) return;
      if (balance.account_id.length === 0) throw new Error("Activity requires an authenticated principal");
      feedGroups(result.page);
      this.publish({ load: {
        kind: "loaded", principal: balance.account_id, page: result.page,
        agents: agentFilterOptions(result.agents),
      } });
    } catch (error: unknown) {
      if (generation !== this.generation) return;
      const failure = activityFailure(error);
      this.publish({ load: { kind: failure.kind === "offline" ? "offline" : "error", failure } });
    }
  }

  async loadMore(): Promise<void> {
    const { load, paging } = this.state;
    if (load.kind !== "loaded" || load.page.next_cursor.length === 0 || paging) return;
    const generation = this.generation;
    this.publish({ paging: true, pagingFailure: undefined });
    try {
      const page = await this.client.activityQuery({
        cursor: load.page.next_cursor, filter: load.page.filter, page_limit: 50,
      });
      if (generation !== this.generation) return;
      const merged = mergePages(load.page, page);
      feedGroups(merged);
      this.publish({ load: { ...load, page: merged } });
    } catch (error: unknown) {
      if (generation === this.generation) this.publish({ pagingFailure: activityFailure(error) });
    } finally {
      if (generation === this.generation) this.publish({ paging: false });
    }
  }

  async requestExport(kind: ExportKind): Promise<void> {
    const { load, exportState } = this.state;
    if (load.kind !== "loaded" || exportState.kind === "preparing") return;
    const filter = retainedFilter(load.page.filter);
    const identity = exportIdentity(load.principal, kind, filter);
    const attempt = this.attempts.get(identity) ?? Object.freeze({
      principal: load.principal, kind, filter, key: newExportKey(),
    });
    this.attempts.set(identity, attempt);
    await this.executeExport(attempt);
  }

  async retryExport(key: string): Promise<void> {
    const { load, exportState } = this.state;
    if (load.kind !== "loaded" || exportState.kind === "preparing") return;
    const attempt = [...this.attempts.values()].find((value) => value.key === key);
    if (attempt === undefined || attempt.principal !== load.principal) return;
    await this.executeExport(attempt);
  }

  private async executeExport(attempt: ActivityExportAttempt): Promise<void> {
    if (this.inFlight.has(attempt.key)) return;
    this.inFlight.add(attempt.key);
    const accountGeneration = this.accountGeneration;
    const exportGeneration = ++this.exportGeneration;
    this.publish({ exportState: { kind: "preparing", attempt } });
    try {
      const balance = await this.client.accountBalance();
      if (accountGeneration !== this.accountGeneration) return;
      if (balance.account_id !== attempt.principal) {
        throw new Error("The export principal changed before reconciliation");
      }
      const value = attempt.kind === "statement"
        ? await this.client.activityExportStatement({ filter: attempt.filter }, attempt.key)
        : await this.client.activityExportEvidence({ filter: attempt.filter }, attempt.key);
      const artefact = safeExportArtefact(value);
      if (artefact.kind !== attempt.kind) throw new Error("The export kind did not match the request");
      this.attempts.delete(exportIdentity(attempt.principal, attempt.kind, attempt.filter));
      if (accountGeneration === this.accountGeneration && exportGeneration === this.exportGeneration) {
        this.publish({ exportState: { kind: "ready", attempt, artefact } });
      }
    } catch (error: unknown) {
      if (accountGeneration === this.accountGeneration && exportGeneration === this.exportGeneration) {
        this.publish({ exportState: { kind: "failed", attempt, failure: activityFailure(error) } });
      }
    } finally {
      this.inFlight.delete(attempt.key);
      if (accountGeneration === this.accountGeneration) this.publish({});
    }
  }
}
