import assert from "node:assert/strict";
import { createHash, generateKeyPairSync, sign, verify } from "node:crypto";
import { readFile } from "node:fs/promises";
import { createServer, type IncomingHttpHeaders } from "node:http";
import type { AddressInfo } from "node:net";
import { test } from "node:test";

import {
  createHumanApiClient,
  decodeActivityEntryDetail,
  decodeApprovalDetail,
  decodeJourney,
  decodeProgramApprovalDetail,
  decodeProgramApprovalDisclosureRequest,
  decodeStepUpFinish,
  encodeApprovalDecision,
  encodeApprovalDetail,
  encodeJourney,
  encodeProgramApprovalDecision,
  encodeProgramApprovalDetail,
  encodeProgramApprovalDisclosure,
  encodeStepUpChallenge,
  encodeStepUpEvidence,
  journeyStateVariants,
  type HumanApiClient,
  type JourneyState,
  type JsonValue,
  type StepUpChallenge,
  type StepUpEvidence,
} from "../src/api/generated/index.ts";
import { Approvals } from "../src/journeys/approvals/controller.ts";
import {
  browserPasskeyAuthenticator,
  performStepUp,
} from "../src/journeys/approvals/ceremony.ts";
import { canApprove, heldDigest, stepUpEvidenceReference } from "../src/journeys/approvals/model.ts";
import { journeyStatus as approvalJourneyStatus } from "../src/journeys/approvals/model.ts";
import { entryStatusKey, stageView } from "../src/journeys/activity/model.ts";
import { journeyProgress, journeyStatusKey as agentJourneyStatus } from "../src/journeys/agents/model.ts";
import { homeActivityRows } from "../src/journeys/home/model.ts";
import { moveResult } from "../src/journeys/move/model.ts";
import { journeyStatusKey as onboardingJourneyStatus, stagePresentation } from "../src/journeys/onboarding/model.ts";
import type { StatusKey } from "../src/kit/model.ts";
import {
  performAssertionCeremony,
  serializeAssertionCredential,
} from "../src/journeys/onboarding/ceremony.ts";

const repository = new URL("../../../", import.meta.url);
const digest = `opd_${"12".repeat(32)}`;
const challengeId = "chg_00112233445566778899aabbccddeeff";
const passkeyId = "pky_ffeeddccbbaa99887766554433221100";
const idempotencyKey = "00112233445566778899aabbccddeeff";

function record(value: unknown): Record<string, unknown> {
  assert.ok(typeof value === "object" && value !== null && !Array.isArray(value));
  return value as Record<string, unknown>;
}

function text(value: unknown): string {
  assert.equal(typeof value, "string");
  assert.ok((value as string).length > 0);
  return value as string;
}

function timestamp(value: string): number {
  assert.match(value, /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z$/u);
  const milliseconds = Date.parse(value);
  assert.ok(Number.isFinite(milliseconds));
  return milliseconds;
}

async function golden(name: string): Promise<JsonValue> {
  const fixture = record(JSON.parse(await readFile(new URL(
    `services/human/schema/human-api/golden/${name}.response.json`, repository,
  ), "utf8")) as unknown);
  return record(fixture["body"])["result"] as JsonValue;
}

interface Exchange {
  method: "GET" | "POST";
  path: string;
  result: JsonValue;
  body?: JsonValue;
  inspect?: (body: JsonValue | undefined, headers: IncomingHttpHeaders) => void;
}

