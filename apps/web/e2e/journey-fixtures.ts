import assert from "node:assert/strict";
import { createHash, createPublicKey, verify } from "node:crypto";
import { readFileSync } from "node:fs";

import {
  decodeActivityEntryDetail, decodeActivityPage, decodeApprovalDecision, decodeApprovalDetail,
  decodeApprovalPage, decodeJourney, decodeMoveQuote, decodeNotificationPage,
  type ApiError, type ApprovalDetail, type ApprovalDecision, type Journey, type JsonValue,
} from "../src/api/generated/index.ts";

const goldenRoot = new URL("../../../services/human/schema/human-api/golden/", import.meta.url);
export function goldenResult(name: string): JsonValue {
  const fixture = JSON.parse(readFileSync(new URL(`${name}.response.json`, goldenRoot), "utf8")) as {
    body: { ok: boolean; result: JsonValue; };
  };
  assert.equal(fixture.body.ok, true);
  return fixture.body.result;
}

export function jsonWire(value: unknown): string {
  return JSON.stringify(value, (_key, item: unknown) => typeof item === "bigint" ? item.toString() : item);
}

export type MoveScenario = "done" | "no-receipt" | "sending" | "still-checking" | "refused" | "change-path" | "quote-expired";
export type OnboardingScenario = "progress" | "active" | "failure" | "queued" | "resume";
export interface FixtureAnswer { readonly status: number; readonly body: string; }
export interface FixtureRequest {
  readonly method: string;
  readonly path: string;
  readonly body?: string | undefined;
  readonly idempotencyKey?: string | undefined;
}

export function onboardingFixture(scenario: OnboardingScenario): Journey {
  const journey = decodeJourney(goldenResult(scenario === "active" ? "onboarding.resume" : "onboarding.status"), "committed producer fixture");
  if (scenario === "failure") journey.stages[2]!.state = "refused";
  if (scenario === "queued") journey.stages[2]!.state = "getting-ready";
  return decodeJourney(JSON.parse(jsonWire(journey)) as JsonValue, "scenario journey");
}

export function moveFixture(scenario: MoveScenario): Journey {
  const journey = decodeJourney(goldenResult("move.commit"), "committed producer fixture");
  if (scenario === "no-receipt") {
    journey.evidence = [];
    for (const stage of journey.stages) stage.evidence = [];
  } else if (scenario === "sending" || scenario === "still-checking") {
    journey.state = scenario;
    journey.state_copy_key = `status.${scenario}`;
    journey.evidence = [];
    for (const stage of journey.stages) { stage.state = scenario; stage.evidence = []; }
  } else if (scenario === "refused" || scenario === "change-path") {
    journey.state = "refused";
    journey.state_copy_key = "status.refused";
    journey.refusal = {
      refused_by: "budget", copy_key: "journey.refused.generic", money_left: false,
      ...(scenario === "change-path" ? { change_path: "/app/agents/agt_01j8agd2s3a1b2c3d4e5f6g7h8" } : {}),
    };
    for (const stage of journey.stages) { stage.state = "refused"; stage.evidence = []; }
    journey.evidence = [];
  }
  return decodeJourney(JSON.parse(jsonWire(journey)) as JsonValue, "scenario journey");
}

export function verifyPasskeyCredential(encoded: string, challenge: string, origin: string, publicKey: string, counter: number): { counter: number; signature: string; } {
  const serialized = JSON.parse(Buffer.from(encoded, "base64url").toString()) as {
    response?: { clientDataJSON: string; authenticatorData: string; signature: string; };
    clientDataJSON: string; authenticatorData: string; signature: string;
  };
  const credential = serialized.response ?? serialized;
  const data = Buffer.from(credential.clientDataJSON, "base64url");
  const clientData = JSON.parse(data.toString()) as { type: string; challenge: string; origin: string; };
  assert.equal(clientData.type, "webauthn.get"); assert.equal(clientData.challenge, challenge);
  assert.equal(clientData.origin, origin);
  const auth = Buffer.from(credential.authenticatorData, "base64url");
  assert.deepEqual(auth.subarray(0, 32), createHash("sha256").update(new URL(origin).hostname).digest());
  assert.equal(auth[32]! & 5, 5, "user presence and verification are required");
  assert.ok(auth.readUInt32BE(33) > counter, "authenticator counter must advance");
  assert.ok(verify(null, Buffer.concat([auth, createHash("sha256").update(data).digest()]),
    createPublicKey({ key: { kty: "OKP", crv: "Ed25519", x: publicKey }, format: "jwk" }),
    Buffer.from(credential.signature, "base64url")), "the real passkey signature must verify");
  return { counter: auth.readUInt32BE(33), signature: credential.signature };
}

