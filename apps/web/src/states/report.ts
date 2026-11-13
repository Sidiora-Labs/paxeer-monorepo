import {
  parseSavedReportReceipt,
  parseSupportReportRequest,
  type ReportShell,
  type SavedReportReceipt,
  type SupportReportRequest,
} from "./report-schema.ts";
import { csrfTokenFromCookie, type VerifiedSessionIdentity } from "../auth/session.ts";
import { redactSupportReportRoute } from "./report-routes.ts";

export interface ErrorReportDraft {
  readonly consented: boolean;
  readonly traceId: string;
  readonly machineCode: string;
  readonly route: string;
  readonly shell: ReportShell;
}

export type ErrorReportReceipt = SavedReportReceipt | Readonly<{
  traceId: string;
  status: "pending";
}>;

export interface ErrorReporter {
  submit(draft: ErrorReportDraft): Promise<ErrorReportReceipt>;
}

export interface SupportReportAttempt {
  readonly principalScope: string;
  readonly report: SupportReportRequest;
}

const LEGACY_PENDING_REPORTS_KEY = "layerx.human.pending-support-reports.v1";
const PENDING_REPORTS_PREFIX = "layerx.human.pending-support-reports.v2.";
const MAX_PENDING_REPORTS = 50;
export const SUPPORT_REPORT_PRINCIPAL_HEADER = "x-layerx-report-principal";

export class SupportReportPrincipalError extends Error {
  constructor() {
    super("Support report requires its originating verified principal and session");
  }
}

export function requireSupportReportPrincipal(
  request: Request,
  session: VerifiedSessionIdentity | undefined,
): void {
  if (session === undefined || request.headers.get(SUPPORT_REPORT_PRINCIPAL_HEADER) !== session.principalScope) {
    throw new SupportReportPrincipalError();
  }
}

let privateReportEpoch = 0;

export function clearPrivateSupportReports(storage: Storage): void {
  privateReportEpoch += 1;
  const keys = Array.from({ length: storage.length }, (_, index) => storage.key(index));
  for (const key of keys) {
    if (key !== null && (key === LEGACY_PENDING_REPORTS_KEY || key.startsWith(PENDING_REPORTS_PREFIX))) {
      storage.removeItem(key);
    }
  }
}

class ReportServerError extends Error {
  readonly status: number;

  constructor(status: number) {
    super("Support report server rejected the request");
    this.status = status;
  }
}

export function redactErrorReport(draft: ErrorReportDraft): SupportReportRequest {
  if (!draft.consented) {
    throw new Error("Report consent is required");
  }
  return parseSupportReportRequest({
    traceId: draft.traceId,
    machineCode: draft.machineCode,
    route: redactSupportReportRoute(draft.route),
    shell: draft.shell,
  });
}

function pendingReports(storage: Storage, principalScope: string): SupportReportAttempt[] {
  const stored = storage.getItem(PENDING_REPORTS_PREFIX + principalScope);
  if (stored === null) return [];
  const parsed: unknown = JSON.parse(stored);
  if (!Array.isArray(parsed) || parsed.length > MAX_PENDING_REPORTS) {
    throw new Error("Invalid private report outbox");
  }
  return parsed.map((entry: unknown) => {
    if (typeof entry !== "object" || entry === null || !("principalScope" in entry)
      || entry.principalScope !== principalScope || !("report" in entry)
      || Object.keys(entry).length !== 2) {
      throw new SupportReportPrincipalError();
    }
    return Object.freeze({ principalScope, report: parseSupportReportRequest(entry.report) });
  });
}

function sameReport(left: SupportReportRequest, right: SupportReportRequest): boolean {
  return left.traceId === right.traceId && left.machineCode === right.machineCode;
}

export class BrowserSupportReportOutbox implements ErrorReporter {
  readonly #principalScope: string | undefined;
  readonly #csrf: string | undefined;
  readonly #epoch = privateReportEpoch;
  #closed = false;
  #activeFlush: Promise<void> | undefined;

  constructor(session?: VerifiedSessionIdentity) {
    if (session !== undefined) {
      if (!/^[A-Za-z0-9_-]{16,128}$/u.test(session.principalScope)
        || session.accountId.length === 0 || session.sessionId.length === 0) {
        throw new SupportReportPrincipalError();
      }
      this.#principalScope = session.principalScope;
      this.#csrf = typeof document === "undefined" ? undefined : csrfTokenFromCookie(document.cookie);
      if (typeof window !== "undefined") {
        // Unowned v1 reports cannot safely be attributed to any signed-in account.
        window.localStorage.removeItem(LEGACY_PENDING_REPORTS_KEY);
      }
    }
  }

