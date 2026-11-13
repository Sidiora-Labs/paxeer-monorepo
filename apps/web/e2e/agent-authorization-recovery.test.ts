import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { registerHooks } from "node:module";
import type { AddressInfo } from "node:net";
import { test, type TestContext } from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

import {
  createHumanApiClient,
  decodeAgent,
  decodeAgentCreateRequest,
  decodeJourney,
  decodeMoveQuote,
  decodeNativeSendAccessRequest,
  decodeOwnerRotationRequest,
  encodeJourney,
  encodeMoveQuote,
  type HumanApiClient,
  type JsonValue,
} from "../src/api/index.ts";
import { Agents, journeyProgress, type CreationDraft } from "../src/journeys/agents/model.ts";
import {
  agentRecoveryKey,
  type AgentRecoveryStorage,
  type PendingAgentOperation,
} from "../src/journeys/agents/recovery.ts";

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
    if (specifier === "server-only") return nextResolve("next/dist/compiled/server-only/empty.js", context);
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js")
          ? new URL(candidate.href.slice(0, -3) + suffix) : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    if (specifier === "next/navigation") return nextResolve("next/navigation.js", context);
    if (specifier === "next/link") return nextResolve("next/link.js", context);
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      return { format: "module", shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX },
        }).outputText };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

function fixture(name: string, field: "body" | "result" = "result"): JsonValue {
  const value = JSON.parse(readFileSync(new URL(
    `../../../services/human/schema/human-api/golden/${name}.json`, import.meta.url,
  ), "utf8")) as { body: JsonValue & { result: JsonValue } };
  return field === "body" ? value.body : value.body.result;
}

class PrivateMemoryStore implements AgentRecoveryStorage {
  readonly records = new Map<string, string>();
  getItem(key: string): string | null { return this.records.get(key) ?? null; }
  setItem(key: string, value: string): void { this.records.set(key, value); }
  removeItem(key: string): void { this.records.delete(key); }
}

const account = "act_01j8agd2s3a1b2c3d4e5f6g7h8";
const agentId = "agt_01j8agd2s3a1b2c3d4e5f6g7h8";
const key = "00112233445566778899aabbccddeeff";
const recordedCreate = decodeAgentCreateRequest(fixture("agent.create.request", "body"), "committed creation request");
const draft: CreationDraft = {
  name: "Recovery Agent", purpose: "Private recovery", limitInput: "100000", currency: "LXP",
  ...(recordedCreate.native_fee_budget === undefined ? {} : { nativeFeeBudget: recordedCreate.native_fee_budget }),
};

interface AcceptedOperation {
  readonly principal: string;
  readonly path: string;
  readonly key: string;
  readonly body: JsonValue | undefined;
  readonly result: JsonValue;
}

