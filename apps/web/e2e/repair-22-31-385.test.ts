import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createServer, type Server } from "node:http";
import type { AddressInfo } from "node:net";
import test from "node:test";

import { copyEntry } from "../copy/runtime.ts";
import { createHumanApiClient, type Journey, type JsonValue } from "../src/api/generated/index.ts";
import { ACTIVE_ACCOUNT_STORAGE_KEY } from "../src/auth/session.ts";
import {
  accountActive,
  completedStageIds,
  PROTOCOL_IDENTITY_STAGE_KEY,
  stagePresentation,
} from "../src/journeys/onboarding/model.ts";
import {
  pendingOnboarding,
  rememberActivatedAccount,
  rememberPending,
  type OnboardingStorage,
} from "../src/journeys/onboarding/recovery.ts";

const repository = new URL("../../../", import.meta.url);
const goldenRoot = new URL("services/human/schema/human-api/golden/", repository);
const dispatcherPath = "services/human/crates/layerx-human-service/src/server/identity_dispatch.rs";

class BrowserStorage implements OnboardingStorage {
  private readonly rows: Map<string, string>;

  constructor(rows: Map<string, string>) {
    this.rows = rows;
  }

  getItem(key: string): string | null {
    return this.rows.get(key) ?? null;
  }

  setItem(key: string, value: string): void {
    this.rows.set(key, value);
  }

  removeItem(key: string): void {
    this.rows.delete(key);
  }
}

interface GoldenResponse {
  status: number;
  body: JsonValue;
}

async function goldenResponse(name: string): Promise<GoldenResponse> {
  return JSON.parse(await readFile(new URL(`${name}.response.json`, goldenRoot), "utf8")) as GoldenResponse;
}