// The peer emits the production codecs' output over a real socket. It has no
// substitute verifier, authorization implementation, or approval state machine.
async function withHttpPeer(
  exchanges: readonly Exchange[],
  run: (client: HumanApiClient) => Promise<void>,
): Promise<void> {
  let index = 0;
  const failures: unknown[] = [];
  const server = createServer(async (request, response) => {
    try {
      const exchange = exchanges[index++];
      assert.ok(exchange, `unexpected request: ${request.method} ${request.url}`);
      assert.equal(request.method, exchange.method);
      assert.equal(request.url, exchange.path);
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const bytes = Buffer.concat(chunks).toString("utf8");
      const body = bytes.length === 0 ? undefined : JSON.parse(bytes) as JsonValue;
      if (exchange.body !== undefined) assert.deepEqual(body, exchange.body);
      exchange.inspect?.(body, request.headers);
      response.writeHead(200, { "Content-Type": "application/json", Connection: "close" });
      response.end(JSON.stringify({ ok: true, result: exchange.result, trace: "trc_approval_repair" }));
    } catch (error) {
      failures.push(error);
      response.writeHead(500).end();
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => { server.off("error", reject); resolve(); });
  });
  try {
    const baseUrl = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
    await run(createHumanApiClient({ baseUrl }));
    assert.deepEqual(failures, []);
    assert.equal(index, exchanges.length, "every declared transport exchange must execute");
  } finally {
    await new Promise<void>((resolve, reject) => {
      server.close((error) => { if (error) reject(error); else resolve(); });
      server.closeAllConnections();
    });
  }
}

function stepUpValues() {
  const now = Math.floor(Date.now() / 1000) * 1000;
  const challenge: StepUpChallenge = {
    challenge_id: challengeId,
    confirms: digest,
    ceremony: Buffer.from(JSON.stringify({
      challenge: "AAECAwQFBgcICQoLDA0ODw", rpId: "wallet.example", userVerification: "required",
    })).toString("base64url"),
    expires_at: new Date(now + 300_000).toISOString(),
  };
  const evidence: StepUpEvidence = {
    challenge_id: challengeId, confirms: digest, passkey_id: passkeyId,
    completed_at: new Date(now).toISOString(), expires_at: new Date(now + 60_000).toISOString(),
  };
  return { challenge, evidence };
}

function stepUpExchanges(challenge: StepUpChallenge, evidence: StepUpEvidence, credential: string): Exchange[] {
  return [
    { method: "POST", path: "/v1/step-up", body: { confirms: challenge.confirms }, result: encodeStepUpChallenge(challenge) },
    { method: "POST", path: `/v1/step-up/${challenge.challenge_id}`, body: { credential }, result: encodeStepUpEvidence(evidence) },
  ];
}

