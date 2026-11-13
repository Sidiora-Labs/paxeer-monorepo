import { copyEntry } from "../../copy/runtime.ts";
import {
  abortableHumanApi,
  decodeSupportCreateRequest,
  decodeSupportReplyRequest,
  type JsonValue,
  type SupportCreateRequest,
  type SupportReplyRequest,
  type HumanApiClient,
  type SupportConversation,
  type SupportConversationState,
  type SupportMessage,
  type SupportShell,
  type SupportTopic,
  type TraceId,
} from "../api/index.ts";
import { supportTopic } from "./topics.ts";

export type SupportChatDelivery = "sending" | "sent" | "failed";
export type SupportChatConnection = "online" | "offline";
export type SupportChatLoad = "loading" | "ready" | "failed";
export type SupportFeedbackDelivery = "idle" | "sending" | "saved" | "failed";

export interface SupportChatEntry extends SupportMessage {
  readonly delivery: SupportChatDelivery;
}

export interface SupportChatSnapshot {
  readonly load: SupportChatLoad;
  readonly conversationId?: string;
  readonly conversationState?: SupportConversationState;
  readonly traceId?: TraceId;
  readonly draft: string;
  readonly topic?: SupportTopic;
  readonly connection: SupportChatConnection;
  readonly entries: readonly SupportChatEntry[];
  readonly feedback: ReadonlyMap<string, boolean>;
  readonly feedbackDelivery: ReadonlyMap<string, SupportFeedbackDelivery>;
  readonly awaitingFeedbackId?: string;
  readonly sending: boolean;
}

type PendingOperation =
  | Readonly<{ kind: "create"; request: SupportCreateRequest }>
  | Readonly<{ kind: "reply"; conversationId: string; request: SupportReplyRequest }>;

interface PendingMessage {
  readonly idempotencyKey: string;
  readonly messageId: string;
  readonly operation: PendingOperation;
  state: "sending" | "failed";
}

export type SupportPendingStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

export interface SupportChatSessionOptions {
  readonly shell: SupportShell;
  readonly traceId?: TraceId;
  readonly client?: HumanApiClient;
  readonly storage?: SupportPendingStorage;
}

function restorePending(raw: string): PendingMessage[] {
  const rows: unknown = JSON.parse(raw);
  if (!Array.isArray(rows)) throw new Error("Invalid support pending journal");
  return rows.map((value: unknown) => {
    if (typeof value !== "object" || value === null) throw new Error("Invalid support attempt");
    const row = value as Record<string, unknown>;
    if (typeof row.idempotencyKey !== "string" || typeof row.messageId !== "string"
      || typeof row.operation !== "object" || row.operation === null) {
      throw new Error("Invalid support operation identity");
    }
    const operation = row.operation as Record<string, unknown>;
    let decoded: PendingOperation;
    if (operation.kind === "create") {
      decoded = { kind: "create", request: decodeSupportCreateRequest(operation.request as JsonValue, "pending create") };
    } else if (operation.kind === "reply" && typeof operation.conversationId === "string") {
      decoded = { kind: "reply", conversationId: operation.conversationId,
        request: decodeSupportReplyRequest(operation.request as JsonValue, "pending reply") };
    } else {
      throw new Error("Invalid support operation target");
    }
    return { idempotencyKey: row.idempotencyKey, messageId: row.messageId, operation: decoded, state: "failed" };
  });
}

function requestIdentity(kind: "create" | "reply"): Readonly<{ key: string; messageId: string }> {
  const identity = crypto.randomUUID();
  return Object.freeze({ key: `support-${kind}-${identity}`, messageId: `pending_${identity}` });
}

function offlineFailure(error: unknown): boolean {
  return typeof navigator !== "undefined" && !navigator.onLine
    || error instanceof TypeError;
}

export class SupportChatSession {
  readonly #shell: SupportShell;
  readonly #traceId: TraceId | undefined;
  readonly #client: HumanApiClient;
  readonly #listeners = new Set<() => void>();
  #conversation: SupportConversation | undefined;
  #pending: PendingMessage[] = [];
  #draft = "";
  #topic: SupportTopic | undefined;
  #connection: SupportChatConnection = "online";
  #load: SupportChatLoad = "loading";
  #feedbackDelivery = new Map<string, SupportFeedbackDelivery>();
  #snapshot: SupportChatSnapshot;
  readonly #storageOption: SupportPendingStorage | undefined;
  #principal: string | undefined;
  #storage: SupportPendingStorage | undefined;
  #generation = 0;
  #stopped = false;
  #polling = false;
  #controller = new AbortController();
  #timer: ReturnType<typeof setTimeout> | undefined;
  #queue: Promise<void> = Promise.resolve();

