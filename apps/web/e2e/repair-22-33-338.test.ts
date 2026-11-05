import assert from "node:assert/strict";
import test, { type TestContext } from "node:test";
import "./repair-22-33-338-runtime.ts";
import { committedReceipt, withNativeReceipt } from "./repair-22-33-338-receipts.ts";
import { committedEvidence, committedVector, receiptKeys, unhex } from "./repair-22-33-338-commit.ts";
import { SUPPORT_ACCOUNT, SUPPORT_CONVERSATIONS, supportRecording, withSupportConversations } from "./repair-22-33-338-support.ts";
import type { JsonValue } from "../src/api/generated/index.ts";

const client = await import("../src/explorer/client.ts");
const { explorerLinkPath } = await import("../src/explorer/links.ts");
const { ExplorerExternalLink, ExplorerVerificationBadge } = await import("../src/kit/explorer.tsx");
const { verificationWord, verifiedReceiptWord } = await import("../src/kit/model.ts");
const { homeBalance } = await import("../src/journeys/home/model.ts");
const { createElement } = await import("react");
const { renderToStaticMarkup } = await import("react-dom/server");
const { SupportChatSession } = await import("../src/support/conversation.ts");
const { HumanApiError, decodeSupportConversation } = await import("../src/api/generated/index.ts");
const { verifySequencerReceipt, verifyCommittedReceipt } = await import("../../../services/agent/sdk/typescript/src/verifier.ts");

const RECEIPT_ID = "ab".repeat(32);
const RETIRED_SETTINGS = ["LAYERX_EXPLORER_API_ORIGIN", "LAYERX_EXPLORER_PROGRAM_API_ORIGIN", "LAYERX_EXPLORER_PROGRAM_BEARER_TOKEN"] as const;

function preserveEnvironment(context: TestContext): void {
  const names = ["PAXEER_X_EXPLORER_BASE_URL", ...RETIRED_SETTINGS];
  const previous = names.map(name => [name, process.env[name]] as const);
  context.after(() => {
    for (const [name, value] of previous) {
      if (value === undefined) delete process.env[name];
      else process.env[name] = value;
    }
  });
  for (const name of names) delete process.env[name];
}

async function receipt_env_proof_retired_target(context: TestContext): Promise<void> {
  preserveEnvironment(context);
  await withNativeReceipt(async (read, origin, requests) => {
    for (const name of RETIRED_SETTINGS) {
      for (const base of [origin, "https://public-explorer.example"]) {
        const receipt = await read();
        const target = { kind: "receipt", receiptId: RECEIPT_ID.toUpperCase(), transactionHash: receipt.txHash } as const;
        process.env.PAXEER_X_EXPLORER_BASE_URL = base;
        for (const retired of ["https://retired.example", "obsolete-credential", "\r\nmalformed"]) {
          process.env[name] = retired;
          assert.equal(client.explorerLink(target), `${base}/tx/${receipt.txHash}`);
          assert.notEqual(receipt.txHash.slice(2), RECEIPT_ID);
        }
        for (const invalid of [undefined, "", "not a URL", "http://public.example", `${base}/?`, `${base}/#`, `${base}/archive`, "https://user:password@public.example"]) {
          if (invalid === undefined) delete process.env.PAXEER_X_EXPLORER_BASE_URL;
          else process.env.PAXEER_X_EXPLORER_BASE_URL = invalid;
          assert.throws(() => client.explorerLink(target), client.ExplorerUnavailableError);
        }
      }
    }
    assert.equal(requests.filter(method => method === "pax_sequencerReceipt").length, 6);
    assert.equal(requests.filter(method => method === "eth_getRawTransactionByHash").length, 6);
  });
}