async function approval_operation_digest_real_http() {
  const initial = decodeApprovalDetail(await golden("approval.get"), "committed approval fixture");
  initial.facts.expires_at = new Date(Date.now() + 3_600_000).toISOString();
  // The authoritative digest is an independent disclosure. Its bytes must never
  // be inferred from the evidence name, including a native SHA-256 evidence ID.
  initial.evidence[0]!.evidence_id = `evd_${"ab".repeat(32)}`;
  const disclosed = { ...initial, operation_digest: digest };
  assert.equal(heldDigest(disclosed), digest);
  assert.equal(canApprove(disclosed, new Date()), true);
  assert.equal(heldDigest(initial), undefined);
  assert.equal(heldDigest({ ...disclosed, evidence: [] }), undefined);
  for (const malformed of [
    { ...disclosed, operation_digest: digest.toUpperCase() },
    { ...disclosed, operation_digest: `opd_${"A".repeat(43)}` },
  ]) assert.equal(heldDigest(malformed), undefined);
  assert.equal(canApprove({ ...disclosed, state: "approved" }, new Date()), false);
  assert.equal(canApprove(disclosed, new Date(initial.facts.expires_at)), false);

  const { challenge, evidence } = stepUpValues();
  const credential = signedAssertion().credential;
  const approved = {
    approval_id: initial.approval_id, state: "approved" as const,
    state_copy_key: "approval.state.approved", money_moved: false,
    moved_copy_key: "approval.decision.no-money-moved", evidence: initial.evidence,
  };
  const rejectId = `apr_${"34".repeat(32)}`;
  const rejected = { ...approved, approval_id: rejectId, state: "rejected" as const, state_copy_key: "approval.state.rejected" };
  await withHttpPeer([
    { method: "GET", path: `/v1/approvals/${initial.approval_id}`, result: encodeApprovalDetail(disclosed) },
    ...stepUpExchanges(challenge, evidence, credential),
    { method: "POST", path: `/v1/approvals/${initial.approval_id}/approve`,
      body: { step_up_evidence: challengeId }, result: encodeApprovalDecision(approved),
      inspect: (_body, headers) => assert.equal(headers["idempotency-key"], idempotencyKey) },
    { method: "GET", path: `/v1/approvals/${initial.approval_id}`,
      result: encodeApprovalDetail({ ...initial, state: "approved" }) },
    { method: "POST", path: `/v1/approvals/${rejectId}/reject`, result: encodeApprovalDecision(rejected) },
    { method: "GET", path: `/v1/approvals/${rejectId}`,
      result: encodeApprovalDetail({ ...initial, approval_id: rejectId, state: "rejected" }) },
  ], async (client) => {
    const approvals = new Approvals({ client, authenticator: async () => credential });
    const detail = await approvals.detail(initial.approval_id);
    assert.equal(detail.state, "pending");
    assert.ok(detail.evidence.some((item) => item.class === "approval-hold"));
    assert.equal(heldDigest(detail), digest,
      "the production ApprovalDetail encoder and decoder must retain the explicit operation digest");
    assert.equal(canApprove(detail, new Date()), true);
    const outcome = await approvals.approve(detail, idempotencyKey);
    assert.equal(outcome.kind, "decided");
    if (outcome.kind !== "decided") throw new Error("Ordinary approval did not reach a decision");
    assert.equal(outcome.decision.approval_id, detail.approval_id);
    assert.equal(outcome.decision.state, "approved");
    assert.equal((await client.approvalGet(initial.approval_id)).state, "approved");
    const rejection = await approvals.reject(rejectId, idempotencyKey);
    assert.equal(rejection.kind, "decided");
    if (rejection.kind !== "decided") throw new Error("Ordinary rejection did not reach a decision");
    assert.equal(rejection.decision.state, "rejected");
    assert.equal(rejection.decision.money_moved, false);
    assert.equal((await client.approvalGet(rejectId)).state, "rejected");
  });
}

async function stepup_reference_real_http_ordinary_and_program() {
  const { challenge, evidence } = stepUpValues();
  const credential = signedAssertion().credential;
  const ordinary = decodeApprovalDetail(await golden("approval.get"), "ordinary fixture");
  const program = decodeProgramApprovalDetail(await golden("approval.program.get"), "Programs fixture");
  program.expires_at = new Date(Date.now() + 3_600_000).toISOString();
  const decision = {
    approval_id: program.approval_id, held_digest: program.held_digest, state: "approved" as const,
    state_copy_key: "approval.state.approved", money_moved: false,
    moved_copy_key: "approval.decision.no-money-moved", evidence: program.evidence, release_ref: "release-fixture",
  };
  await withHttpPeer([
    ...stepUpExchanges(challenge, evidence, credential),
    { method: "POST", path: `/v1/approvals/${ordinary.approval_id}/approve`,
      body: { step_up_evidence: challengeId }, result: encodeApprovalDecision({
        approval_id: ordinary.approval_id, state: "approved", state_copy_key: "approval.state.approved",
        money_moved: false, moved_copy_key: "approval.decision.no-money-moved", evidence: ordinary.evidence,
      }) },
    { method: "GET", path: `/v1/program-approvals/${program.approval_id}`, result: encodeProgramApprovalDetail(program) },
    { method: "POST", path: `/v1/program-approvals/${program.approval_id}/disclosure`,
      result: encodeProgramApprovalDisclosure({
        approval_id: program.approval_id, held_digest: program.held_digest,
        decision: "approve", confirms: digest, evidence: program.evidence,
      }), inspect: (body) => {
        const request = decodeProgramApprovalDisclosureRequest(body, "disclosure request");
        assert.equal(request.held_digest, program.held_digest);
        assert.equal(request.decision, "approve");
        assert.ok(request.idempotency_key.length > 0);
      } },
    ...stepUpExchanges(challenge, evidence, credential),
    { method: "POST", path: `/v1/program-approvals/${program.approval_id}/approve`,
      body: { held_digest: program.held_digest, step_up_evidence: challengeId }, result: encodeProgramApprovalDecision(decision) },
  ], async (client) => {
    const completed = await performStepUp(client, digest, async () => credential);
    assert.equal(stepUpEvidenceReference(completed), challenge.challenge_id);
    assert.equal((await client.approvalApprove(ordinary.approval_id, {
      step_up_evidence: stepUpEvidenceReference(completed),
    }, idempotencyKey)).state, "approved");
    const approvals = new Approvals({ client, authenticator: async () => credential });
    const detail = await approvals.programDetail(program.approval_id);
    assert.equal((await approvals.decideProgram(detail, "approve")).state, "approved");
  });
}

