// Task 22.33.357 closure record:
// support-idle-reply-refresh -> support_idle_reply_refresh_and_cleanup
// support-pending-message-loss -> support_offline_pending_survives_reopen;
//   support_response_loss_recovers_original_create_and_reply
// support-retry-operation-target -> support_uncertain_create_precedes_another_send;
//   support_response_loss_recovers_original_create_and_reply
// support-fixture-contract -> support_producer_contract_refusals_and_private_scope
// Baseline: not run; the explicit gate order forbids verification before complete
// implementation and manual verify_cmd runs. No baseline failures are claimed.
// Wiring request: services/human/crates/layerx-human-service/src/support.rs,
// domain 10: retain native service conformance for the committed in-memory peer;
// this Node-only gate cannot execute the Rust service or an external toolchain.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test, { type TestContext } from "node:test";
import { setTimeout as delay } from "node:timers/promises";
import { Window } from "happy-dom";

import { createHumanApiClient } from "../src/api/generated/index.ts";
import { SupportChatSession } from "../src/support/conversation.ts";
import { openSupportFixture } from "../src/support/service.fixture.ts";

const traceId = "trc_0123456789abcdef0123456789abcdef";

async function setup(t: TestContext) {
  const peer = await openSupportFixture();
  const window = new Window({ url: "https://support.example.test" });
  const storage = window.sessionStorage;
  const client = createHumanApiClient({ baseUrl: peer.baseUrl });
  const sessions: SupportChatSession[] = [];
  function open(shell: "mobile" | "desktop" = "desktop", trace = traceId) {
    const session = new SupportChatSession({ shell, traceId: trace, storage, client });
    sessions.push(session);
    return session;
  }
  t.after(async () => {
    for (const session of sessions) session.stop();
    await peer.close();
    await window.happyDOM.close();
  });
  return { ...peer, storage, client, open };
}

async function until(predicate: () => boolean, label: string): Promise<void> {
  const deadline = Date.now() + 7_000;
  while (!predicate() && Date.now() < deadline) await delay(20);
  assert.ok(predicate(), label);
}

async function support_idle_reply_refresh_and_cleanup(t: TestContext) {
  const { service, open } = await setup(t);
  const session = open();
  await session.initialize();
  session.setDraft("Please check my account");
  await session.send();
  const id = session.snapshot().conversationId!;
  service.refuseMutations = true;
  session.setDraft("Pending while waiting for the operator");
  await session.send();
  const pending = session.snapshot().entries.find((entry) => entry.delivery === "failed")!;
  assert.ok(pending);
  session.start();
  await until(() => service.requests.filter((entry) => entry.method === "GET" && entry.path.endsWith("/conversations")).length >= 2,
    "the lifecycle performs its initial authoritative refresh");
  service.recordSupportReply(id, "operator-idle-reply", "Your account is ready");
  await until(() => session.snapshot().entries.some((entry) => entry.author === "support" && entry.body === "Your account is ready"),
    "an idle customer receives the operator reply without sending or reopening");
  assert.equal(session.snapshot().conversationState, "waiting-for-you");
  assert.ok(session.snapshot().entries.some((entry) => entry.message_id === pending.message_id && entry.body === pending.body));
  service.refuseMutations = false;
  const reply = session.snapshot().entries.find((entry) => entry.author === "support")!;
  await session.sendFeedback(reply.message_id, true);
  assert.equal(session.snapshot().feedback.get(reply.message_id), true);
  assert.ok(service.requests.some((entry) => entry.path.endsWith("/status")));
  assert.ok(service.requests.some((entry) => entry.path.endsWith("/read")));
  service.resolve(id);
  await until(() => session.snapshot().conversationState === "resolved", "polling projects authoritative resolution");
  session.stop();
  await delay(50);
  const requests = service.requests.length;
  await delay(5_100);
  assert.equal(service.requests.length, requests, "stopping the component cancels future refreshes");
  const source = readFileSync(new URL("../src/support/chat.tsx", import.meta.url), "utf8");
  assert.match(source, /session\.start\(\)/u);
  assert.match(source, /session\.stop\(\)/u);
  assert.match(source, /useActiveAccountId\(\)/u);
  assert.match(source, /SupportChatForAccount key=/u);
}

