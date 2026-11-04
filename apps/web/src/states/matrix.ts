import { copyEntry } from "../../copy/runtime.ts";

export const REQUIRED_SCREEN_STATES = [
  "loading",
  "empty",
  "error",
  "offline",
  "degraded",
  "still-checking",
] as const;

export const STATE_ROUTE_MANIFEST = Object.freeze([
  ["route.root", "/"],
  ["route.app", "/app"],
  ["route.app.move", "/app/move"],
  ["route.app.activity", "/app/activity"],
  ["route.app.activity.detail", "/app/activity/[entryId]"],
  ["route.app.settings", "/app/settings"],
  ["route.app.settings.devices", "/app/settings/devices"],
  ["route.app.settings.security", "/app/settings/security"],
  ["route.app.settings.wallet", "/app/settings/wallet"],
  ["route.app.security.devices", "/app/security/devices"],
  ["route.app.security.recovery", "/app/security/recovery"],
  ["route.app.support", "/app/support"],
  ["route.app.approvals", "/app/approvals"],
  ["route.app.approval", "/app/approvals/[approvalId]"],
  ["route.app.notifications", "/app/notifications"],
  ["route.app.agents", "/app/agents"],
  ["route.app.agents.new", "/app/agents/new"],
  ["route.app.agents.detail", "/app/agents/[agentId]"],
  ["route.explorer", "/explorer"],
  ["route.explorer.receipt", "/explorer/receipts/[receiptId]"],
  ["route.explorer.account", "/explorer/accounts/[accountId]"],
  ["route.explorer.program", "/explorer/programs/[programId]"],
  ["route.bridge", "/bridge"],
  ["route.exchange", "/exchange"],
  ["route.launchpad", "/launchpad"],
] as const);

export const CURRENT_ROUTES = Object.freeze(STATE_ROUTE_MANIFEST.map(([, route]) => route));
export const CURRENT_SHELLS = ["authenticated"] as const;

export type ScreenState = (typeof REQUIRED_SCREEN_STATES)[number];
export type HumanShell = "mobile" | "desktop";
export type StateTargetKind = "route" | "shell";

export interface StatePresentation {
  readonly titleKey: string;
  readonly descriptionKey: string;
  readonly actionKeys: readonly string[];
  readonly duplicateActionsLocked: boolean;
}

export interface ScreenStateEntry {
  readonly id: string;
  readonly kind: StateTargetKind;
  readonly route: string;
  readonly shells: readonly HumanShell[];
  readonly states: Readonly<Record<ScreenState, () => StatePresentation>>;
}

const presentation = (
  titleKey: string,
  descriptionKey: string,
  actionKeys: readonly string[] = [],
  duplicateActionsLocked = false,
): (() => StatePresentation) =>
  () => Object.freeze({
    titleKey,
    descriptionKey,
    actionKeys: Object.freeze(actionKeys),
    duplicateActionsLocked,
  });

const completeStates = (): ScreenStateEntry["states"] => Object.freeze({
  loading: presentation("state.loading", "state.loading.body"),
  empty: presentation("state.empty", "state.empty.body"),
  error: presentation(
    "state.error",
    "state.error.body",
    ["action.retry", "action.reload", "action.report"],
  ),
  offline: presentation("state.offline", "state.offline.body", ["action.retry", "action.report"]),
  degraded: presentation(
    "state.degraded",
    "state.degraded.age",
    ["action.reload", "action.report"],
  ),
  "still-checking": presentation(
    "status.still_checking",
    "state.still_checking.body",
    [],
    true,
  ),
});

const BOTH_SHELLS: readonly HumanShell[] = Object.freeze(["mobile", "desktop"]);

const currentTargets: readonly ScreenStateEntry[] = Object.freeze([
  ...STATE_ROUTE_MANIFEST.map(([id, route]): ScreenStateEntry => Object.freeze({
    id, kind: "route", route, shells: BOTH_SHELLS, states: completeStates(),
  })),
  Object.freeze({ id: "shell.authenticated", kind: "shell", route: "/app", shells: BOTH_SHELLS, states: completeStates() }),
]);

export class StateMatrix {
  readonly #entries: ReadonlyMap<string, ScreenStateEntry>;

  constructor(entries: readonly ScreenStateEntry[]) {
    this.#entries = new Map(entries.map((entry) => [entry.id, entry]));
    if (this.#entries.size !== entries.length) {
      throw new Error("state-matrix target identifiers must be unique");
    }
  }

  enumerate(): readonly ScreenStateEntry[] {
    return Object.freeze([...this.#entries.values()]);
  }

  assertComplete(
    routes: readonly string[] = CURRENT_ROUTES,
    shells: readonly string[] = CURRENT_SHELLS,
  ): void {
    const registeredRoutes = new Set<string>();
    const registeredShells = new Set<string>();
    const expectedRoutes = new Set(routes);
    const expectedShells = new Set(shells);

    for (const entry of this.#entries.values()) {
      if (entry.kind === "route") {
        if (!expectedRoutes.has(entry.route)) throw new Error(`${entry.route} is not a current route`);
        if (registeredRoutes.has(entry.route)) throw new Error(`${entry.route} has duplicate state registrations`);
        registeredRoutes.add(entry.route);
      } else {
        const shell = entry.id.replace(/^shell\./u, "");
        if (!expectedShells.has(shell)) throw new Error(`${shell} is not a current shell`);
        registeredShells.add(shell);
      }
      if (
        entry.shells.length !== 2
        || !entry.shells.includes("mobile")
        || !entry.shells.includes("desktop")
      ) {
        throw new Error(`${entry.id} must declare both shells`);
      }
      for (const state of REQUIRED_SCREEN_STATES) {
        const renderer = entry.states[state];
        if (typeof renderer !== "function") {
          throw new Error(`${entry.id} is missing ${state}`);
        }
        const rendered = renderer();
        copyEntry(rendered.titleKey);
        copyEntry(rendered.descriptionKey);
        for (const actionKey of rendered.actionKeys) {
          copyEntry(actionKey);
        }
        if (state === "still-checking" && !rendered.duplicateActionsLocked) {
          throw new Error(`${entry.id} must lock duplicate actions while still checking`);
        }
      }
    }

    for (const route of routes) {
      if (!registeredRoutes.has(route)) {
        throw new Error(`${route} has no state-matrix registration`);
      }
    }
    for (const shell of shells) {
      if (!registeredShells.has(shell)) {
        throw new Error(`${shell} has no state-matrix registration`);
      }
    }
  }
}

export function humanStateMatrix(): StateMatrix {
  return new StateMatrix(currentTargets);
}

export function human_state_matrix_registry(): StateMatrix {
  return humanStateMatrix();
}