async function backend(t: TestContext, storage: AgentRecoveryStorage) {
  let principal = account;
  let dropResponse = true;
  let failJourneyRead = false;
  let replayGate: Promise<void> | undefined;
  const accepted = new Map<string, AcceptedOperation>();
  const posts: AcceptedOperation[] = [];
  const errors: unknown[] = [];
  const journeys = new Map<string, JsonValue>();
  const reads: string[] = [];
  const server = createServer((request, response) => { void serve(request, response); });
  async function serve(request: IncomingMessage, response: ServerResponse) {
    try {
      const path = request.url!;
      const send = (result: JsonValue) => {
        response.writeHead(200, { "Content-Type": "application/json", Connection: "close" });
        response.end(JSON.stringify({ ok: true, result, trace: "trc_agent_recovery_recording" }));
      };
      if (request.method === "GET") {
        reads.push(path);
        if (path === "/v1/account/balance") {
          const balance = fixture("account.balance.response");
          assert.ok(typeof balance === "object" && balance !== null && !Array.isArray(balance));
          send({ ...balance, account_id: principal });
          return;
        }
        if (path.startsWith("/v1/journeys/")) {
          if (failJourneyRead) { response.destroy(); return; }
          const journey = journeys.get(path.slice("/v1/journeys/".length));
          assert.ok(journey !== undefined);
          send(journey);
          return;
        }
        assert.fail(`unexpected GET ${path}`);
      }
      assert.equal(request.method, "POST");
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const bytes = Buffer.concat(chunks).toString("utf8");
      const body = bytes.length === 0 ? undefined : JSON.parse(bytes) as JsonValue;
      const idempotencyKey = request.headers["idempotency-key"];
      assert.equal(typeof idempotencyKey, "string");
      const scope = `${principal}:${idempotencyKey as string}`;
      let operation = accepted.get(scope);
      if (operation === undefined) {
        let result: JsonValue;
        let target = agentId;
        if (path === "/v1/agents") {
          decodeAgentCreateRequest(body, "actual creation submission");
          result = fixture("agent.create.response");
          target = "create";
        } else if (path === "/v1/moves") {
          result = fixture("agent.reclaim.response");
        } else {
          const action = path.slice(path.lastIndexOf("/") + 1);
          const names: Record<string, string> = {
            pause: "agent.pause", resume: "agent.resume", limit: "agent.limit",
            reclaim: "agent.reclaim", archive: "agent.archive", rotate: "agent.rotate",
            recover: "agent.recover", rotation: "agent.rotation.start",
          };
          assert.ok(names[action], `unexpected mutation ${path}`);
          if (action === "rotation") decodeOwnerRotationRequest(body, "actual prepared rotation submission");
          result = fixture(`${names[action]}.response`);
        }
        const persisted = storage.getItem(await agentRecoveryKey(principal, target));
        assert.ok(persisted, "operation identity must be durable before the producer receives the request");
        const pending = JSON.parse(persisted) as PendingAgentOperation;
        assert.equal(pending.key, idempotencyKey);
        assert.equal(pending.principal, principal);
        operation = { principal, path, key: idempotencyKey as string, body, result };
        accepted.set(scope, operation);
        if (typeof result === "object" && result !== null && !Array.isArray(result) && typeof result["journey_id"] === "string") {
          journeys.set(result["journey_id"], result);
        }
      } else {
        assert.equal(path, operation.path, "reconciliation must address the original operation");
        assert.deepEqual(body, operation.body, "reconciliation must reuse the exact reviewed request");
        await replayGate;
      }
      posts.push(operation);
      if (dropResponse) { dropResponse = false; response.destroy(); return; }
      send(operation.result);
    } catch (error) {
      errors.push(error);
      response.writeHead(500).end();
    }
  }
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  t.after(async () => {
    await new Promise<void>((resolve, reject) => {
      server.close((error) => { if (error) reject(error); else resolve(); });
      server.closeAllConnections();
    });
    assert.deepEqual(errors, []);
  });
  const baseUrl = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  return {
    client: () => createHumanApiClient({ baseUrl }), accepted, posts, journeys, reads,
    switchPrincipal: (value: string) => { principal = value; },
    loseNextResponse: () => { dropResponse = true; },
    failJourneyReads: (value: boolean) => { failJourneyRead = value; },
    holdReplayResponse: () => {
      let release!: () => void;
      replayGate = new Promise<void>((resolve) => { release = resolve; });
      return release;
    },
  };
}

function reopened(client: HumanApiClient, storage: AgentRecoveryStorage): Agents {
  return new Agents({ client, storage, idempotencyKey: () => assert.fail("Reopening must not allocate a replacement identity") });
}

async function agent_uncertain_reopen_creation_result_real_http(t: TestContext) {
  const storage = new PrivateMemoryStore();
  const peer = await backend(t, storage);
  await assert.rejects(new Agents({ client: peer.client(), storage, idempotencyKey: () => key }).create(draft));
  assert.equal(peer.accepted.size, 1);
  const openAgain = reopened(peer.client(), storage);
  await assert.rejects(openAgain.create({ ...draft, limitInput: "200000" }), /Reconcile/);
  assert.equal(peer.posts.length, 1, "a changed review cannot replace an uncertain request");
  const result = await openAgain.recoverPending();
  assert.equal(result?.kind, "journey");
  assert.ok(result?.kind === "journey");
  assert.equal(result.value.journey_id, decodeJourney(fixture("agent.create.response"), "recording").journey_id);
  assert.equal(journeyProgress(result.value).complete, false);
  assert.equal(peer.posts.length, 2);
  assert.equal(peer.accepted.size, 1);
  const recoveredAfterAnotherReload = await reopened(peer.client(), storage).recoverPending();
  assert.deepEqual(recoveredAfterAnotherReload, result);
  assert.equal(peer.posts.length, 2, "a known journey is recovered with a read, not another submission");
  peer.failJourneyReads(true);
  await assert.rejects(reopened(peer.client(), storage).recoverPending());
  await assert.rejects(openAgain.create(draft));
  assert.equal(peer.posts.length, 2);
  peer.failJourneyReads(false);
  const completed = { ...result.value, state: "done-finalised" as const,
    stages: result.value.stages.map((stage) => ({ ...stage, state: "done-finalised" as const,
      evidence: [{ evidence_id: "evd_recovered_committed", class: "layerx-receipt" as const, verification: "receipt-verified" as const }] })) };
  peer.journeys.set(completed.journey_id, encodeJourney(completed));
  const final = await reopened(peer.client(), storage).recoverPending();
  assert.ok(final?.kind === "journey");
  assert.equal(journeyProgress(final.value).complete, true);
  assert.equal(storage.records.size, 0);
  await new Agents({ client: peer.client(), storage, idempotencyKey: () => "ffeeddccbbaa99887766554433221100" }).create(draft);
  assert.equal(peer.accepted.size, 2, "replacement becomes possible only after authoritative reconciliation");
}