function arrayBuffer(bytes: Uint8Array): ArrayBuffer {
  const result = new ArrayBuffer(bytes.length);
  new Uint8Array(result).set(bytes);
  return result;
}

function signedAssertion() {
  const pair = generateKeyPairSync("ed25519");
  const clientData = Buffer.from(JSON.stringify({
    type: "webauthn.get", challenge: "AAECAwQFBgcICQoLDA0ODw", origin: "https://wallet.example", crossOrigin: false,
  }));
  const authenticatorData = Buffer.concat([
    createHash("sha256").update("wallet.example").digest(), Buffer.from([5, 0, 0, 0, 1]),
  ]);
  const signed = Buffer.concat([authenticatorData, createHash("sha256").update(clientData).digest()]);
  const signature = sign(null, signed, pair.privateKey);
  const rawId = arrayBuffer(Buffer.from("00112233445566778899aabbccddeeff", "hex"));
  const response = {
    clientDataJSON: arrayBuffer(clientData), authenticatorData: arrayBuffer(authenticatorData),
    signature: arrayBuffer(signature), userHandle: arrayBuffer(Buffer.from("principal-fixture")),
  };
  return { credential: serializeAssertionCredential({ rawId }, response), rawId, response, publicKey: pair.publicKey };
}

async function stepup_webauthn_shape_real_finish() {
  assert.equal(browserPasskeyAuthenticator(), performAssertionCeremony);
  const fixture = signedAssertion();
  const assertion = record(JSON.parse(Buffer.from(fixture.credential, "base64url").toString("utf8")) as unknown);
  assert.deepEqual(Object.keys(assertion).sort(), ["authenticatorData", "clientDataJSON", "id", "signature", "userHandle"]);
  assert.equal(assertion["id"], Buffer.from(fixture.rawId).toString("base64url"));
  for (const field of ["authenticatorData", "clientDataJSON", "signature", "userHandle"] as const) {
    assert.equal(assertion[field], Buffer.from(fixture.response[field]).toString("base64url"));
  }
  const withoutUser = record(JSON.parse(Buffer.from(serializeAssertionCredential(
    { rawId: fixture.rawId }, { ...fixture.response, userHandle: null },
  ), "base64url").toString("utf8")) as unknown);
  assert.equal("userHandle" in withoutUser, false);

  const { challenge, evidence } = stepUpValues();
  await withHttpPeer([
    { method: "POST", path: `/v1/step-up/${challengeId}`, result: encodeStepUpEvidence(evidence), inspect: (body) => {
      assert.equal(decodeStepUpFinish(body, "real HTTP finish body").credential, fixture.credential);
    } },
  ], async (client) => {
    const completed = await client.stepupFinish(challengeId, { credential: fixture.credential });
    assert.equal(completed.challenge_id, challenge.challenge_id);
    assert.equal(completed.confirms, digest);
    assert.equal(stepUpEvidenceReference(completed), challengeId);
  });

  // This checks that serialization preserves signed bytes; it is not the
  // service's passkey-auth verifier and cannot close its integration proof.
  function signatureValid(value: Record<string, unknown>): boolean {
    const data = Buffer.concat([
      Buffer.from(text(value["authenticatorData"]), "base64url"),
      createHash("sha256").update(Buffer.from(text(value["clientDataJSON"]), "base64url")).digest(),
    ]);
    return verify(null, data, fixture.publicKey, Buffer.from(text(value["signature"]), "base64url"));
  }
  assert.equal(signatureValid(assertion), true);
  const clientData = record(JSON.parse(Buffer.from(text(assertion["clientDataJSON"]), "base64url").toString("utf8")) as unknown);
  const mutations = [
    { ...assertion, signature: Buffer.alloc(64).toString("base64url") },
    { ...assertion, clientDataJSON: Buffer.from(JSON.stringify({ ...clientData, origin: "https://invalid.example" })).toString("base64url") },
    { ...assertion, clientDataJSON: Buffer.from(JSON.stringify({ ...clientData, challenge: "unbound-challenge" })).toString("base64url") },
    { ...assertion, authenticatorData: Buffer.concat([Buffer.alloc(32), Buffer.from(fixture.response.authenticatorData).subarray(32)]).toString("base64url") },
  ];
  for (const mutation of mutations) assert.equal(signatureValid(mutation), false);
}