  connect(): () => void {
    if (this.#epoch !== privateReportEpoch) throw new SupportReportPrincipalError();
    this.#closed = false;
    return () => { this.close(); };
  }

  close(): void {
    this.#closed = true;
  }

  logout(): void {
    this.close();
    if (typeof window !== "undefined") clearPrivateSupportReports(window.localStorage);
  }

  prepare(draft: ErrorReportDraft): SupportReportAttempt {
    const principalScope = this.#requirePrincipal();
    return Object.freeze({ principalScope, report: redactErrorReport(draft) });
  }

  async submit(draft: ErrorReportDraft): Promise<ErrorReportReceipt> {
    return this.submitAttempt(this.prepare(draft));
  }

  async submitAttempt(attempt: SupportReportAttempt): Promise<ErrorReportReceipt> {
    const principalScope = this.#requirePrincipal();
    if (attempt.principalScope !== principalScope) throw new SupportReportPrincipalError();
    const report = parseSupportReportRequest(attempt.report);
    try {
      const receipt = await this.#postReport(report);
      try {
        this.#removePending(report);
      } catch (error) {
        if (error instanceof SupportReportPrincipalError) throw error;
        // Delivery succeeded; retained entries remain safe to retry idempotently.
      }
      return receipt;
    } catch (error) {
      if (error instanceof ReportServerError || error instanceof SupportReportPrincipalError) throw error;
      this.#requirePrincipal();
      this.#queuePending(report);
      return Object.freeze({ traceId: report.traceId, status: "pending" as const });
    }
  }

  flushPending(): Promise<void> {
    if (typeof window === "undefined") return Promise.resolve();
    this.#requirePrincipal();
    if (!navigator.onLine) return Promise.resolve();
    if (this.#activeFlush !== undefined) return this.#activeFlush;
    const flush = this.#drainPending().finally(() => {
      if (this.#activeFlush === flush) this.#activeFlush = undefined;
    });
    this.#activeFlush = flush;
    return flush;
  }

  #requirePrincipal(): string {
    if (typeof window === "undefined" || this.#closed || this.#epoch !== privateReportEpoch || this.#principalScope === undefined
      || this.#csrf === undefined || csrfTokenFromCookie(document.cookie) !== this.#csrf) {
      throw new SupportReportPrincipalError();
    }
    return this.#principalScope;
  }

  async #postReport(report: SupportReportRequest): Promise<SavedReportReceipt> {
    const principalScope = this.#requirePrincipal();
    const csrf = this.#csrf;
    if (csrf === undefined) throw new SupportReportPrincipalError();
    const response = await fetch("/api/support/reports", {
      method: "POST",
      credentials: "same-origin",
      headers: {
        "content-type": "application/json",
        "x-layerx-csrf": csrf,
        [SUPPORT_REPORT_PRINCIPAL_HEADER]: principalScope,
      },
      body: JSON.stringify(report),
    });
    this.#requirePrincipal();
    if (!response.ok) throw new ReportServerError(response.status);
    try {
      const receipt = parseSavedReportReceipt(await response.json());
      if (receipt.traceId !== report.traceId) throw new Error("Support report receipt trace mismatch");
      return receipt;
    } catch {
      throw new ReportServerError(502);
    }
  }

  #writePending(attempts: readonly SupportReportAttempt[]): void {
    const key = PENDING_REPORTS_PREFIX + this.#requirePrincipal();
    if (attempts.length === 0) window.localStorage.removeItem(key);
    else window.localStorage.setItem(key, JSON.stringify(attempts));
  }

  #queuePending(report: SupportReportRequest): void {
    const principalScope = this.#requirePrincipal();
    const reports = pendingReports(window.localStorage, principalScope);
    const existing = reports.find((candidate) => sameReport(candidate.report, report));
    if (existing !== undefined) {
      if (JSON.stringify(existing.report) !== JSON.stringify(report)) throw new Error("Support report attempt conflict");
      return;
    }
    if (reports.length === MAX_PENDING_REPORTS) throw new Error("Private report outbox is full");
    this.#writePending([...reports, Object.freeze({ principalScope, report })]);
  }

  #removePending(report: SupportReportRequest): void {
    this.#writePending(pendingReports(window.localStorage, this.#requirePrincipal())
      .filter((candidate) => !sameReport(candidate.report, report)));
  }

  async #drainPending(): Promise<void> {
    for (const attempt of pendingReports(window.localStorage, this.#requirePrincipal())) {
      await this.#postReport(attempt.report);
      this.#removePending(attempt.report);
    }
  }
}

export const browserSupportReportOutbox = new BrowserSupportReportOutbox();