  constructor(options: SupportChatSessionOptions) {
    this.#shell = options.shell;
    this.#traceId = options.traceId;
    this.#client = options.client ?? abortableHumanApi(
      () => AbortSignal.any([this.#controller.signal, AbortSignal.timeout(10_000)]),
    );
    this.#storageOption = options.storage;
    this.#snapshot = this.#buildSnapshot();
  }

  subscribe = (listener: () => void): (() => void) => {
    this.#listeners.add(listener);
    return () => {
      this.#listeners.delete(listener);
    };
  };

  snapshot = (): SupportChatSnapshot => this.#snapshot;

  start(): void {
    if (this.#polling) return;
    this.#polling = true;
    this.#stopped = false;
    this.#controller = new AbortController();
    const generation = this.#generation;
    const poll = async () => {
      await this.initialize();
      if (!this.#stopped && generation === this.#generation) {
        this.#timer = setTimeout(() => { void poll(); }, 5_000);
      }
    };
    void poll();
  }

  stop(): void {
    this.#polling = false;
    this.#stopped = true;
    this.#generation += 1;
    clearTimeout(this.#timer);
    this.#controller.abort();
  }

  #enqueue(work: (generation: number) => Promise<void>): Promise<void> {
    const generation = this.#generation;
    const stopped = () => this.#stopped;
    const next = this.#queue.then(async () => {
      if (stopped() || generation !== this.#generation) return;
      try {
        await work(generation);
      } catch (error) {
        if (stopped() || generation !== this.#generation) return;
        this.#connection = offlineFailure(error) ? "offline" : "online";
        this.#load = "failed";
        this.#notify();
      }
    });
    this.#queue = next;
    return next;
  }

  #check(generation: number): void {
    if (this.#stopped || generation !== this.#generation) throw new Error("Support session ended");
  }

  async #verifyPrincipal(generation: number): Promise<void> {
    const account = await this.#client.accountBalance();
    this.#check(generation);
    if (this.#principal !== undefined && this.#principal !== account.account_id) {
      this.#conversation = undefined;
      this.#pending = [];
      this.#draft = "";
      this.#topic = undefined;
      this.#feedbackDelivery.clear();
      this.#load = "failed";
      this.#notify();
      this.stop();
      throw new Error("Support principal changed");
    }
    if (this.#principal === undefined) {
      if (account.account_id.length === 0) throw new Error("Support principal is missing");
      const storage = this.#storageOption ?? window.sessionStorage;
      const raw = storage.getItem(`layerx.support.pending.v1.${account.account_id}`);
      const pending = raw === null ? [] : restorePending(raw);
      this.#principal = account.account_id;
      this.#storage = storage;
      this.#pending = pending;
    }
  }

  #persist(pending: readonly PendingMessage[]): void {
    if (this.#storage === undefined || this.#principal === undefined) throw new Error("Support journal unavailable");
    const key = `layerx.support.pending.v1.${this.#principal}`;
    if (pending.length === 0) this.#storage.removeItem(key);
    else this.#storage.setItem(key, JSON.stringify(pending));
  }

  initialize(): Promise<void> {
    return this.#enqueue(async (generation) => {
      await this.#verifyPrincipal(generation);
      const page = await this.#client.supportList();
      this.#check(generation);
      this.#conversation = page.conversations.find((entry) => entry.conversation_id === this.#conversation?.conversation_id)
        ?? this.#selectConversation(page.conversations);
      this.#connection = "online";
      this.#load = "ready";
      this.#notify();
      await this.#recover(generation);
      await this.#markVisibleRepliesRead(generation);
    });
  }

  setOnline(online: boolean): void {
    this.#connection = online ? "online" : "offline";
    this.#notify();
  }

  seedTopic(id: SupportTopic): void {
    const topic = supportTopic(id);
    this.#topic = id;
    this.#draft = copyEntry(topic.seedKey).message;
    this.#notify();
  }

  setDraft(value: string): void {
    this.#draft = value.slice(0, 2_000);
    this.#notify();
  }

  send(): Promise<void> {
    return this.#enqueue(async (generation) => {
      if (this.#draft.trim().length === 0) return;
      if (this.#principal === undefined || this.#connection !== "offline") await this.#verifyPrincipal(generation);
      if (!await this.#recover(generation)) return;
      const body = this.#draft.trim();
      const kind = this.#conversation === undefined ? "create" : "reply";
      const identity = requestIdentity(kind);
      const operation: PendingOperation = this.#conversation === undefined
        ? { kind: "create", request: {
          body, shell: this.#shell,
          ...(this.#topic === undefined ? {} : { topic: this.#topic }),
          ...(this.#traceId === undefined ? {} : { trace_id: this.#traceId }),
        } }
        : { kind: "reply", conversationId: this.#conversation.conversation_id, request: { body } };
      const message: PendingMessage = {
        idempotencyKey: identity.key, messageId: identity.messageId, operation, state: "sending",
      };
      const pending = [...this.#pending, message];
      this.#persist(pending);
      this.#pending = pending;
      this.#draft = "";
      this.#topic = undefined;
      this.#notify();
      await this.#deliver(message, generation);
    });
  }

  retry(messageId: string): Promise<void> {
    return this.#enqueue(async (generation) => {
      await this.#verifyPrincipal(generation);
      if (this.#pending.some((candidate) => candidate.messageId === messageId)) await this.#recover(generation);
    });
  }

  retryFailed(): Promise<void> {
    return this.initialize();
  }

  async #recover(generation: number): Promise<boolean> {
    for (const message of [...this.#pending]) {
      if (!await this.#deliver(message, generation)) return false;
    }
    return true;
  }

  sendFeedback(messageId: string, helpful: boolean): Promise<void> {
    return this.#enqueue(async (generation) => {
      await this.#verifyPrincipal(generation);
      if (this.#conversation === undefined) return;
      this.#feedbackDelivery.set(messageId, "sending");
      this.#notify();
      try {
        const conversation = await this.#client.supportFeedback(
          this.#conversation.conversation_id, { message_id: messageId, helpful },
        );
        this.#check(generation);
        this.#conversation = conversation;
        this.#feedbackDelivery.set(messageId, "saved");
        this.#connection = "online";
      } catch (error) {
        this.#check(generation);
        this.#feedbackDelivery.set(messageId, "failed");
        if (offlineFailure(error)) this.#connection = "offline";
      }
      this.#notify();
    });
  }

  async #deliver(message: PendingMessage, generation: number): Promise<boolean> {
    message.state = "sending";
    this.#notify();
    try {
      await this.#verifyPrincipal(generation);
      const operation = message.operation;
      const conversation = operation.kind === "create"
        ? await this.#client.supportCreate(operation.request, message.idempotencyKey)
        : await this.#client.supportReply(operation.conversationId, operation.request, message.idempotencyKey);
      this.#check(generation);
      const remaining = this.#pending.filter((candidate) => candidate.messageId !== message.messageId);
      this.#persist(remaining);
      this.#conversation = conversation;
      this.#pending = remaining;
      this.#connection = "online";
      this.#load = "ready";
      this.#notify();
      return true;
    } catch (error) {
      this.#check(generation);
      message.state = "failed";
      if (offlineFailure(error)) this.#connection = "offline";
      this.#notify();
      return false;
    }
  }

  #selectConversation(conversations: readonly SupportConversation[]): SupportConversation | undefined {
    if (this.#traceId !== undefined) {
      return conversations.find((conversation) => conversation.trace_id === this.#traceId);
    }
    return conversations.find((conversation) => conversation.state !== "resolved");
  }

  async #markVisibleRepliesRead(generation: number): Promise<void> {
    const conversation = this.#conversation;
    const lastUnread = conversation?.messages.findLast(
      (message) => message.author === "support" && !message.read,
    );
    if (conversation === undefined || lastUnread === undefined) {
      return;
    }
    try {
      await this.#client.supportStatus(conversation.conversation_id);
      this.#check(generation);
      await this.#client.supportRead(
        conversation.conversation_id,
        { through_message_id: lastUnread.message_id },
      );
      this.#check(generation);
      this.#conversation = {
        ...conversation,
        messages: conversation.messages.map((message) => (
          message.author === "support" ? { ...message, read: true } : message
        )),
      };
      this.#notify();
    } catch (error) {
      this.#check(generation);
      if (offlineFailure(error)) {
        this.#connection = "offline";
        this.#notify();
      }
    }
  }

  #notify(): void {
    this.#snapshot = this.#buildSnapshot();
    for (const listener of this.#listeners) {
      listener();
    }
  }

  #buildSnapshot(): SupportChatSnapshot {
    const delivered: SupportChatEntry[] = (this.#conversation?.messages ?? []).map((message) => ({
      ...message,
      delivery: "sent" as const,
    }));
    const pending: SupportChatEntry[] = this.#pending.map((message) => ({
      message_id: message.messageId,
      author: "you" as const,
      body: message.operation.request.body,
      sent_at: new Date().toISOString(),
      read: true,
      ...(message.operation.kind === "create" && message.operation.request.topic !== undefined
        ? { topic: message.operation.request.topic } : {}),
      delivery: message.state,
    }));
    const feedback = new Map<string, boolean>(
      (this.#conversation?.feedback ?? []).map((entry) => [entry.message_id, entry.helpful]),
    );
    const awaiting = [...delivered].reverse().find((entry) => (
      entry.author === "support"
      && !feedback.has(entry.message_id)
      && this.#feedbackDelivery.get(entry.message_id) !== "saved"
    ));
    return Object.freeze({
      load: this.#load,
      ...(this.#conversation === undefined ? {} : {
        conversationId: this.#conversation.conversation_id,
        conversationState: this.#conversation.state,
      }),
      ...(this.#traceId === undefined ? {} : { traceId: this.#traceId }),
      draft: this.#draft,
      ...(this.#topic === undefined ? {} : { topic: this.#topic }),
      connection: this.#connection,
      entries: Object.freeze([...delivered, ...pending]),
      feedback,
      feedbackDelivery: new Map(this.#feedbackDelivery),
      ...(awaiting === undefined ? {} : { awaitingFeedbackId: awaiting.message_id }),
      sending: this.#pending.some((message) => message.state === "sending"),
    });
  }
}
