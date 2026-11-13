import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { createServer, type Server } from "node:http";
import { registerHooks } from "node:module";
import test, { type TestContext } from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

import { createHumanApiClient, decodeJourney, decodeSessionList, type JsonValue, type Session } from "../src/api/generated/index.ts";
import { forbiddenCopy, onboardingMessages, vocabularyPattern } from "./copy-policy.ts";
import { JourneyFixtureStore, goldenResult, jsonWire, verifyPasskeyCredential } from "./journey-fixtures.ts";
import { openPublicSession } from "./public-session.ts";
import { SoftwareAuthenticator } from "./software-authenticator.ts";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "server-only") return { url: "data:text/javascript,", shortCircuit: true };
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      return {
        format: "module", shortCircuit: true, source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX },
        }).outputText
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const { createElement } = await import("react");
const { renderToStaticMarkup } = await import("react-dom/server");
const { Approvals } = await import("../src/journeys/approvals/controller.ts");
const { ApprovalDetailCard, ApprovalOutcomePanel } = await import("../src/journeys/approvals/screens.tsx");
const { canApprove, heldDigest } = await import("../src/journeys/approvals/model.ts");
const { PrivacyModeProvider } = await import("../src/settings/privacy.tsx");
const move = await import("../src/journeys/move/model.ts");
const onboarding = await import("../src/journeys/onboarding/model.ts");
const recovery = await import("../src/journeys/onboarding/recovery.ts");
const { copyEntry } = await import("../copy/runtime.ts");
const links = await import("../src/explorer/links.ts");
const gateway = await import("../src/api/gateway.ts");

// These are repaired proofs, executed by the sole declared verify command too.
await import("./copy-retired-operation.test.ts");
await import("./explorer.spec.ts");
await import("./gateway-history.test.ts");

const ORIGIN = "https://qualification.paxeer.network";
const PASSKEY_ID = "pky_01j2h5m2we5f7g9h1j3k5m7n9p";

function source(name: string): string { return readFileSync(new URL(name, import.meta.url), "utf8"); }
function renderPrivate(element: ReturnType<typeof createElement>): string {
  return renderToStaticMarkup(createElement(PrivacyModeProvider, { principalScope: "repair-22-33-337", children: element }));
}