async function stepup_timestamps_real_begin_finish_expiry_decision() {
  const { challenge, evidence } = stepUpValues();
  const credential = signedAssertion().credential;
  timestamp(challenge.expires_at);
  assert.ok(timestamp(evidence.completed_at) < timestamp(evidence.expires_at));
  const program = decodeProgramApprovalDetail(await golden("approval.program.get"), "timestamp decision fixture");
  await withHttpPeer([
    ...stepUpExchanges(challenge, evidence, credential),
    { method: "POST", path: `/v1/program-approvals/${program.approval_id}/approve`,
      body: { held_digest: program.held_digest, step_up_evidence: challengeId },
      result: encodeProgramApprovalDecision({
        approval_id: program.approval_id, held_digest: program.held_digest, state: "approved",
        state_copy_key: "approval.state.approved", money_moved: false,
        moved_copy_key: "approval.decision.no-money-moved", evidence: program.evidence,
        release_ref: "release-fixture",
      }) },
  ], async (client) => {
    const completed = await performStepUp(client, digest, async () => credential);
    assert.deepEqual(completed, evidence);
    const decision = await client.approvalProgramApprove(program.approval_id, {
      held_digest: program.held_digest, step_up_evidence: stepUpEvidenceReference(completed),
    }, idempotencyKey);
    assert.equal(decision.state, "approved");
    assert.equal(decision.held_digest, program.held_digest);
  });
  const expired = { ...challenge, expires_at: new Date(Date.now() - 1_000).toISOString() };
  await withHttpPeer([
    { method: "POST", path: "/v1/step-up", result: encodeStepUpChallenge(expired) },
  ], async (client) => {
    await assert.rejects(performStepUp(client, digest, async () => {
      assert.fail("an expired challenge must be refused before asking for a credential");
    }), /challenge is not bound/u);
  });
  for (const invalid of [
    { ...evidence, completed_at: "invalid" },
    { ...evidence, completed_at: new Date(timestamp(challenge.expires_at) + 1_000).toISOString() },
    { ...evidence, expires_at: evidence.completed_at },
    { ...evidence, expires_at: new Date(Date.now() - 1_000).toISOString() },
    { ...evidence, confirms: `opd_${"34".repeat(32)}` },
    { ...evidence, challenge_id: "chg_ffeeddccbbaa99887766554433221100" },
  ]) {
    await withHttpPeer(stepUpExchanges(challenge, invalid, credential), async (client) => {
      await assert.rejects(performStepUp(client, digest, async () => credential), /evidence is not bound/u);
    });
  }
  const projection = await readFile(new URL(
    "services/human/crates/layerx-human-service/src/server/identity_projection.rs", repository,
  ), "utf8");
  assert.match(projection, /"expires_at":unix_time\(value.expires_at\)/u);
  assert.match(projection, /"completed_at":unix_time\(value.completed_at\(\)\)/u);
  assert.match(projection, /"expires_at":unix_time\(value.expires_at\(\)\)/u);
  assert.match(projection, /seconds_from_rfc3339\(&result\) != Some\(seconds\)/u);
  const dispatcher = await readFile(new URL(
    "services/human/crates/layerx-human-service/src/server/identity_dispatch.rs", repository,
  ), "utf8");
  assert.match(dispatcher, /api_projection::step_up_challenge\(value\)/u);
  assert.match(dispatcher, /api_projection::step_up_evidence\(value\)/u);
  assert.doesNotMatch(dispatcher, /^(?:<<<<<<<|=======|>>>>>>>)/mu);
}