// The peer owns durable fixture state. Both HTTP and browser paths use this same
// contract, while production clients, ceremony, controllers and renderers run unchanged.
export class JourneyFixtureStore {
  readonly requests: FixtureRequest[] = [];
  readonly decisions = new Map<string, ApprovalDecision>();
  readonly decisionKeys = new Map<string, string>();
  readonly details = new Map<string, ApprovalDetail>();
  readonly releases = new Set<string>();
  readonly signatures: string[] = [];
  readonly #origin: string;
  readonly #publicKey: string;
  #challenge: string | undefined;
  #confirms: string | undefined;
  #verified = false;
  #counter = 0;
  releaseReceiptAvailable = true;
  moveScenario: MoveScenario = "done";
  onboardingScenario: OnboardingScenario = "progress";
  readonly now: Date;

  constructor(origin: string, publicKey: string, now = new Date()) {
    this.#origin = origin;
    this.#publicKey = publicKey;
    this.now = now;
    const pending = decodeApprovalDetail(goldenResult("approval.get"), "committed producer fixture");
    pending.facts.expires_at = new Date(now.getTime() + 3_600_000).toISOString();
    this.details.set(pending.approval_id, pending);
    const expired = structuredClone(pending);
    expired.approval_id = "apr_01j8agf4u5a1b2c3d4e5f6g7h9";
    expired.state = "expired"; expired.state_copy_key = "approval.state.expired";
    expired.facts.expires_at = new Date(now.getTime() - 60_000).toISOString();
    this.details.set(expired.approval_id, expired);
    const defective = structuredClone(pending);
    defective.approval_id = "apr_01j8agf4u5a1b2c3d4e5f6g7ha";
    defective.state = "defective"; defective.state_copy_key = "approval.state.defective";
    defective.evidence = [];
    this.details.set(defective.approval_id, defective);
  }

  pendingId(): string { return [...this.details.values()].find((row) => row.state === "pending")!.approval_id; }
  idFor(state: ApprovalDetail["state"]): string {
    const row = [...this.details.values()].find((item) => item.state === state);
    assert.ok(row, `required ${state} approval must exist`);
    return row.approval_id;
  }