async function receipt_consumer_proof_retired_target(context: TestContext): Promise<void> {
  preserveEnvironment(context);
  await withNativeReceipt(async (read, origin) => {
    process.env.PAXEER_X_EXPLORER_BASE_URL = origin;
    for (let pass = 0; pass < 2; pass += 1) {
      const receipt = await read();
      const target = { kind: "receipt", receiptId: RECEIPT_ID, transactionHash: receipt.txHash.toUpperCase().replace("0X", "0x") } as const;
      const href = client.explorerLink(target);
      assert.equal(explorerLinkPath(target), `/tx/${committedReceipt.txHash}`);
      assert.equal(href, `${origin}/tx/${committedReceipt.txHash}`);
      const rendered = renderToStaticMarkup(createElement(ExplorerExternalLink, { href, children: receipt.txHash }));
      assert.ok(rendered.includes(`href="${origin}/tx/${committedReceipt.txHash}"`));
      assert.ok(rendered.includes('data-explorer-link="external"'));
      assert.ok(rendered.includes('rel="noreferrer"'));
      assert.equal(receipt.blockHash, committedReceipt.blockHash);
      assert.deepEqual(receipt.logs, committedReceipt.logs);
      assert.equal(receipt.receipt, committedReceipt.receipt);
    }
    const historical = { kind: "historical-receipt", receiptId: RECEIPT_ID, frozen: true } as const;
    assert.equal(client.explorerLink(historical), `${origin}/paxeer-x/receipts/${RECEIPT_ID}`);
    for (const id of ["", "../anchors", `${RECEIPT_ID}?next=archive`, `${RECEIPT_ID}#archive`]) {
      assert.throws(() => explorerLinkPath({ kind: "receipt", receiptId: id, transactionHash: committedReceipt.txHash }), TypeError);
      assert.throws(() => explorerLinkPath({ ...historical, receiptId: id }), TypeError);
    }
    for (const hash of ["", RECEIPT_ID, "../archive", `${committedReceipt.txHash}?archive`]) {
      assert.throws(() => explorerLinkPath({ kind: "receipt", receiptId: RECEIPT_ID, transactionHash: hash }), TypeError);
    }
    // @ts-expect-error Unresolved current receipts are refused at the runtime boundary too.
    assert.throws(() => explorerLinkPath({ kind: "receipt", receiptId: RECEIPT_ID }), TypeError);
    // @ts-expect-error A current receipt cannot acquire a historical identity without frozen evidence.
    assert.throws(() => explorerLinkPath({ ...historical, frozen: false }), TypeError);
  });
}

async function kit_separate_layerx_finality(): Promise<void> {
  const signedReceipt = unhex(committedVector.signedReceipt);
  const rawTransaction = unhex(committedVector.rawTransaction);
  const keys = receiptKeys();
  const verified = await verifySequencerReceipt(unhex(committedVector.signedReceipt), unhex(committedVector.rawTransaction), receiptKeys());
  assert.equal(verified.level, "sequencer-signed");
  const beforeCommit = renderToStaticMarkup(createElement(ExplorerVerificationBadge, { label: await verifiedReceiptWord(signedReceipt, rawTransaction, keys) }));
  assert.ok(beforeCommit.includes("Verified against its receipt"));
  assert.ok(!beforeCommit.includes("Finalised in a committed Paxeer block"));
  const evidence = committedEvidence();
  const absentVotes = evidence.commit.signatures.filter(signature => signature.blockIdFlag === 1);
  assert.equal(absentVotes.length, 1);
  assert.deepEqual(absentVotes[0]!.signature, new Uint8Array());
  assert.deepEqual(absentVotes[0]!.validatorAddress, new Uint8Array());
  await assert.rejects(verifyCommittedReceipt(verified, { ...evidence, commit: { ...evidence.commit, signatures: [] } }));
  await assert.rejects(verifyCommittedReceipt(verified, { ...evidence, commit: { ...evidence.commit, signatures: absentVotes } }));
  await assert.rejects(verifyCommittedReceipt(verified, { ...evidence, inclusion: { ...evidence.inclusion!, transaction: new Uint8Array([0]) } }));
  await assert.rejects(verifiedReceiptWord(signedReceipt, rawTransaction, keys, { ...evidence, commit: { ...evidence.commit, signatures: [] } }));
  await assert.rejects(verifiedReceiptWord(signedReceipt, rawTransaction, keys, { ...evidence, inclusion: { ...evidence.inclusion!, transaction: new Uint8Array([0]) } }));
  assert.equal(homeBalance().kind, "unavailable");
  const committed = await verifyCommittedReceipt(verified, evidence);
  assert.equal(committed.level, "committed");
  assert.equal(committed.landingHeight, 11n);
  assert.equal(committed.signedPower, 7_000_000_000n);
  assert.equal(committed.totalPower, 7_000_000_001n);
  const finality = await verifiedReceiptWord(signedReceipt, rawTransaction, keys, evidence);
  assert.equal(finality, verificationWord("checkpoint-finalised"));
  assert.equal(finality, "Finalised in a committed Paxeer block");
  const afterCommit = renderToStaticMarkup(createElement(ExplorerVerificationBadge, { label: finality }));
  assert.ok(afterCommit.includes("Finalised in a committed Paxeer block"));
  assert.ok(!afterCommit.includes("LayerX"));
}