async function listen(server: Server): Promise<string> {
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      server.off("error", reject);
      resolve();
    });
  });
  return `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
}

async function close(server: Server): Promise<void> {
  await new Promise<void>((resolve, reject) => {
    server.close((error) => {
      if (error === undefined) resolve();
      else reject(error);
    });
    server.closeAllConnections();
  });
}

async function repair_22_31_385_producer_copy_contract_matches_consumer() {
  const source = await readFile(new URL(dispatcherPath, repository), "utf8");
  const declaration = source.match(/fn onboarding_stage_copy_key\([^]*?\n\}/u)?.[0];
  assert.ok(declaration, "the production dispatcher must declare the onboarding copy projection");
  const keys = new Map(Array.from(
    declaration.matchAll(/OnboardingStage::(\w+)\s*=>\s*(?:\{\s*)?"([^"]+)"/gu),
    (match) => [match[1]!, match[2]!],
  ));
  assert.deepEqual([...keys.keys()], [
    "ApplicationIdentity", "CustodyKey", "DidRegistration", "InitialFunding", "RecoveryRegistration",
  ]);
  assert.equal(keys.get("DidRegistration"), PROTOCOL_IDENTITY_STAGE_KEY);
  assert.deepEqual([...keys.values()].map((key) => copyEntry(key).message), [
    "Creating your account", "Adding your passkey", "Activating your account",
    "Adding its first funds", "Putting recovery in place",
  ]);

  const identity = await readFile(new URL(
    "services/human/crates/layerx-human-service/src/server/identity.rs", repository,
  ), "utf8");
  assert.match(identity, /identity_dispatch::onboarding_status\(scope, value\)/u);
  const handlers = await readFile(new URL(
    "services/human/crates/layerx-human-service/src/server/production_components.rs", repository,
  ), "utf8");
  for (const name of ["execute_onboarding_status", "execute_onboarding_resume"]) {
    const handler = handlers.split(`fn ${name}(`)[1]?.split("\n    fn ")[0];
    assert.ok(handler, name);
    assert.match(handler, /IdentityProjector::onboarding\(/u, `${name} uses the checked producer`);
  }
  assert.match(source, /journey\s*\.activation_evidence\(scope\)/u,
    "projection checks the durable evidence before exposing receipt-backed activation");
}

async function repair_22_31_385_pending_reopens_and_receipt_activation_over_http() {
  const creationFixture = await goldenResponse("account.create");
  const statusFixture = await goldenResponse("onboarding.status");
  const resumeFixture = await goldenResponse("onboarding.resume");
  const retained = new Map<string, string>();
  const requests: string[] = [];
  function fixtureServer(): Server {
    return createServer((request, response) => {
      const route = `${request.method} ${request.url}`;
      requests.push(route);
      const fixture = route === "POST /v1/accounts" ? creationFixture
        : route === "GET /v1/onboarding" ? statusFixture
          : route === "POST /v1/onboarding/resume" ? resumeFixture : undefined;
      if (fixture === undefined) {
        response.writeHead(404).end();
        return;
      }
      request.resume();
      request.once("end", () => {
        response.writeHead(fixture.status, { "Content-Type": "application/json" });
        response.end(JSON.stringify(fixture.body));
      });
    });
  }

  let server = fixtureServer();
  let baseUrl = await listen(server);
  try {
    let api = createHumanApiClient({ baseUrl });
    const creation = await api.accountCreate(
      { email: "mara@example.com", display_name: "Mara Okafor" },
      "9f8b6d4c2e0a1b3c5d7e9f0a2b4c6d8e",
    );
    let storage = new BrowserStorage(retained);
    rememberPending({
      version: 1,
      idempotencyKey: "9f8b6d4c2e0a1b3c5d7e9f0a2b4c6d8e",
      email: "mara@example.com",
      displayName: "Mara Okafor",
      accountId: creation.account_id,
      passkeyAdded: true,
    }, storage);
    rememberActivatedAccount(creation.onboarding, storage);
    assert.equal(storage.getItem(ACTIVE_ACCOUNT_STORAGE_KEY), null);
    assert.equal(accountActive(creation.onboarding), false);

    const pending = await api.onboardingStatus();
    assert.equal(pending.journey_id, creation.onboarding.journey_id);
    assert.equal(pending.started_at, creation.onboarding.started_at);
    assert.equal(accountActive(pending), false);
    assert.equal(pending.stages.map(stagePresentation)[2]?.title, "Activating your account");
    const beforeRestart = pendingOnboarding(storage);
    assert.ok(beforeRestart);
    rememberActivatedAccount(pending, storage);
    assert.deepEqual(pendingOnboarding(storage), beforeRestart);
    const savedStages = [...completedStageIds(pending)];

    await close(server);
    server = fixtureServer();
    baseUrl = await listen(server);
    api = createHumanApiClient({ baseUrl });
    storage = new BrowserStorage(retained);
    assert.deepEqual(pendingOnboarding(storage), beforeRestart);
    const recovered = await api.onboardingStatus();
    assert.deepEqual(recovered, pending);
    assert.deepEqual([...completedStageIds(recovered)], savedStages);
    assert.equal(storage.getItem(ACTIVE_ACCOUNT_STORAGE_KEY), null);

    const activated = await api.onboardingResume();
    assert.equal(activated.journey_id, pending.journey_id);
    assert.equal(activated.started_at, pending.started_at);
    assert.ok(Date.parse(activated.updated_at) > Date.parse(pending.updated_at));
    assert.deepEqual(activated.stages.map((stage) => stage.stage_id), pending.stages.map((stage) => stage.stage_id));
    const identity = activated.stages.find((stage) => stage.copy_key === PROTOCOL_IDENTITY_STAGE_KEY);
    assert.ok(identity);
    assert.equal(identity.state, "done");
    assert.deepEqual(identity.evidence.map((item) => [item.class, item.verification]), [
      ["layerx-receipt", "receipt-verified"],
    ]);
    assert.equal(activated.state, "processing", "account activation precedes recovery completion");
    assert.equal(accountActive(activated), true);
    for (const id of savedStages) assert.ok(completedStageIds(activated).has(id));
    rememberActivatedAccount(activated, storage);
    assert.equal(storage.getItem(ACTIVE_ACCOUNT_STORAGE_KEY), creation.account_id);
    assert.equal(pendingOnboarding(storage), undefined);
    const reopenedBrowser = new BrowserStorage(retained);
    assert.equal(reopenedBrowser.getItem(ACTIVE_ACCOUNT_STORAGE_KEY), creation.account_id);
    assert.equal(pendingOnboarding(reopenedBrowser), undefined);
    assert.deepEqual(requests, [
      "POST /v1/accounts", "GET /v1/onboarding", "GET /v1/onboarding", "POST /v1/onboarding/resume",
    ]);
  } finally {
    await close(server);
  }
}

async function repair_22_31_385_receipt_requirement_survives_reopen() {
  const fixture = await goldenResponse("onboarding.resume");
  const server = createServer((request, response) => {
    request.resume();
    response.writeHead(fixture.status, { "Content-Type": "application/json" });
    response.end(JSON.stringify(fixture.body));
  });
  const baseUrl = await listen(server);
  try {
    const activated = await createHumanApiClient({ baseUrl }).onboardingResume();
    function refusesActivation(journey: Journey): void {
      const rows = new Map<string, string>();
      const storage = new BrowserStorage(rows);
      rememberPending({
        version: 1, idempotencyKey: "retained-key", email: "mara@example.com",
        displayName: "Mara Okafor", accountId: "retained-account", passkeyAdded: true,
      }, storage);
      const before = pendingOnboarding(storage);
      assert.equal(accountActive(journey), false);
      rememberActivatedAccount(journey, storage);
      const reopened = new BrowserStorage(rows);
      assert.deepEqual(pendingOnboarding(reopened), before);
      assert.equal(reopened.getItem(ACTIVE_ACCOUNT_STORAGE_KEY), null);
    }
    for (const verification of ["unverified", "paxeer-finalised", "settlement-anchored"] as const) {
      const unverified = structuredClone(activated);
      const stage = unverified.stages.find((item) => item.copy_key === PROTOCOL_IDENTITY_STAGE_KEY);
      assert.ok(stage?.evidence[0]);
      stage.evidence[0].verification = verification;
      refusesActivation(unverified);
    }
    const evidenceOutsideIdentity = structuredClone(activated);
    const identity = evidenceOutsideIdentity.stages.find((item) => item.copy_key === PROTOCOL_IDENTITY_STAGE_KEY);
    assert.ok(identity);
    evidenceOutsideIdentity.evidence = identity.evidence;
    identity.evidence = [];
    refusesActivation(evidenceOutsideIdentity);
    const noIdentity = structuredClone(activated);
    noIdentity.stages = noIdentity.stages.filter((stage) => stage.copy_key !== PROTOCOL_IDENTITY_STAGE_KEY);
    refusesActivation(noIdentity);
    const refused = structuredClone(activated);
    refused.state = "refused";
    refusesActivation(refused);
    for (const state of ["processing", "getting-ready", "sending", "still-checking", "refused"] as const) {
      const unfinished = structuredClone(activated);
      const stage = unfinished.stages.find((item) => item.copy_key === PROTOCOL_IDENTITY_STAGE_KEY);
      assert.ok(stage);
      stage.state = state;
      refusesActivation(unfinished);
    }
  } finally {
    await close(server);
  }
}

test(repair_22_31_385_producer_copy_contract_matches_consumer);
test(repair_22_31_385_pending_reopens_and_receipt_activation_over_http);
test(repair_22_31_385_receipt_requirement_survives_reopen);