  answer(request: FixtureRequest): FixtureAnswer | undefined {
    const { method, path } = request;
    if (!path.startsWith("/human/v1/")) return undefined;
    const route = path.slice("/human".length);
    const input = request.body === undefined || request.body === "" ? {} : JSON.parse(request.body) as Record<string, unknown>;
    const success = (result: unknown, status = 200): FixtureAnswer => ({ status, body: jsonWire({ ok: true, trace: "trc_01j8agg7x8a1b2c3d4e5f6g7h8", result }) });
    const failure = (code: ApiError["code"], copy_key: string): FixtureAnswer => ({
      status: 409,
      body: jsonWire({ ok: false, trace: "trc_01j8agg7x8a1b2c3d4e5f6g7h8", error: { code, copy_key, retry: "final" } })
    });
    let answer: FixtureAnswer | undefined;
    if (method === "GET" && route === "/v1/approvals") {
      const template = decodeApprovalPage(goldenResult("approval.list"), "committed producer fixture").approvals[0]!;
      answer = success({
        approvals: [...this.details.values()].map((detail) => ({
          ...template,
          approval_id: detail.approval_id, state: detail.state, expires_at: detail.facts.expires_at
        })), next_cursor: ""
      });
    } else if (method === "GET" && route === "/v1/program-approvals/page/start") {
      answer = success({ approvals: [], next_cursor: "" });
    } else if (method === "GET" && route.startsWith("/v1/approvals/")) {
      const detail = this.details.get(route.slice("/v1/approvals/".length));
      assert.ok(detail, "the requested approval must exist");
      answer = success(detail);
    } else if (method === "POST" && route === "/v1/step-up") {
      assert.equal(typeof input.confirms, "string");
      this.#confirms = String(input.confirms);
      this.#challenge = createHash("sha256").update(`${this.#confirms}:${this.requests.length}`).digest("base64url");
      this.#verified = false;
      answer = success({
        challenge_id: "chg_01j2h5w4eh8j0k2m4n6p8q0r2s", confirms: this.#confirms,
        ceremony: Buffer.from(jsonWire({ rpId: new URL(this.#origin).hostname, challenge: this.#challenge, userVerification: "required" })).toString("base64url"),
        expires_at: new Date(Date.now() + 300_000).toISOString()
      }, 201);
    } else if (method === "POST" && route === "/v1/step-up/chg_01j2h5w4eh8j0k2m4n6p8q0r2s") {
      assert.ok(this.#challenge);
      assert.equal(typeof input.credential, "string");
      const credential = verifyPasskeyCredential(String(input.credential), this.#challenge, this.#origin, this.#publicKey, this.#counter);
      this.#counter = credential.counter; this.#verified = true;
      this.signatures.push(credential.signature);
      answer = success({
        challenge_id: "chg_01j2h5w4eh8j0k2m4n6p8q0r2s", confirms: this.#confirms,
        passkey_id: "pky_01j2h5m2we5f7g9h1j3k5m7n9p", completed_at: new Date().toISOString(),
        expires_at: new Date(Date.now() + 300_000).toISOString()
      });
    } else if (method === "POST" && /^\/v1\/approvals\/[^/]+\/(approve|reject)$/u.test(route)) {
      const [, , , id, action] = route.split("/");
      const detail = this.details.get(id!); assert.ok(detail);
      assert.ok(request.idempotencyKey, "every decision must have an idempotency identity");
      if (this.decisions.has(id!)) answer = failure("already-decided", "error.approval.already-decided");
      else if (detail.state === "expired") answer = failure("hold-expired", "error.approval.hold-expired");
      else if (detail.state === "defective") answer = failure("hold-defective", "error.approval.hold-defective");
      else {
        if (action === "approve") {
          assert.equal(this.#confirms, `opd_${detail.evidence[0]!.evidence_id.slice(4)}`);
          assert.equal(input.step_up_evidence, "evd_01j2h5w4eh8j0k2m4n6p8q0r2s");
          assert.ok(this.#verified, "approval must complete the real passkey ceremony");
          this.#verified = false;
        }
        const decision = decodeApprovalDecision(goldenResult(`approval.${action}`), "committed producer fixture");
        this.decisions.set(id!, decision); this.decisionKeys.set(id!, request.idempotencyKey!);
        detail.state = decision.state; detail.state_copy_key = decision.state_copy_key;
        if (action === "approve") this.releases.add(id!);
        answer = success(decision);
      }
    } else if (method === "POST" && route === "/v1/activity/query") {
      const page = decodeActivityPage(goldenResult("activity.query"), "committed producer fixture");
      page.groups = page.groups.slice(0, 1);
      page.groups[0]!.entries = [...this.releases].map((id) => ({ ...page.groups[0]!.entries[1]!, approval_id: id }));
      page.next_cursor = "";
      answer = success(page);
    } else if (method === "GET" && route.startsWith("/v1/activity/")) {
      const detail = decodeActivityEntryDetail(goldenResult("activity.entry"), "committed producer fixture");
      detail.entry_id = route.slice("/v1/activity/".length);
      detail.kind = "movement"; detail.state = "done"; detail.state_copy_key = "status.done";
      detail.summary_copy_key = "activity.move.summary";
      detail.evidence = decodeJourney(goldenResult("move.commit"), "committed producer fixture").evidence;
      detail.stages = decodeJourney(goldenResult("move.commit"), "committed producer fixture").stages;
      if (!this.releaseReceiptAvailable) {
        detail.state = "processing"; detail.state_copy_key = "status.processing"; detail.evidence = [];
        for (const stage of detail.stages) { stage.state = "processing"; stage.evidence = []; }
      }
      answer = success(detail);
    } else if (method === "POST" && route === "/v1/moves/quote") {
      const quote = decodeMoveQuote(goldenResult("move.quote"), "committed producer fixture");
      assert.ok(input.source); assert.ok(input.destination); assert.ok(input.money);
      quote.expires_at = new Date(Date.now() + 300_000).toISOString();
      answer = success(quote);
    } else if (method === "POST" && route === "/v1/moves") {
      assert.ok(request.idempotencyKey); assert.equal(input.quote_id, decodeMoveQuote(goldenResult("move.quote"), "committed producer fixture").quote_id);
      answer = this.moveScenario === "quote-expired" ? failure("quote-expired", "error.move.quote-expired")
        : success(moveFixture(this.moveScenario));
    } else if (method === "GET" && route.startsWith("/v1/journeys/")) {
      answer = success(moveFixture(this.moveScenario));
    } else if (method === "GET" && route === "/v1/onboarding") {
      answer = success(onboardingFixture(this.onboardingScenario));
    } else if (method === "POST" && route === "/v1/onboarding/resume") {
      this.onboardingScenario = "active";
      answer = success(onboardingFixture(this.onboardingScenario));
    } else if (method === "GET" && route === "/v1/notifications") {
      answer = success(decodeNotificationPage(goldenResult("notification.list"), "committed producer fixture"));
    }
    if (answer !== undefined) this.requests.push(request);
    return answer;
  }
}