async function peer(t: TestContext) {
  const authenticator = new SoftwareAuthenticator(ORIGIN);
  const store = new JourneyFixtureStore(ORIGIN, authenticator.publicKey());
  const unexpected: string[] = [];
  const sessions: Session[] = [];
  let assertionVerified = false;
  let counter = 0;
  const challenge = "cHVibGljLXNlc3Npb24tY2hhbGxlbmdl";
  const ceremony = (value: unknown): string => Buffer.from(jsonWire(value)).toString("base64url");
  const answer = (result: unknown): string => jsonWire({ ok: true, result, trace: "trc_01j8agg7x8a1b2c3d4e5f6g7h8" });
  const sessionRoutes = new Set(["/human/v1/accounts", "/human/v1/passkeys/registrations", "/human/v1/passkeys/assertions", "/human/v1/sessions"]);
  function server(): Server {
    return createServer(async (request, response) => {
      try {
        let body = "";
        for await (const chunk of request) body += String(chunk);
        const path = request.url ?? "";
        const method = request.method ?? "";
        const input = body === "" ? {} : JSON.parse(body) as Record<string, unknown>;
        response.setHeader("content-type", "application/json");
        response.setHeader("connection", "close");
        if (method === "POST" && path === "/human/v1/accounts") {
          response.end(answer(goldenResult("account.create"))); return;
        }
        if (method === "POST" && path === "/human/v1/passkeys/registrations") {
          const fixture = goldenResult("passkey.register.begin") as Record<string, JsonValue>;
          response.end(answer({
            ...fixture, ceremony: ceremony({
              rp: { id: new URL(ORIGIN).hostname },
              user: { id: "YWNjb3VudA" }, challenge, pubKeyCredParams: [{ type: "public-key", alg: -8 }]
            })
          })); return;
        }
        if (method === "POST" && path.startsWith("/human/v1/passkeys/registrations/")) {
          assert.equal(typeof input.credential, "string");
          response.end(answer(goldenResult("passkey.register.finish"))); return;
        }
        if (method === "POST" && path === "/human/v1/passkeys/assertions") {
          const fixture = goldenResult("passkey.assert.begin") as Record<string, JsonValue>;
          response.end(answer({ ...fixture, ceremony: ceremony({ rpId: new URL(ORIGIN).hostname, challenge }) })); return;
        }
        if (method === "POST" && path.startsWith("/human/v1/passkeys/assertions/")) {
          assert.equal(typeof input.credential, "string");
          counter = verifyPasskeyCredential(String(input.credential), challenge, ORIGIN, authenticator.publicKey(), counter).counter;
          assertionVerified = true;
          response.end(answer(goldenResult("passkey.assert.finish"))); return;
        }
        if (method === "POST" && path === "/human/v1/sessions") {
          assert.ok(assertionVerified); assertionVerified = false;
          assert.equal(input.assertion_id, (goldenResult("passkey.assert.finish") as Record<string, JsonValue>).assertion_id);
          assert.ok(request.headers["idempotency-key"]);
          const fixture = decodeSessionList(goldenResult("session.list"), "committed session fixture");
          const session = structuredClone(fixture.sessions[sessions.length]); assert.ok(session);
          session.current = true; sessions.push(session);
          response.end(answer(session)); return;
        }
        if (!sessionRoutes.has(path)) {
          if (!sessions.some((session) => request.headers.cookie === `__Host-layerx_access=${session.session_id}`)) {
            response.writeHead(401).end(jsonWire({
              ok: false, trace: "trc_01j8agg7x8a1b2c3d4e5f6g7h8",
              error: { code: "unauthenticated", copy_key: "error.session.required", retry: "structural" }
            })); return;
          }
          const result = store.answer({
            method, path, body: body || undefined,
            idempotencyKey: typeof request.headers["idempotency-key"] === "string" ? request.headers["idempotency-key"] : undefined
          });
          if (result !== undefined) { response.writeHead(result.status).end(result.body); return; }
        }
        unexpected.push(`${method} ${path}`); response.writeHead(404).end();
      } catch (error) { unexpected.push(String(error)); response.writeHead(500).end(jsonWire({ error: String(error) })); }
    });
  }
  async function listen(server: Server, port: number): Promise<void> {
    await new Promise<void>((resolve, reject) => { server.once("error", reject); server.listen(port, "127.0.0.1", resolve); });
  }
  async function close(server: Server): Promise<void> {
    await new Promise<void>((resolve, reject) => { server.close((error) => error === undefined ? resolve() : reject(error)); server.closeAllConnections(); });
  }
  let serving = server(); await listen(serving, 0);
  const address = serving.address(); assert.ok(address !== null && typeof address !== "string");
  const origin = `http://127.0.0.1:${address.port}`;
  t.after(async () => { await close(serving); assert.deepEqual(unexpected, []); });
  const publicClient = createHumanApiClient({ baseUrl: `${origin}/human` });
  const account = await publicClient.accountCreate({ email: "mara@example.com", display_name: "Mara" }, "account-repair-22-33-337");
  const registration = await publicClient.passkeyRegisterBegin({ account_id: account.account_id });
  const registered = await publicClient.passkeyRegisterFinish(registration.registration_id, { credential: authenticator.register(registration.ceremony) });
  assert.equal(registered.passkey_id, PASSKEY_ID);
  const session = await openPublicSession(publicClient, "mara@example.com", authenticator, PASSKEY_ID);
  function clientForSession(identity: Session) {
    return createHumanApiClient({ baseUrl: `${origin}/human`, headers: { Cookie: `__Host-layerx_access=${identity.session_id}` } });
  }
  const client = clientForSession(session);
  return {
    client, session, clientForSession, store, authenticator, sessions, origin,
    async reopen() { await close(serving); serving = server(); await listen(serving, address.port); },
    controller(api = client) { return new Approvals({ client: api, authenticator: async (options) => authenticator.assert(options) }); },
  };
}

