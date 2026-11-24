import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import type { HumanApiClient, JsonValue } from "../src/api/generated/index.ts";
import type { SupportPendingStorage } from "../src/support/conversation.ts";

export const SUPPORT_CONVERSATIONS = "/human/v1/support/conversations";
export const SUPPORT_ACCOUNT = "/human/v1/account/balance";

export interface SupportRecording {
  readonly status: number;
  readonly body: JsonValue;
}
export function supportRecording(operation: string, kind = "response"): SupportRecording {
  return JSON.parse(readFileSync(new URL(`../../../services/human/schema/human-api/golden/support.${operation}.${kind}.json`, import.meta.url), "utf8")) as SupportRecording;
}

export async function withSupportConversations(run: (value: {
  client: HumanApiClient;
  requests: { path: string; method: string; body: unknown; idempotency: string | undefined }[];
  errors: readonly unknown[];
  storage: SupportPendingStorage;
  failNext(kind: "typed" | "transport"): void;
}) => Promise<void>): Promise<void> {
  const { humanApi } = await import("../src/api/index.ts");
  const { decodeSupportConversation, encodeSupportConversationPage } = await import("../src/api/generated/index.ts");
  const created = supportRecording("create");
  const reply = supportRecording("reply");
  const feedback = supportRecording("feedback");
  const failure = supportRecording("list", "failure");
  const empty = supportRecording("list");
  const account = JSON.parse(readFileSync(new URL("../../../services/human/schema/human-api/golden/account.balance.response.json", import.meta.url), "utf8")) as SupportRecording;
  const object = (value: JsonValue): { [key: string]: JsonValue } => {
    assert.ok(value !== null && typeof value === "object" && !Array.isArray(value));
    return value;
  };
  const createdValue = decodeSupportConversation(object(created.body).result, "recorded create");
  const repliedValue = decodeSupportConversation(object(feedback.body).result, "recorded feedback");
  const requests: { path: string; method: string; body: unknown; idempotency: string | undefined }[] = [];
  const errors: unknown[] = [];
  const journal = new Map<string, string>();
  const storage: SupportPendingStorage = {
    getItem(key) { return journal.get(key) ?? null; },
    setItem(key, value) { journal.set(key, value); },
    removeItem(key) { journal.delete(key); },
  };
  let fail: "typed" | "transport" | undefined;
  let hasConversation = false;
  const server = createServer(async (request, response) => {
    try {
      const path = request.url ?? "";
      const method = request.method ?? "";
      assert.ok(path === SUPPORT_ACCOUNT || path === SUPPORT_CONVERSATIONS || path.startsWith(`${SUPPORT_CONVERSATIONS}/${createdValue.conversation_id}/`), `Unexpected support fixture route ${method} ${path}`);
      assert.equal(request.headers.authorization, undefined);
      let text = "";
      for await (const chunk of request) text += String(chunk);
      const body: unknown = text.length === 0 ? undefined : JSON.parse(text);
      requests.push({ path, method, body, idempotency: request.headers["idempotency-key"] as string | undefined });
      if (path === SUPPORT_ACCOUNT) {
        assert.equal(method, "GET");
        assert.equal(body, undefined);
        response.writeHead(account.status, { "content-type": "application/json" });
        response.end(JSON.stringify(account.body));
        return;
      }
      if (method === "POST") {
        assert.equal(request.headers["x-layerx-csrf"], "support-proof-csrf");
        assert.equal(request.headers["content-type"], "application/json");
      }
      if (fail !== undefined) {
        const kind = fail;
        fail = undefined;
        if (kind === "transport") { request.socket.destroy(); return; }
        response.writeHead(failure.status, { "content-type": "application/json" });
        response.end(JSON.stringify(failure.body));
        return;
      }
      let recording: SupportRecording;
      if (path === SUPPORT_CONVERSATIONS && method === "GET") {
        recording = hasConversation ? {
          ...empty,
          body: { ...object(empty.body), result: encodeSupportConversationPage({ conversations: [repliedValue] }) },
        } : empty;
      } else if (path === SUPPORT_CONVERSATIONS && method === "POST") {
        assert.ok(request.headers["idempotency-key"]);
        assert.deepEqual(body, supportRecording("create", "request").body);
        hasConversation = true;
        recording = created;
      } else if (path.endsWith("/replies") && method === "POST") {
        assert.ok(request.headers["idempotency-key"]);
        assert.deepEqual(body, supportRecording("reply", "request").body);
        recording = reply;
      } else if (path.endsWith("/feedback") && method === "POST") {
        const recordedFeedback = repliedValue.feedback[0]!;
        assert.deepEqual(body, { message_id: recordedFeedback.message_id, helpful: recordedFeedback.helpful });
        recording = feedback;
      } else if (path.endsWith("/status") && method === "GET") {
        recording = supportRecording("status");
      } else if (path.endsWith("/read") && method === "POST") {
        recording = supportRecording("read");
      } else throw new Error(`Unexpected support operation ${method} ${path}`);
      response.writeHead(recording.status, { "content-type": "application/json" });
      response.end(JSON.stringify(recording.body));
    } catch (error) {
      errors.push(error);
      response.writeHead(500, { "content-type": "text/plain" });
      response.end(error instanceof Error ? error.message : String(error));
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  try {
    const address = server.address();
    assert.ok(address !== null && typeof address !== "string");
    const client = humanApi({ baseUrl: `http://127.0.0.1:${address.port}/human`, csrfToken: () => "support-proof-csrf" });
    await run({ client, requests, errors, storage, failNext(kind) { fail = kind; } });
    assert.deepEqual(errors, []);
  } finally {
    await new Promise<void>((resolve, reject) => {
      server.close(error => error === undefined ? resolve() : reject(error));
      server.closeAllConnections();
    });
  }
}