async function agent_uncertain_reopen_controls_real_http(t: TestContext) {
  const operations = [
    (agents: Agents) => agents.pause(agentId),
    (agents: Agents) => agents.resume(agentId),
    (agents: Agents) => agents.changeLimit(agentId, { amount: 12345678901234567890n, currency: "LXP" }),
    (agents: Agents) => agents.reclaim(agentId, { amount: 17n, currency: "LXP" }),
    (agents: Agents) => agents.archive(agentId, "Recovery Agent"),
    (agents: Agents) => agents.rotate(agentId),
    (agents: Agents) => agents.recover(agentId),
  ];
  for (const operation of operations) {
    const storage = new PrivateMemoryStore();
    const peer = await backend(t, storage);
    await assert.rejects(operation(new Agents({ client: peer.client(), storage, idempotencyKey: () => key })));
    const recovered = await reopened(peer.client(), storage).recoverPending(agentId);
    assert.ok(recovered !== undefined);
    assert.equal(peer.posts.length, 2);
    assert.deepEqual(peer.posts[0], peer.posts[1]);
    assert.equal(peer.accepted.size, 1);
    if (recovered.kind === "journey") assert.equal(journeyProgress(recovered.value).journeyId.length > 0, true);
    else assert.equal(recovered.value.agent_id, agentId);
  }
}

async function agent_uncertain_reopen_prepared_authorizations_real_http(t: TestContext) {
  const rotation = decodeOwnerRotationRequest(fixture("agent.rotation.start.request", "body"), "committed prepared rotation");
  const quote = {
    ...decodeMoveQuote(fixture("move.quote.response"), "committed funding quote"),
    native_send_access: decodeNativeSendAccessRequest(fixture("native.send.access.begin.request", "body"), "committed native access request"),
  };
  const access = fixture("native.send.access.confirm.response");
  assert.ok(typeof access === "object" && access !== null && !Array.isArray(access));
  const nativeAccessId = access["native_access_id"];
  assert.equal(typeof nativeAccessId, "string");
  for (const mode of ["rotation", "fund"] as const) {
    const storage = new PrivateMemoryStore();
    const peer = await backend(t, storage);
    const pending: PendingAgentOperation = {
      version: 1, principal: account, target: agentId, key,
      scope: mode === "rotation" ? JSON.stringify(["rotation.start", agentId, "86400", "3600"])
        : JSON.stringify(["fund", quote.quote_id]),
      ...(mode === "rotation" ? { rotationRequest: fixture("agent.rotation.start.request", "body") }
        : { request: encodeMoveQuote(quote), fundingAccess: nativeAccessId as string }),
    };
    storage.setItem(await agentRecoveryKey(account, agentId), JSON.stringify(pending));
    const first = reopened(peer.client(), storage);
    await assert.rejects(mode === "rotation" ? first.startRotation(agentId, rotation) : first.fundCommit(quote, agentId));
    const retained = JSON.parse(storage.getItem(await agentRecoveryKey(account, agentId))!) as PendingAgentOperation;
    assert.deepEqual(retained, pending, "lost responses retain prepared authorization and original identity");
    const result = await reopened(peer.client(), storage).recoverPending(agentId);
    assert.equal(result?.kind, mode === "rotation" ? "challenge" : "journey");
    assert.equal(peer.accepted.size, 1);
    assert.equal(peer.posts.length, 2);
    assert.deepEqual(peer.posts[0]?.body, peer.posts[1]?.body);
    if (mode === "fund") assert.deepEqual(peer.posts[1]?.body, { quote_id: quote.quote_id, native_access_id: nativeAccessId });
    assert.ok(peer.reads.every((path) => path === "/v1/account/balance"), "reopening uses the saved authorization without another ceremony");
  }
}

