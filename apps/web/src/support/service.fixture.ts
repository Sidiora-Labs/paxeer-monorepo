// In-memory contract peer for support journey proofs. State and idempotency follow
// services/human/crates/layerx-human-service/src/support.rs; wire codecs are generated.
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";

import {
  decodeSupportCreateRequest,
  decodeSupportReplyRequest,
  decodeSupportReadRequest,
  decodeSupportFeedbackRequest,
  encodeSupportConversation,
  encodeSupportConversationPage,
  encodeSupportConversationStatus,
  type JsonValue,
  type SupportConversation,
  type SupportMessage,
} from "../api/index.ts";

function golden(name: string): { status: number; body: JsonValue } {
  return JSON.parse(readFileSync(new URL(`../../../../services/human/schema/human-api/golden/${name}.json`, import.meta.url), "utf8"));
}

function identity(domain: string, principal: string, key: string): string {
  return createHash("sha256").update(`${domain}\0${principal}\0${key}`).digest("hex");
}

class SupportRefusal extends Error {
  readonly code: string;
  constructor(code: string) { super(code); this.code = code; }
}

export class SupportServiceFixture {
  principal = "act_support_customer_a";
  unavailable = false;
  refuseMutations = false;
  listFault: "open-state" | "missing-shell" | undefined;
  holdList: Promise<void> | undefined;
  loseNextMutationResponse = false;
  readonly requests: { method: string; path: string; principal: string; key: string | null; body: JsonValue }[] = [];
  readonly #rows = new Map<string, Map<string, SupportConversation>>();
  readonly #creates = new Map<string, string>();
  #clock = Date.parse("2026-08-20T09:00:00Z");

  #now(): string { return new Date(this.#clock += 1_000).toISOString(); }