async function approval_vacuous_cases_require_targets_and_real_stepup(t: TestContext) {
  const fixture = await peer(t);
  const controller = fixture.controller();
  const inbox = await controller.inbox();
  assert.equal(inbox.length, 3);
  const pending = await controller.detail(fixture.store.idFor("pending"));
  assert.ok(canApprove(pending, fixture.store.now)); assert.ok(heldDigest(pending));
  const html = renderPrivate(createElement(ApprovalDetailCard, { detail: pending, at: fixture.store.now, shell: "desktop" }));
  for (const key of ["approval.approve.action", "approval.reject.action", "approval.detail.agent", "approval.detail.counterparty", "approval.detail.amount", "approval.detail.fees", "approval.detail.budget", "error.technical.title"]) {
    const label = copyEntry(key).message;
    assert.ok(html.includes(label), `${key}: ${label}`);
  }
  assert.ok(html.includes('role="timer"'));
  fixture.store.releaseReceiptAvailable = false;
  const result = await controller.approve(pending, "decision-primary");
  assert.equal(result.kind, "decided");
  assert.equal(fixture.store.signatures.length, 1);
  const decision = fixture.store.decisions.get(pending.approval_id); assert.ok(decision);
  assert.equal(decision.approval_id, pending.approval_id); assert.equal(decision.state, "approved");
  assert.equal(decision.money_moved, false, "approval releases work; it cannot claim money has moved");
  const waiting = await controller.released(pending.approval_id); assert.ok(waiting);
  assert.equal(waiting.state, "processing"); assert.equal(waiting.evidence.length, 0);
  const waitingHtml = renderPrivate(createElement(ApprovalOutcomePanel, { outcome: result, released: waiting }));
  assert.doesNotMatch(waitingHtml, />Done<|>Complete</u);
  fixture.store.releaseReceiptAvailable = true;
  const released = await controller.released(pending.approval_id); assert.ok(released);
  assert.ok(released.evidence.some(move.verifiedOutcomeEvidence));
  assert.ok(released.stages.every((stage) => stage.evidence.some(move.verifiedOutcomeEvidence)));
  const outcome = renderPrivate(createElement(ApprovalOutcomePanel, { outcome: result, released }));
  assert.ok(outcome.includes("Released activity")); assert.ok(outcome.includes(`/app/activity/${released.entry_id}`));
  for (const state of ["expired", "defective"] as const) {
    const detail = await controller.detail(fixture.store.idFor(state));
    assert.equal(canApprove(detail, fixture.store.now), false);
    const card = renderPrivate(createElement(ApprovalDetailCard, { detail, at: fixture.store.now, shell: "mobile" }));
    assert.doesNotMatch(card, />Approve<|>Reject</u);
    assert.ok(card.includes(state === "expired" ? "expired" : "cannot"));
    const refused = await controller.reject(detail.approval_id, `refuse-${state}`);
    assert.equal(refused.kind, state);
    assert.equal((await controller.approve(detail, `approve-${state}`)).kind, state);
    assert.equal(fixture.store.decisions.has(detail.approval_id), false);
  }
  assert.equal(fixture.store.releases.size, 1);
  assert.doesNotMatch(source("approvals.spec.ts"), /if\s*\(await .*isVisible/u);
}

async function approval_two_device_decision_survives_reopen_and_refuses_replay(t: TestContext) {
  for (const action of ["approve", "reject"] as const) {
    const fixture = await peer(t);
    const firstClient = fixture.client;
    const publicClient = createHumanApiClient({ baseUrl: `${fixture.origin}/human` });
    const firstSession = fixture.session;
    const secondSession = await openPublicSession(publicClient, "mara@example.com", fixture.authenticator, PASSKEY_ID);
    const secondClient = fixture.clientForSession(secondSession);
    assert.notEqual(firstSession.session_id, secondSession.session_id);
    assert.notEqual(firstSession.device.device_id, secondSession.device.device_id);
    const first = fixture.controller(firstClient), second = fixture.controller(secondClient);
    const id = fixture.store.idFor("pending");
    const stale = await second.detail(id);
    const decision = action === "approve" ? await first.approve(await first.detail(id), "device-one") : await first.reject(id, "device-one");
    assert.equal(decision.kind, "decided");
    const original = fixture.store.decisions.get(id);
    const replay = await second.reject(stale.approval_id, "device-two");
    assert.equal(replay.kind, "converged");
    await fixture.reopen();
    const reopened = fixture.controller(fixture.clientForSession(secondSession));
    const recovered = await reopened.detail(id);
    assert.equal(recovered.state, action === "approve" ? "approved" : "rejected");
    assert.equal(canApprove(recovered, new Date()), false);
    assert.equal((await reopened.reject(id, "reload-replay")).kind, "converged");
    assert.deepEqual(fixture.store.decisions.get(id), original);
    assert.equal(fixture.store.decisions.size, 1); assert.equal(fixture.store.decisionKeys.get(id), "device-one");
    assert.equal(fixture.store.releases.size, action === "approve" ? 1 : 0);
    const released = await reopened.released(id);
    assert.equal(released !== undefined, action === "approve");
    const rendered = renderPrivate(createElement(ApprovalDetailCard, { detail: recovered, shell: "desktop", at: new Date(), released }));
    assert.doesNotMatch(rendered, />Approve<|>Reject</u);
  }
  assert.ok(source("approvals.spec.ts").includes("browser.newContext("));
  assert.ok(source("approvals.spec.ts").includes("signInPublicSession("));
  assert.doesNotMatch(source("approvals.spec.ts"), /context\.newPage\(/u);
}

function retired_copy_boundaries_reject_a_forbidden_message() {
  assert.equal(forbiddenCopy("Your proof is ready."), "proof");
  assert.throws(() => assert.doesNotMatch("Your proof is ready.", vocabularyPattern("proof")), assert.AssertionError);
  assert.equal(forbiddenCopy("Your waterproof coat is ready."), undefined);
  assert.match("Use a+b here.", vocabularyPattern("a+b"));
  assert.doesNotMatch("Use aaab here.", vocabularyPattern("a+b"));
  assert.equal(forbiddenCopy("Your DID is ready."), "DID");
  assert.equal(forbiddenCopy("Nothing you did is lost."), undefined);
  assert.equal(forbiddenCopy("Did you continue?"), undefined);
  assert.equal(forbiddenCopy("Your DIDactic guide is ready."), undefined);
  assert.equal(forbiddenCopy("Your PROOF is ready."), "proof");
  assert.throws(() => assert.doesNotMatch("Your DID is ready.", vocabularyPattern("DID")), assert.AssertionError);
}

async function receipt_links_resolve_native_data_and_keep_archive_explicit(t: TestContext) {
  const exchanges = JSON.parse(readFileSync(new URL(
    "../../../services/explorer/backend/apps/explorer/test/support/fixture/vcr_cassettes/transaction_importer_import_1_receipt.json",
    import.meta.url,
  ), "utf8")) as { request: { body: string }; response: { body: string } }[];
  const exchange = exchanges[0]; assert.ok(exchange);
  const recordedRequest = JSON.parse(exchange.request.body) as { method: string; params: string[] };
  const recorded = JSON.parse(exchange.response.body) as {
    jsonrpc: string; result: { transactionHash: string; blockNumber: string; status: string; logs: unknown[] };
  };
  const hash = recorded.result.transactionHash, receiptId = "a".repeat(64);
  assert.equal(recordedRequest.method, "eth_getTransactionReceipt");
  assert.deepEqual(recordedRequest.params, [hash]);
  const native = createServer(async (request, response) => {
    assert.equal(request.method, "POST"); assert.equal(request.url, "/");
    let body = ""; for await (const chunk of request) body += String(chunk);
    const call = JSON.parse(body) as { id: string; method: string; params: string[]; };
    assert.equal(call.method, "eth_getTransactionReceipt"); assert.deepEqual(call.params, [hash]);
    response.setHeader("content-type", "application/json");
    response.end(jsonWire({ ...recorded, id: call.id }));
  });
  await new Promise<void>((resolve) => native.listen(0, "127.0.0.1", resolve));
  const address = native.address(); assert.ok(address !== null && typeof address !== "string");
  const previous = process.env.LAYERX_GATEWAY_URL;
  process.env.LAYERX_GATEWAY_URL = `http://127.0.0.1:${address.port}/`;
  t.after(async () => {
    await new Promise<void>((resolve) => { native.close(() => resolve()); native.closeAllConnections(); });
    if (previous === undefined) delete process.env.LAYERX_GATEWAY_URL; else process.env.LAYERX_GATEWAY_URL = previous;
  });
  let reads = 0;
  const target = await links.resolveReceiptLink(receiptId.toUpperCase(), async (requested) => {
    reads++; assert.equal(requested, receiptId);
    const receipt = await gateway.ethTransactionReceipt(hash); assert.ok(receipt);
    assert.equal(receipt.blockNumber, BigInt(recorded.result.blockNumber)); assert.equal(receipt.status, "success");
    assert.equal(receipt.logs.length, recorded.result.logs.length);
    return { receiptId: requested, transactionHash: receipt.transactionHash };
  });
  assert.equal(reads, 1); assert.equal(links.explorerLinkPath(target), `/tx/${hash}`);
  const base = links.parseExplorerBaseUrl("https://explorer.paxeer.network/"); assert.ok(base);
  const href = new URL(links.explorerLinkPath(target), base);
  assert.equal(href.origin, base.origin);
  assert.equal(href.href, `${base.origin}/tx/${hash}`);
  assert.doesNotMatch(href.pathname, /paxeer-x\/receipts/u);
  assert.equal(links.historicalReceiptLinkPath({ receiptId, frozen: true }), `/paxeer-x/receipts/${receiptId}`);
  // @ts-expect-error Missing native transaction identity must fail closed at runtime too.
  assert.throws(() => links.explorerLinkPath({ kind: "receipt", receiptId }), TypeError);
  await assert.rejects(links.resolveReceiptLink(receiptId, async () => ({ receiptId: "c".repeat(64), transactionHash: hash })), TypeError);
  await assert.rejects(links.resolveReceiptLink(receiptId, async () => undefined), TypeError);
  for (const bad of ["../tx", `${receiptId}?redirect=x`, ""]) await assert.rejects(links.resolveReceiptLink(bad, async () => undefined), TypeError);
  assert.equal(links.parseExplorerBaseUrl("https://user:password@explorer.paxeer.network/"), undefined);
}

async function move_fixture_routes_decode_every_outcome_and_receipt_gate(t: TestContext) {
  const fixture = await peer(t);
  const expected = { done: "done", "no-receipt": "in-progress", sending: "in-progress", "still-checking": "still-checking", refused: "refused", "change-path": "refused" } as const;
  const quote = await move.requestMoveQuote(fixture.client, { source: "account", destination: "agent", amount: 1000n });
  assert.equal(quote.kind, "quoted"); if (quote.kind !== "quoted") assert.fail("quote is required");
  assert.equal(quote.quote.money.currency, "LXP");
  assert.ok(move.moveReview(quote.quote).fee.length > 0);
  for (const scenario of Object.keys(expected) as (keyof typeof expected)[]) {
    fixture.store.moveScenario = scenario;
    const committed = await move.commitMove(fixture.client, quote.quote.quote_id, "0123456789abcdef0123456789abcdef");
    assert.equal(committed.kind, "journey"); if (committed.kind !== "journey") assert.fail("producer journey required");
    const outcome = move.moveResult(committed.journey);
    assert.equal(outcome.kind, expected[scenario]);
    if (outcome.kind === "done") { assert.ok(outcome.receipts.length > 0); assert.ok(move.receiptBacked(committed.journey)); }
    if (scenario === "no-receipt") { assert.equal(move.receiptBacked(committed.journey), false); assert.equal(outcome.status, "processing"); }
    if (outcome.kind === "still-checking") assert.equal(outcome.locked, true);
    if (outcome.kind === "refused") {
      assert.ok(outcome.moneyLeftMessage.length > 0);
      if (scenario === "change-path") assert.equal(outcome.changePath, "/app/agents/agt_01j8agd2s3a1b2c3d4e5f6g7h8");
    }
  }
  fixture.store.moveScenario = "quote-expired";
  const expired = await move.commitMove(fixture.client, quote.quote.quote_id, "0123456789abcdef0123456789abcdef");
  assert.equal(expired.kind, "failed"); if (expired.kind !== "failed") assert.fail("typed expiry is required");
  assert.equal(expired.failure.kind, "quote-expired");
  assert.equal(fixture.store.requests.filter((item) => item.path === "/human/v1/moves").length, 7);
  assert.ok(fixture.store.requests.every((item) => item.path.startsWith("/human/v1/")));
  assert.doesNotMatch(source("move.spec.ts"), /\/v1\/move\/commit|created_at|jny_test/u);
}

async function onboarding_real_api_states_resume_and_recover_after_reopen(t: TestContext) {
  const fixture = await peer(t);
  const retained = new Map<string, string>();
  const storage = () => ({
    getItem: (key: string) => retained.get(key) ?? null,
    setItem: (key: string, value: string) => { retained.set(key, value); }, removeItem: (key: string) => { retained.delete(key); }
  });
  recovery.rememberPending({ version: 1, idempotencyKey: "onboarding-repair", email: "mara@example.com", displayName: "Mara", accountId: "account-repair", passkeyAdded: true }, storage());
  for (const scenario of ["progress", "failure", "queued", "resume"] as const) {
    fixture.store.onboardingScenario = scenario;
    const journey = await fixture.client.onboardingStatus();
    assert.equal(journey.stages.length, 4); assert.equal(onboarding.accountActive(journey), false);
    assert.equal(onboarding.completedStageIds(journey).size, 2);
    const active = onboarding.accountActive(journey);
    assert.equal(active, false);
    const message = copyEntry(active ? "onboarding.active" : "onboarding.not_active").message;
    assert.equal(message, copyEntry("onboarding.not_active").message);
    assert.equal(forbiddenCopy(message), undefined);
    for (const stage of journey.stages.map(onboarding.stagePresentation)) {
      assert.ok(stage.title.length > 0);
      assert.equal(forbiddenCopy(stage.title), undefined);
    }
    if (scenario === "failure") assert.ok(journey.stages.some((stage) => stage.state === "refused"));
    if (scenario === "queued") assert.ok(journey.stages.map(onboarding.stagePresentation).some((stage) => stage.queued));
  }
  const pending = recovery.pendingOnboarding(storage()); assert.ok(pending);
  const before = await fixture.client.onboardingStatus();
  await fixture.reopen();
  const reopened = fixture.clientForSession(fixture.session);
  assert.deepEqual(await reopened.onboardingStatus(), before);
  assert.deepEqual(recovery.pendingOnboarding(storage()), pending);
  const resumed = await reopened.onboardingResume();
  assert.equal(resumed.journey_id, before.journey_id);
  assert.equal(onboarding.completedStageIds(resumed).size, 3);
  assert.equal(onboarding.accountActive(resumed), true);
  const active = onboarding.accountActive(resumed);
  assert.equal(active, true);
  assert.equal(copyEntry(active ? "onboarding.active" : "onboarding.not_active").message, copyEntry("onboarding.active").message);
  recovery.rememberActivatedAccount(resumed, storage());
  assert.equal(recovery.pendingOnboarding(storage()), undefined);
  await fixture.reopen();
  assert.equal(onboarding.accountActive(await reopened.onboardingStatus()), true);
  assert.equal(storage().getItem("layerx.account"), "account-repair");
  assert.doesNotMatch(source("onboarding.spec.ts"), /mock_onboarding|mock_signin|mock_account_created|mock_ceremony/u);
}

function onboarding_copy_resolves_catalogue_and_interpolates_visible_messages() {
  const messages = onboardingMessages(); assert.ok(messages.length >= 32);
  for (const message of messages) { assert.ok(message.trim().length > 0); assert.doesNotMatch(message, /[{}]/u); assert.equal(forbiddenCopy(message), undefined, message); }
  assert.throws(() => assert.equal(forbiddenCopy("Your proof is ready."), undefined), assert.AssertionError);
  assert.ok(source("onboarding.spec.ts").includes("requireVisibleCopy(page)"));
  assert.doesNotMatch(source("onboarding.spec.ts"), /copyKey\.toLowerCase/u);
}

function repaired_suites_have_no_vacuous_targets_or_retired_transport() {
  for (const file of ["approvals.spec.ts", "move.spec.ts", "onboarding.spec.ts"]) {
    const text = source(file); assert.doesNotMatch(text, /if\s*\(await .*isVisible\(\)/u);
  }
  assert.doesNotMatch(source("gateway-history.test.ts"), /:\$\{String\(address.port\)\}\/rpc/u);
  assert.ok(source("gateway-history.test.ts").includes('request.url !== "/"'));
  // A bare journey (the old third schema) cannot be decoded as an HTTP envelope.
  assert.throws(() => decodeJourney({ state: "done", created_at: "2026-01-01T00:00:00Z" }, "obsolete fixture"));
}

test(approval_vacuous_cases_require_targets_and_real_stepup);
test(approval_two_device_decision_survives_reopen_and_refuses_replay);
test(retired_copy_boundaries_reject_a_forbidden_message);
test(receipt_links_resolve_native_data_and_keep_archive_explicit);
test(move_fixture_routes_decode_every_outcome_and_receipt_gate);
test(onboarding_real_api_states_resume_and_recover_after_reopen);
test(onboarding_copy_resolves_catalogue_and_interpolates_visible_messages);
test(repaired_suites_have_no_vacuous_targets_or_retired_transport);