async function support_offline_pending_survives_reopen(t: TestContext) {
  const { service, storage, open } = await setup(t);
  const first = open();
  await first.initialize();
  first.seedTopic("report");
  first.setDraft("Private offline message");
  first.setOnline(false);
  service.unavailable = true;
  await first.send();
  assert.equal(first.snapshot().draft, "");
  assert.equal(first.snapshot().entries[0]?.delivery, "failed");
  const key = `layerx.support.pending.v1.${service.principal}`;
  const journal = storage.getItem(key);
  assert.ok(journal?.includes("Private offline message"));
  const attempt = JSON.parse(journal!)[0];
  assert.equal(attempt.operation.kind, "create");
  assert.equal(attempt.operation.request.trace_id, traceId);
  assert.equal(attempt.operation.request.topic, "report");
  first.stop();

  service.unavailable = false;
  service.refuseMutations = true;
  const reopened = open();
  await reopened.initialize();
  assert.equal(reopened.snapshot().entries[0]?.body, "Private offline message");
  assert.equal(reopened.snapshot().entries[0]?.delivery, "failed");
  assert.equal(storage.getItem(key), journal, "a refusal retains the original recoverable identity");
  reopened.stop();
  service.refuseMutations = false;
  const recovered = open();
  await recovered.initialize();
  assert.equal(recovered.snapshot().entries[0]?.delivery, "sent");
  assert.equal(recovered.snapshot().entries[0]?.body, "Private offline message");
  assert.equal(storage.getItem(key), null, "only authoritative acknowledgement deletes the attempt");
  assert.equal(service.conversations().length, 1);
  assert.equal(service.conversations()[0]?.messages.length, 1);
  const creates = service.requests.filter((entry) => entry.method === "POST" && entry.path.endsWith("/conversations"));
  assert.equal(creates.length, 2);
  assert.ok(creates.every((entry) => entry.key === attempt.idempotencyKey));
}

async function support_response_loss_recovers_original_create_and_reply(t: TestContext) {
  const { service, storage, open } = await setup(t);
  const first = open();
  await first.initialize();
  first.seedTopic("report");
  first.setDraft("Create accepted but response lost");
  service.loseNextMutationResponse = true;
  await first.send();
  assert.equal(first.snapshot().entries[0]?.delivery, "failed");
  assert.equal(service.conversations().length, 1);
  const key = `layerx.support.pending.v1.${service.principal}`;
  const createAttempt = JSON.parse(storage.getItem(key)!)[0];
  first.stop();

  // A new shell/trace must not mutate the original create payload or key.
  const reopened = open("mobile", "trc_ffffffffffffffffffffffffffffffff");
  await reopened.initialize();
  assert.equal(reopened.snapshot().entries.length, 1);
  assert.equal(reopened.snapshot().entries[0]?.delivery, "sent");
  assert.equal(service.conversations().length, 1);
  const creates = service.requests.filter((entry) => entry.method === "POST" && entry.path.endsWith("/conversations"));
  assert.equal(creates.length, 2);
  assert.equal(creates[0]?.key, creates[1]?.key);
  assert.deepEqual(creates[0]?.body, creates[1]?.body);
  assert.equal(creates[1]?.key, createAttempt.idempotencyKey);
  assert.equal(service.conversations()[0]?.shell, "desktop");
  assert.equal(service.conversations()[0]?.trace_id, traceId);

  reopened.setDraft("Reply accepted but response lost");
  service.loseNextMutationResponse = true;
  await reopened.send();
  const replyAttempt = JSON.parse(storage.getItem(key)!)[0];
  assert.equal(replyAttempt.operation.kind, "reply");
  assert.equal(replyAttempt.operation.conversationId, service.conversations()[0]?.conversation_id);
  reopened.stop();
  const recovered = open();
  await recovered.initialize();
  assert.deepEqual(recovered.snapshot().entries.map((entry) => entry.body), [
    "Create accepted but response lost", "Reply accepted but response lost",
  ]);
  assert.ok(recovered.snapshot().entries.every((entry) => entry.delivery === "sent"));
  assert.equal(storage.getItem(key), null);
  assert.equal(service.conversations()[0]?.messages.length, 2);
  const replies = service.requests.filter((entry) => entry.path.endsWith("/replies"));
  assert.equal(replies.length, 2);
  assert.equal(replies[0]?.key, replies[1]?.key);
  assert.equal(replies[0]?.path, replies[1]?.path);
  assert.deepEqual(replies[0]?.body, replies[1]?.body);
}