function result(value: JsonValue): JsonValue | undefined {
  assert.ok(value !== null && typeof value === "object" && !Array.isArray(value));
  assert.equal(value.ok, true);
  return value.result;
}

async function support_fixture_route_shape(context: TestContext): Promise<void> {
  // Node exposes Navigator without the browser connectivity property. Supply the
  // browser platform fact while transport interruptions still use real sockets.
  const online = Object.getOwnPropertyDescriptor(navigator, "onLine");
  Object.defineProperty(navigator, "onLine", { configurable: true, value: true });
  context.after(() => {
    if (online === undefined) Reflect.deleteProperty(navigator, "onLine");
    else Object.defineProperty(navigator, "onLine", online);
  });
  await withSupportConversations(async ({ client: supportClient, requests, errors, storage, failNext }) => {
    const created = decodeSupportConversation(result(supportRecording("create").body), "recorded create");
    const replied = decodeSupportConversation(result(supportRecording("reply").body), "recorded reply");
    const withFeedback = decodeSupportConversation(result(supportRecording("feedback").body), "recorded feedback");
    assert.ok(created.trace_id);
    const session = new SupportChatSession({ shell: "desktop", traceId: created.trace_id, client: supportClient, storage });
    await session.initialize();
    assert.deepEqual(errors, []);
    assert.equal(session.snapshot().load, "ready");
    assert.deepEqual(session.snapshot().entries, []);
    assert.deepEqual(requests.map(request => [request.method, request.path]), [["GET", SUPPORT_ACCOUNT], ["GET", SUPPORT_CONVERSATIONS]]);
    session.seedTopic("report");
    session.setDraft(created.messages[0]!.body);
    failNext("typed");
    await session.send();
    const failed = session.snapshot().entries[0]!;
    assert.equal(failed.delivery, "failed");
    assert.equal(failed.body, created.messages[0]!.body);
    assert.equal(session.snapshot().connection, "online");
    const firstRequest = requests.at(-1)!;
    assert.deepEqual([firstRequest.method, firstRequest.path], ["POST", SUPPORT_CONVERSATIONS]);
    assert.ok(firstRequest.idempotency);
    session.setDraft("Retain this unsent draft during retry");
    await session.retry(failed.message_id);
    assert.equal(session.snapshot().entries[0]!.delivery, "sent");
    assert.equal(session.snapshot().draft, "Retain this unsent draft during retry");
    assert.equal(requests.at(-1)!.idempotency, firstRequest.idempotency);
    assert.deepEqual(requests.at(-1)!.body, firstRequest.body);
    const replyBody = replied.messages.at(-1)!.body;
    session.setDraft(replyBody);
    failNext("transport");
    await session.send();
    const pending = session.snapshot().entries.at(-1)!;
    assert.equal(pending.delivery, "failed");
    assert.equal(pending.body, replyBody);
    assert.equal(session.snapshot().connection, "offline");
    const interrupted = requests.at(-1)!;
    assert.equal(interrupted.path, `${SUPPORT_CONVERSATIONS}/${created.conversation_id}/replies`);
    session.setDraft("Keep the next draft");
    await session.retry(pending.message_id);
    assert.equal(session.snapshot().draft, "Keep the next draft");
    assert.equal(session.snapshot().connection, "online");
    assert.equal(session.snapshot().entries.filter(entry => entry.body === replyBody).length, 1);
    assert.equal(requests.at(-1)!.idempotency, interrupted.idempotency);
    assert.deepEqual(requests.at(-1)!.body, interrupted.body);
    const replyId = withFeedback.messages.find(message => message.author === "support")!.message_id;
    failNext("typed");
    await session.sendFeedback(replyId, true);
    assert.equal(session.snapshot().feedbackDelivery.get(replyId), "failed");
    await session.sendFeedback(replyId, true);
    assert.equal(session.snapshot().feedbackDelivery.get(replyId), "saved");
    assert.equal(session.snapshot().feedback.get(replyId), true);
    assert.deepEqual(requests.at(-1)!.body, { message_id: replyId, helpful: true });
    assert.equal(requests.at(-1)!.path, `${SUPPORT_CONVERSATIONS}/${created.conversation_id}/feedback`);
    assert.equal((await supportClient.supportStatus(created.conversation_id)).conversation_id, created.conversation_id);
    assert.equal((await supportClient.supportRead(created.conversation_id, { through_message_id: replyId })).unread_count, 0);
    failNext("typed");
    await assert.rejects(supportClient.supportList(), error => error instanceof HumanApiError && error.status === 503 && error.detail.code === "support-unavailable" && error.detail.retry === "retriable");
    session.stop();
    const reopened = new SupportChatSession({ shell: "desktop", client: supportClient, storage });
    await reopened.initialize();
    assert.deepEqual(errors, []);
    assert.equal(reopened.snapshot().conversationId, created.conversation_id);
    assert.equal(reopened.snapshot().feedback.get(replyId), true);
    assert.ok(requests.every(request => request.path === SUPPORT_ACCOUNT || request.path.startsWith(SUPPORT_CONVERSATIONS)));
    reopened.stop();
  });
}