async function agent_uncertain_reopen_principal_isolation(t: TestContext) {
  const storage = new PrivateMemoryStore();
  const peer = await backend(t, storage);
  const original = new Agents({ client: peer.client(), storage, idempotencyKey: () => key });
  await assert.rejects(original.create(draft));
  const originalRecord = [...storage.records.entries()][0];
  peer.switchPrincipal("act_other_authenticated_principal");
  assert.equal(await reopened(peer.client(), storage).recoverPending(), undefined);
  await assert.rejects(original.create(draft), /principal changed/);
  assert.equal(peer.posts.length, 1);
  assert.deepEqual([...storage.records.entries()][0], originalRecord);
  await new Agents({ client: peer.client(), storage, idempotencyKey: () => key }).create(draft);
  assert.equal(peer.accepted.size, 2, "the same opaque key is isolated by authenticated principal");
  peer.switchPrincipal(account);
  assert.equal((await reopened(peer.client(), storage).recoverPending())?.kind, "journey");
  assert.equal(peer.accepted.size, 2);
}

async function agent_uncertain_reopen_storage_failure_blocks_submission(t: TestContext) {
  const storage = new PrivateMemoryStore();
  const peer = await backend(t, storage);
  const unavailable: AgentRecoveryStorage = {
    getItem: (name) => storage.getItem(name), removeItem: (name) => storage.removeItem(name),
    setItem: () => { throw new Error("Private storage quota exhausted"); },
  };
  await assert.rejects(new Agents({ client: peer.client(), storage: unavailable }).create(draft), /quota exhausted/);
  assert.equal(peer.posts.length, 0);
  storage.setItem(await agentRecoveryKey(account, "create"), "corrupt private record");
  await assert.rejects(reopened(peer.client(), storage).recoverPending());
  await assert.rejects(new Agents({ client: peer.client(), storage }).create(draft));
  assert.equal(peer.posts.length, 0, "corrupt state must block replacement rather than be silently discarded");
}

async function agent_uncertain_reopen_control_owner(t: TestContext) {
  const { Window } = await import("happy-dom");
  const { createElement, act } = await import("react");
  const browser = new Window({ url: "https://wallet.example/app/agents" });
  const globals = { window: browser, document: browser.document, navigator: browser.navigator, IS_REACT_ACT_ENVIRONMENT: true };
  const previous = new Map(Object.keys(globals).map((name) => [name, Object.getOwnPropertyDescriptor(globalThis, name)]));
  for (const [name, value] of Object.entries(globals)) Object.defineProperty(globalThis, name, { configurable: true, writable: true, value });
  t.after(async () => {
    for (const [name, descriptor] of previous) {
      if (descriptor === undefined) Reflect.deleteProperty(globalThis, name);
      else Object.defineProperty(globalThis, name, descriptor);
    }
    await browser.happyDOM.close();
  });
  const { createRoot } = await import("react-dom/client");
  const { AgentControls } = await import("../src/journeys/agents/controls.tsx");
  const { PrivacyModeProvider } = await import("../src/settings/privacy.tsx");
  const storage = browser.localStorage;
  const peer = await backend(t, storage);
  const container = browser.document.createElement("div");
  browser.document.body.append(container);
  let renderedAgent = decodeAgent(fixture("agent.get.response"), "committed agent");
  async function mount(agents: Agents) {
    const root = createRoot(container as unknown as HTMLElement);
    await act(async () => {
      root.render(createElement(PrivacyModeProvider, { principalScope: "recovery-owner", children:
        createElement(AgentControls, { shell: "desktop", agent: renderedAgent, agents, ownerAccount: account,
          onAgent: (agent) => { renderedAgent = agent; }, onChanged: () => {} }) }));
    });
    return root;
  }
  let root = await mount(new Agents({ client: peer.client(), storage, idempotencyKey: () => key }));
  await act(async () => { root.unmount(); });
  await assert.rejects(new Agents({ client: peer.client(), storage, idempotencyKey: () => key }).pause(agentId));
  const releaseResponse = peer.holdReplayResponse();
  root = await mount(reopened(peer.client(), storage));
  try {
    assert.ok(container.querySelectorAll("button[disabled]").length > 0, "reopened controls lock while the saved operation is reconciled");
    releaseResponse();
    const deadline = Date.now() + 5_000;
    while (renderedAgent.state !== "paused" && Date.now() < deadline) {
      await act(async () => { await new Promise((resolve) => setTimeout(resolve, 10)); });
    }
    assert.equal(renderedAgent.state, "paused", "the reopened real owner consumes the authoritative recovered agent");
    assert.equal(peer.accepted.size, 1);
    assert.equal(peer.posts.length, 2);
  } finally {
    releaseResponse();
    await act(async () => { root.unmount(); });
  }
}

test(agent_uncertain_reopen_creation_result_real_http);
test(agent_uncertain_reopen_controls_real_http);
test(agent_uncertain_reopen_prepared_authorizations_real_http);
test(agent_uncertain_reopen_principal_isolation);
test(agent_uncertain_reopen_storage_failure_blocks_submission);
test(agent_uncertain_reopen_control_owner);