async function support_uncertain_create_precedes_another_send(t: TestContext) {
  const { service, open } = await setup(t);
  const session = open();
  await session.initialize();
  service.loseNextMutationResponse = true;
  session.setDraft("First uncertain create");
  await session.send();
  const failedId = session.snapshot().entries[0]!.message_id;
  service.refuseMutations = true;
  session.setDraft("Second customer message");
  await session.send();
  assert.equal(session.snapshot().draft, "Second customer message", "a refused recovery cannot consume the next draft");
  assert.equal(service.conversations().length, 1);
  assert.equal(service.conversations()[0]?.messages.length, 1);
  service.refuseMutations = false;
  await session.send();
  await session.retry(failedId);
  const writes = service.requests.filter((entry) => entry.method === "POST");
  assert.equal(writes.length, 4);
  assert.ok(writes.slice(0, 3).every((entry) => entry.path === "/human/v1/support/conversations"));
  assert.ok(writes.slice(0, 3).every((entry) => entry.key === writes[0]?.key));
  assert.equal(writes[3]?.path, `/human/v1/support/conversations/${service.conversations()[0]?.conversation_id}/replies`);
  assert.notEqual(writes[3]?.key, writes[0]?.key);
  assert.equal(service.conversations().length, 1);
  assert.deepEqual(service.conversations()[0]?.messages.map((entry) => entry.body), ["First uncertain create", "Second customer message"]);
  session.stop();
  const reopened = open();
  await reopened.initialize();
  assert.equal(reopened.snapshot().entries.length, 2);
}

async function support_producer_contract_refusals_and_private_scope(t: TestContext) {
  const { service, client, storage, open } = await setup(t);
  const session = open();
  await session.initialize();
  session.setDraft("Contract message");
  await session.send();
  const id = session.snapshot().conversationId!;
  const reply = service.recordSupportReply(id, "operator-contract", "A genuine operator response");
  await session.initialize();
  assert.equal(session.snapshot().conversationState, "waiting-for-you");
  assert.equal(session.snapshot().entries.find((entry) => entry.author === "support")?.read, true);
  assert.equal(session.snapshot().awaitingFeedbackId, reply.messages.at(-1)?.message_id);
  assert.equal(service.conversations()[0]?.messages.at(-1)?.read, true);
  for (const fault of ["open-state", "missing-shell"] as const) {
    service.listFault = fault;
    await assert.rejects(client.supportList(), fault === "open-state" ? /must be a declared SupportConversationState variant/u : /shell/u);
    const refused = open();
    await refused.initialize();
    assert.equal(refused.snapshot().load, "failed");
    assert.equal(refused.snapshot().entries.length, 0);
  }
  service.listFault = undefined;
  service.unavailable = true;
  const refused = open();
  await refused.initialize();
  assert.equal(refused.snapshot().load, "failed");
  service.unavailable = false;

  service.refuseMutations = true;
  session.setDraft("Principal A private pending body");
  await session.send();
  const journal = storage.getItem(`layerx.support.pending.v1.${service.principal}`);
  assert.ok(journal);
  service.principal = "act_support_customer_b";
  const otherAccount = open();
  await otherAccount.initialize();
  assert.equal(otherAccount.snapshot().entries.length, 0);
  assert.equal(service.conversations().length, 0);
  service.refuseMutations = false;
  await assert.rejects(client.supportReply(id, { body: "Cannot enter another account" }, "cross-principal"), /support-conversation-unknown/u);
  await session.initialize();
  assert.equal(session.snapshot().entries.length, 0, "account changes clear the previous account's private projection");
  assert.equal(storage.getItem("layerx.support.pending.v1.act_support_customer_a"), journal);
  service.principal = "act_support_customer_a";
  const returned = open();
  await returned.initialize();
  assert.ok(returned.snapshot().entries.some((entry) => entry.body === "Principal A private pending body" && entry.delivery === "sent"));
  assert.equal(service.conversations()[0]?.messages.filter((entry) => entry.body === "Principal A private pending body").length, 1);
}

async function support_stop_discards_an_inflight_refresh(t: TestContext) {
  const { service, open } = await setup(t);
  const session = open();
  let release: () => void = () => { throw new Error("refresh hold not installed"); };
  service.holdList = new Promise<void>((resolve) => { release = resolve; });
  const initial = session.snapshot();
  const refreshing = session.initialize();
  await until(() => service.requests.some((entry) => entry.path.endsWith("/conversations")), "refresh reached real HTTP producer");
  session.stop();
  release();
  await refreshing;
  assert.equal(session.snapshot(), initial, "late HTTP completion cannot publish after unmount");
}

test(support_idle_reply_refresh_and_cleanup);
test(support_offline_pending_survives_reopen);
test(support_response_loss_recovers_original_create_and_reply);
test(support_uncertain_create_precedes_another_send);
test(support_producer_contract_refusals_and_private_scope);
test(support_stop_discards_an_inflight_refresh);