const { wallet, HASH, expectedReceipt, withSubmission } = await import("./repair-22-33-338-sponsored.ts");

async function sponsored_retired_transport_reopens_recovery(): Promise<void> {
  await withSubmission(async session => {
    session.receiptStatus = "0x1";
    const review = wallet.sidioraQuotePresentation(session.quoted);
    assert.ok(review);
    const confirmed = { outcome: "sent", transactionHash: HASH, receipt: expectedReceipt("0x1") };
    assert.deepEqual(await wallet.sendWalletSponsoredBatch(session.quoted, review.identity), confirmed);
    const key = `layerx:private-sponsored:v1:${session.quoted.batch.account}`;
    const saved = session.storage.getItem(key);
    assert.ok(saved);
    const firstStore = session.storage;
    await session.restart();
    assert.notStrictEqual(session.storage, firstStore);
    assert.equal(session.storage.getItem(key), saved);
    const submits = session.batches.length;
    const signatures = session.hostedPaths.filter(path => path === "/v1/wallet/sign-digest").length;
    for (let replay = 0; replay < 2; replay += 1) {
      assert.deepEqual(await wallet.resumeWalletSponsoredSubmission(session.quoted.batch.account, session.quoted.batch.calls[0]), confirmed);
      assert.equal(session.storage.getItem(key), saved);
      assert.equal(session.batches.length, submits);
    }
    session.status = "confirmed";
    session.receiptStatus = "0x1";
    assert.deepEqual(await wallet.sendWalletSponsoredBatch(session.quoted, review.identity), {
      outcome: "sent", transactionHash: HASH, receipt: expectedReceipt("0x1"),
    });
    assert.equal(session.storage.getItem(key), null);
    assert.equal(session.storage.getItem(`paxeer:sponsored:pending:v1:125:${session.quoted.batch.account}`), "null");
    assert.equal(session.batches.length, submits);
    assert.equal(session.hostedPaths.filter(path => path === "/v1/wallet/sign-digest").length, signatures);
    assert.equal(session.hostedPaths.filter(path => path === "/v1/wallet/me").length, 2);
    assert.ok(session.nodeMethods.includes("pax_sequencerReceipt"));
    assert.ok(session.nodeMethods.includes("px_getReceiptKeys"));
    assert.ok(session.nodeMethods.includes("eth_getRawTransactionByHash"));
    assert.equal(session.nodeMethods.filter(method => method === "eth_getTransactionReceipt").length, 4);
    assert.deepEqual(session.receiptHashes, [[HASH], [HASH], [HASH], [HASH]]);
    await session.restart();
    assert.equal(await wallet.resumeWalletSponsoredSubmission(session.quoted.batch.account), undefined);
    assert.equal(session.batches.length, submits);
  });
}

test(sponsored_retired_transport_reopens_recovery);

test(receipt_env_proof_retired_target);
test(receipt_consumer_proof_retired_target);
test(kit_separate_layerx_finality);
test(support_fixture_route_shape);
