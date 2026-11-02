// Proves: journeys/activity, approvals, agents, move, home and onboarding model producers -> exported TypeScript models and api/index.ts codecs -> ActivityVerification, ReleasedActivitySection, ProgramBudgetAmount, HomeQuickActions, HomeActivityList, AccountActivationNotice and JourneyStages.
// Repairs: home/model.ts homeActivityRows displayed money without a verified receipt;
// onboarding/model.ts accountActive accepted a processing or refused identity stage.
// The anchor has no storage: each observation encodes and decodes fresh producer state,
// renders the real consumer again, then changes that state to prove there is no cached shortcut.
// UI package imports resolve to its actual source so no unbuilt UI dist entry is used.
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";
import type { ReactElement } from "react";
import type {
  ActivityEntryDetail, EvidenceRef, Journey, JsonValue, ProgramApprovalBudget,
  VerificationLevel,
} from "../src/api/index.ts";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith("@layerx/ui/")) {
      const path = specifier === "@layerx/ui/cn" ? "lib/utils" : specifier.slice("@layerx/ui/".length);
      const source = new URL(`../packages/layerx-ui/src/${path}`, import.meta.url);
      for (const suffix of [".ts", ".tsx"]) {
        const candidate = new URL(source.href + suffix);
        if (existsSync(candidate)) return nextResolve(candidate.href, context);
      }
    }
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js")
          ? new URL(candidate.href.slice(0, -3) + suffix)
          : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    if (specifier === "next/navigation") return nextResolve("next/navigation.js", context);
    if (specifier === "next/link") return nextResolve("next/link.js", context);
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      return {
        format: "module", shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: {
            target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext,
            jsx: ts.JsxEmit.ReactJSX,
          },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const { createElement } = await import("react");
const { renderToStaticMarkup } = await import("react-dom/server");
const api = await import("../src/api/index.ts");
const activity = await import("../src/journeys/activity/model.ts");
const approvals = await import("../src/journeys/approvals/model.ts");
const agents = await import("../src/journeys/agents/model.ts");
const move = await import("../src/journeys/move/model.ts");
const home = await import("../src/journeys/home/model.ts");
const onboarding = await import("../src/journeys/onboarding/model.ts");
const { ActivityVerification } = await import("../src/journeys/activity/detail.tsx");
const { ReleasedActivitySection } = await import("../src/journeys/approvals/screens.tsx");
const { ProgramBudgetAmount } = await import("../src/journeys/approvals/programs.tsx");
const { HomeQuickActions, HomeActivityList } = await import("../src/journeys/home/home.tsx");
const { AccountActivationNotice } = await import("../src/journeys/onboarding/onboarding.tsx");
const { JourneyStages } = await import("../src/journeys/agents/progress.tsx");
const { PrivacyModeProvider } = await import("../src/settings/privacy.tsx");
const { copyEntry } = await import("../copy/runtime.ts");
const { observeActivityVerification } = await import("./activity.spec.ts");

function cross<T>(producer: T, encode: (value: T) => JsonValue, decode: (value: JsonValue, at: string) => T): T {
  const wire: JsonValue = JSON.parse(JSON.stringify(encode(producer)));
  return decode(wire, "journeys proof result");
}

function receipt(verification: VerificationLevel = "receipt-verified"): EvidenceRef {
  return { class: "layerx-receipt", evidence_id: "rcpt_journeys_proof", verification };
}

function journey(evidence: EvidenceRef[]): Journey {
  return {
    journey_id: "jny_journeys_proof", kind: "onboarding", state: "done",
    state_copy_key: "status.done", started_at: "2026-08-15T10:30:00Z",
    updated_at: "2026-08-15T10:30:00Z", evidence: [],
    stages: [{ stage_id: "stg_identity", copy_key: onboarding.PROTOCOL_IDENTITY_STAGE_KEY,
      state: "done", evidence }],
  };
}

function detail(evidence: EvidenceRef[]): ActivityEntryDetail {
  return {
    entry_id: "ent_journeys_proof", kind: "movement", state: "done", state_copy_key: "status.done",
    summary_copy_key: "activity.move.summary", occurred_at: "2026-08-15T10:30:00Z",
    money: { amount: 43210n, currency: "LXP" }, direction: "out",
    stages: [], evidence,
  };
}

function render(element: ReactElement): string {
  return renderToStaticMarkup(element);
}

function proof_activity_producers_reach_detail_and_activity_consumer() {
  const source = detail([receipt(), { class: "checkpoint-proof", evidence_id: "chk_journeys_proof", verification: "checkpoint-finalised" }]);
  let previous: ActivityEntryDetail | undefined;
  for (let observation = 0; observation < 2; observation += 1) {
    const decoded = cross(source, api.encodeActivityEntryDetail, api.decodeActivityEntryDetail);
    assert.notEqual(decoded, previous);
    assert.equal(activity.receiptBacked(decoded.evidence[0]!), true);
    assert.equal(activity.entryVerification(activity.detailEvidence(decoded)), "checkpoint-finalised");
    const expected = activity.verificationLabel("checkpoint-finalised");
    assert.equal(expected, copyEntry("activity.verification.checkpoint_finalised").message);
    assert.ok(render(createElement(ActivityVerification, { detail: decoded })).includes(expected));
    assert.equal(observeActivityVerification(decoded), expected);
    previous = decoded;
  }
  // Stage-only receipt backing must cross the same path used by the detail disclosure.
  source.stages = journey([receipt()]).stages;
  source.evidence = [];
  assert.ok(render(createElement(ActivityVerification, { detail: cross(source, api.encodeActivityEntryDetail, api.decodeActivityEntryDetail) }))
    .includes(copyEntry("activity.verification.receipt_verified").message));
  source.stages[0]!.evidence = [receipt("settlement-anchored")];
  const retired = cross(source, api.encodeActivityEntryDetail, api.decodeActivityEntryDetail);
  assert.equal(activity.entryVerification(activity.detailEvidence(retired)), "unverified");
  assert.equal(activity.receiptBacked(retired.stages[0]!.evidence[0]!), false);
  assert.throws(() => render(createElement(ActivityVerification, { detail: retired })), /verified receipt/u);
  assert.throws(() => observeActivityVerification(retired), /verified receipt/u);
}

function proof_approval_labels_reach_screens_and_programs() {
  const source = detail([receipt("checkpoint-finalised")]);
  const budget: ProgramApprovalBudget = {
    approval_id: "apr_journeys_proof", held_digest: "a".repeat(64), budget_id: "budget-proof",
    asset_id: "LXP", source_account: "account-proof", remaining: 43210n,
    observed_head_sequence: 12n, age_sequences: 0n, maximum_age_sequences: 4n,
    within_bound: true, terminal: false, verification: "checkpoint-finalised",
    proof_digest: "b".repeat(64), receipt_digest: "c".repeat(64), checkpoint_digest: "d".repeat(64),
    evidence: source.evidence,
  };
  for (let observation = 0; observation < 2; observation += 1) {
    const decoded = cross(source, api.encodeActivityEntryDetail, api.decodeActivityEntryDetail);
    const decodedBudget = cross(budget, api.encodeProgramApprovalBudget, api.decodeProgramApprovalBudget);
    const label = approvals.verificationLabel(decoded.evidence[0]!.verification);
    assert.equal(label, copyEntry("verification.checkpoint_finalised").message);
    const screen = render(createElement(ReleasedActivitySection, { entry: decoded }));
    assert.ok(screen.includes(label));
    assert.ok(screen.includes(decoded.evidence[0]!.evidence_id));
    assert.ok(screen.includes(`/app/activity/${source.entry_id}`));
    const program = render(createElement(ProgramBudgetAmount, { budget: decodedBudget }));
    assert.ok(program.includes(label));
    assert.ok(program.includes("43210"));
  }
  source.evidence = [receipt("paxeer-finalised")];
  budget.verification = "paxeer-finalised";
  const label = approvals.verificationLabel("paxeer-finalised");
  assert.equal(label, copyEntry("verification.unverified").message);
  assert.ok(render(createElement(ReleasedActivitySection, { entry: cross(source, api.encodeActivityEntryDetail, api.decodeActivityEntryDetail) })).includes(label));
  assert.ok(render(createElement(ProgramBudgetAmount, { budget: cross(budget, api.encodeProgramApprovalBudget, api.decodeProgramApprovalBudget) })).includes(label));
}

function proof_home_destinations_and_move_receipts_reach_home() {
  assert.ok(Object.isFrozen(home.HOME_DESTINATIONS));
  const routes: string[] = [];
  const actions = HomeQuickActions({ onNavigate: (route) => { routes.push(route); } });
  for (let observation = 0; observation < 2; observation += 1) {
    for (const id of ["add", "move", "withdraw"] as const) actions.props.onAction(id);
  }
  assert.deepEqual(routes, ["/bridge", "/app/move", "/bridge", "/bridge", "/app/move", "/bridge"]);
  assert.throws(() => actions.props.onAction("unknown"), /Unknown Home action/u);
  assert.equal(routes.length, 6);
  const actionMarkup = render(actions);
  for (const action of home.homeActions()) assert.ok(actionMarkup.includes(action.label));

  const source = detail([receipt()]);
  const stages = journey([receipt()]);
  for (let observation = 0; observation < 2; observation += 1) {
    const decoded = cross(source, api.encodeActivityEntryDetail, api.decodeActivityEntryDetail);
    assert.equal(move.verifiedOutcomeEvidence(decoded.evidence[0]!), true);
    assert.equal(move.receiptBacked(cross(stages, api.encodeJourney, api.decodeJourney)), true);
    assert.equal(move.moveResult(cross(stages, api.encodeJourney, api.decodeJourney)).kind, "done");
    assert.equal(home.homeActivityRows([decoded])[0]?.amount, 43210);
    const markup = render(createElement(PrivacyModeProvider, {
      principalScope: "journeys-proof",
      children: createElement(HomeActivityList, { entries: [decoded] }),
    }));
    assert.ok(markup.includes('data-private-figure="masked"'));
    assert.ok(markup.includes(copyEntry("status.done").message));
  }
  source.evidence = [receipt("settlement-anchored")];
  stages.stages[0]!.evidence = source.evidence;
  assert.equal(move.verifiedOutcomeEvidence(source.evidence[0]!), false);
  assert.equal(move.receiptBacked(stages), false);
  assert.equal(move.moveResult(stages).kind, "in-progress");
  const decoded = cross(source, api.encodeActivityEntryDetail, api.decodeActivityEntryDetail);
  const row = home.homeActivityRows([decoded])[0]!;
  assert.equal(row.status, "processing");
  assert.equal(row.amount, undefined);
  assert.equal(row.currency, undefined);
  assert.equal(row.direction, undefined);
  const markup = render(createElement(PrivacyModeProvider, {
    principalScope: "journeys-proof",
    children: createElement(HomeActivityList, { entries: [decoded] }),
  }));
  assert.ok(markup.includes(copyEntry("status.processing").message));
  assert.ok(!markup.includes("data-private-figure"));
}

function proof_onboarding_and_agents_recompute_activation_and_progress() {
  const source = journey([receipt()]);
  for (let observation = 0; observation < 2; observation += 1) {
    const decoded = cross(source, api.encodeJourney, api.decodeJourney);
    assert.equal(onboarding.accountActive(decoded), true);
    assert.ok(render(createElement(AccountActivationNotice, { journey: decoded })).includes('data-account-active="true"'));
    assert.equal(agents.evidenceReceiptVerified(decoded.stages[0]!.evidence), true);
    assert.equal(agents.stageReceiptVerified(decoded.stages[0]!), true);
    const progress = agents.journeyProgress(decoded);
    assert.equal(progress.complete, true);
    assert.ok(render(createElement(JourneyStages, { progress })).includes(copyEntry("verification.receipt_verified").message));
  }
  source.stages[0]!.state = "processing";
  const processing = cross(source, api.encodeJourney, api.decodeJourney);
  assert.equal(onboarding.accountActive(processing), false);
  assert.ok(render(createElement(AccountActivationNotice, { journey: processing })).includes('data-account-active="false"'));
  assert.equal(agents.stageReceiptVerified(processing.stages[0]!), false);
  assert.equal(agents.journeyProgress(processing).complete, false);
  source.state = "refused";
  source.stages[0]!.state = "done";
  assert.equal(onboarding.accountActive(cross(source, api.encodeJourney, api.decodeJourney)), false);
  source.state = "done";
  source.stages[0]!.evidence = [receipt("paxeer-finalised")];
  const retired = cross(source, api.encodeJourney, api.decodeJourney);
  assert.equal(onboarding.accountActive(retired), false);
  assert.equal(agents.evidenceReceiptVerified(retired.stages[0]!.evidence), false);
  const progress = agents.journeyProgress(retired);
  assert.equal(progress.complete, false);
  const markup = render(createElement(JourneyStages, { progress }));
  assert.ok(markup.includes(copyEntry("verification.unverified").message));
  assert.ok(!markup.includes(copyEntry("verification.receipt_verified").message));
}

function proof_malformed_producer_results_are_refused_by_consumer_codecs() {
  const encoded = api.encodeActivityEntryDetail(detail([receipt()]));
  const malformed: Record<string, JsonValue> = JSON.parse(JSON.stringify(encoded));
  malformed.evidence = [{ evidence_id: "rcpt_journeys_proof", class: "layerx-receipt", verification: "invented-rung" }];
  assert.throws(() => {
    const decoded = api.decodeActivityEntryDetail(malformed, "activity.entry result");
    render(createElement(ActivityVerification, { detail: decoded }));
  }, api.HumanApiDecodeError);
  const brokenJourney: Record<string, JsonValue> = JSON.parse(JSON.stringify(api.encodeJourney(journey([receipt()]))));
  brokenJourney.stages = "malformed";
  assert.throws(() => {
    const decoded = api.decodeJourney(brokenJourney, "onboarding.status result");
    render(createElement(AccountActivationNotice, { journey: decoded }));
  }, api.HumanApiDecodeError);
  const malformedMoney: Record<string, JsonValue> = JSON.parse(JSON.stringify(encoded));
  malformedMoney.money = { amount: "4.2", currency: "LXP" };
  assert.throws(() => api.decodeActivityEntryDetail(malformedMoney, "activity.entry result"), api.HumanApiDecodeError);
}

test(proof_activity_producers_reach_detail_and_activity_consumer);
test(proof_approval_labels_reach_screens_and_programs);
test(proof_home_destinations_and_move_receipts_reach_home);
test(proof_onboarding_and_agents_recompute_activation_and_progress);
test(proof_malformed_producer_results_are_refused_by_consumer_codecs);