async function journey_state_consumers_real_http() {
  const initial = decodeJourney(await golden("journey.get"), "committed journey fixture");
  const activity = decodeActivityEntryDetail(await golden("activity.entry"), "committed activity fixture");
  const expected: Readonly<Record<JourneyState, StatusKey>> = {
    "getting-ready": "getting_ready",
    sending: "sending",
    processing: "processing",
    done: "done",
    "done-finalised": "done_finalised",
    "still-checking": "still_checking",
    refused: "refused",
    "waiting-for-you": "waiting_for_you",
    "awaiting-approval": "waiting_for_you",
    closed: "refused",
  };
  for (const state of journeyStateVariants) {
    const stage = { stage_id: "status-stage", copy_key: "move.title", state, evidence: [] };
    const projection = { ...initial, state, stages: [stage], evidence: [] };
    await withHttpPeer([
      { method: "GET", path: `/v1/journeys/${initial.journey_id}`, result: encodeJourney(projection) },
    ], async (client) => {
      const journey = await client.journeyGet(initial.journey_id);
      assert.equal(journey.state, state);
      for (const status of [entryStatusKey, agentJourneyStatus, approvalJourneyStatus, onboardingJourneyStatus]) {
        assert.equal(status(journey.state), expected[state]);
      }
      assert.equal(stageView(journey.stages[0]!).status, expected[state]);
      assert.equal(stagePresentation(journey.stages[0]!).status, expected[state]);
      assert.equal(journeyProgress(journey).statusKey, expected[state]);
      assert.equal(journeyProgress(journey).complete, false);
      const result = moveResult(journey);
      assert.equal(result.journeyId, initial.journey_id);
      assert.notEqual(result.kind, "done", "states without receipt evidence cannot claim a successful movement");
      const status = state === "done" || state === "done-finalised" ? "processing" : expected[state];
      assert.equal(result.status, status);
      assert.equal(result.stages[0]!.status, status);
      assert.equal(homeActivityRows([{ ...activity, state, stages: [], evidence: [], kind: "movement" }])[0]!.status, status);
      if (state === "awaiting-approval") assert.equal(result.kind, "waiting");
      if (state === "closed") {
        assert.equal(result.kind, "refused", "closed history cannot resume execution or polling");
        assert.equal(stagePresentation(stage).done, false);
      }
    });
  }
}

test(approval_operation_digest_real_http);
test(stepup_reference_real_http_ordinary_and_program);
test(stepup_webauthn_shape_real_finish);
test(stepup_timestamps_real_begin_finish_expiry_decision);
test(journey_state_consumers_real_http);