  conversations(principal = this.principal): SupportConversation[] {
    return structuredClone([...(this.#rows.get(principal)?.values() ?? [])]);
  }

  #scope(principal: string): Map<string, SupportConversation> {
    let rows = this.#rows.get(principal);
    if (rows === undefined) { rows = new Map(); this.#rows.set(principal, rows); }
    return rows;
  }

  #conversation(principal: string, id: string): SupportConversation {
    const row = this.#scope(principal).get(id);
    if (row === undefined) throw new SupportRefusal("support-conversation-unknown");
    return row;
  }

  #append(principal: string, id: string, key: string, author: SupportMessage["author"], body: string): SupportConversation {
    const row = this.#conversation(principal, id);
    if (author === "you" && row.state === "resolved") throw new SupportRefusal("support-conversation-resolved");
    const messageId = `msg_${identity("LayerX support message v1", id, key)}`;
    const existing = row.messages.find((entry) => entry.message_id === messageId);
    if (existing !== undefined) {
      if (existing.author !== author || existing.body !== body) throw new SupportRefusal("support-idempotency-conflict");
      return row;
    }
    if (body.trim().length === 0 || body.length > 2_000) throw new SupportRefusal("support-message-invalid");
    const now = this.#now();
    row.messages.push({ message_id: messageId, author, body, sent_at: now, read: author === "you" });
    row.state = author === "you" ? "waiting-for-support" : "waiting-for-you";
    row.updated_at = now;
    return row;
  }

  recordSupportReply(id: string, key: string, body: string, principal = this.principal): SupportConversation {
    return structuredClone(this.#append(principal, id, key, "support", body));
  }

  resolve(id: string, principal = this.principal): void {
    const row = this.#conversation(principal, id);
    row.state = "resolved";
    row.updated_at = this.#now();
  }

  async handle(request: Request): Promise<Response> {
    const path = new URL(request.url).pathname;
    const method = request.method;
    const key = request.headers.get("Idempotency-Key");
    const body = method === "GET" ? null : await request.json() as JsonValue;
    const principal = this.principal;
    this.requests.push({ method, path, principal, key, body });
    const json = (value: JsonValue, status = 200) => Response.json(value, { status });
    if (this.unavailable || (this.refuseMutations && method === "POST")) {
      const failure = golden("support.create.failure");
      return json(failure.body, failure.status);
    }
    if (path === "/human/v1/account/balance") {
      const fixture = golden("account.balance.response");
      const envelope = fixture.body as { [key: string]: JsonValue };
      const account = envelope.result as { [key: string]: JsonValue };
      account.account_id = principal;
      return json(envelope);
    }
    let result: JsonValue;
    let status = 200;
    try {
      if (path === "/human/v1/support/conversations" && method === "GET") {
        if (this.holdList !== undefined) await this.holdList;
        result = encodeSupportConversationPage({ conversations: this.conversations(principal) });
        if (this.listFault !== undefined) {
          const page = result as { conversations: { [key: string]: JsonValue }[] };
          const first = page.conversations[0];
          if (first === undefined) throw new Error("Fault requires an actual conversation");
          if (this.listFault === "open-state") first.state = "open";
          else delete first.shell;
        }
      } else if (path === "/human/v1/support/conversations" && method === "POST") {
        if (key === null) throw new SupportRefusal("idempotency-key-invalid");
        const input = decodeSupportCreateRequest(body, "support create fixture");
        const id = `sup_${identity("LayerX support conversation v1", principal, key)}`;
        let row = this.#scope(principal).get(id);
        const fingerprint = JSON.stringify(input);
        if (row !== undefined) {
          if (this.#creates.get(id) !== fingerprint) throw new SupportRefusal("support-idempotency-conflict");
        } else {
          const now = this.#now();
          row = { conversation_id: id, shell: input.shell, state: "waiting-for-support", created_at: now,
            updated_at: now, feedback: [], messages: [{
              message_id: `msg_${identity("LayerX support message v1", id, key)}`, author: "you", body: input.body,
              sent_at: now, read: true, ...(input.topic === undefined ? {} : { topic: input.topic }),
            }], ...(input.trace_id === undefined ? {} : { trace_id: input.trace_id }) };
          this.#scope(principal).set(id, row);
          this.#creates.set(id, fingerprint);
        }
        result = encodeSupportConversation(row);
        status = 201;
      } else {
        const match = path.match(/^\/human\/v1\/support\/conversations\/([^/]+)\/(replies|status|read|feedback)$/u);
        if (match?.[1] === undefined) return new Response("Unregistered fixture route", { status: 404 });
        const row = this.#conversation(principal, decodeURIComponent(match[1]));
        if (match[2] === "replies" && method === "POST") {
          if (key === null) throw new SupportRefusal("idempotency-key-invalid");
          result = encodeSupportConversation(this.#append(principal, row.conversation_id, key, "you",
            decodeSupportReplyRequest(body, "fixture reply").body));
        } else if (match[2] === "read" || match[2] === "status") {
          if (match[2] === "read") {
            const input = decodeSupportReadRequest(body, "fixture read");
            const through = row.messages.findIndex((entry) => entry.message_id === input.through_message_id);
            if (through < 0) throw new SupportRefusal("support-message-unknown");
            for (const message of row.messages.slice(0, through + 1)) message.read = true;
          }
          result = encodeSupportConversationStatus({ conversation_id: row.conversation_id, state: row.state,
            updated_at: row.updated_at, unread_count: row.messages.filter((entry) => entry.author === "support" && !entry.read).length });
        } else if (match[2] === "feedback" && method === "POST") {
          const input = decodeSupportFeedbackRequest(body, "fixture feedback");
          if (!row.messages.some((entry) => entry.message_id === input.message_id && entry.author === "support")) {
            throw new SupportRefusal("support-message-unknown");
          }
          row.feedback = row.feedback.filter((entry) => entry.message_id !== input.message_id);
          row.feedback.push({ ...input, received_at: this.#now() });
          result = encodeSupportConversation(row);
        } else return new Response("Unregistered fixture method", { status: 405 });
      }
    } catch (error) {
      if (!(error instanceof SupportRefusal)) throw error;
      return json({ ok: false, trace: "trc_support_fixture_refusal", error: {
        code: error.code, copy_key: "error.support.unavailable", retry: "retriable",
      } }, 409);
    }
    if (method === "POST" && this.loseNextMutationResponse) {
      this.loseNextMutationResponse = false;
      return new Response("Committed; response lost", { status: 502 });
    }
    return json({ ok: true, trace: "trc_support_fixture", result }, status);
  }
}

export async function openSupportFixture() {
  const service = new SupportServiceFixture();
  const server = createServer(async (incoming, outgoing) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of incoming) chunks.push(Buffer.from(chunk));
      const body = Buffer.concat(chunks).toString();
      const headers = new Headers();
      for (const [name, value] of Object.entries(incoming.headers)) {
        if (typeof value === "string") headers.set(name, value);
      }
      const response = await service.handle(new Request(`http://localhost${incoming.url}`, {
        method: incoming.method ?? "GET", headers, ...(body.length === 0 ? {} : { body }),
      }));
      outgoing.writeHead(response.status, Object.fromEntries(response.headers));
      outgoing.end(await response.text());
    } catch (error) {
      outgoing.writeHead(500).end(String(error));
    }
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  if (address === null || typeof address === "string") throw new Error("Support fixture did not listen");
  return {
    service,
    baseUrl: `http://127.0.0.1:${address.port}/human`,
    close: () => new Promise<void>((resolve, reject) => {
      server.close((error) => error === undefined ? resolve() : reject(error));
      server.closeAllConnections();
    }),
  };
}
