import type { Hex } from "viem";

type Eip1193RequestArgs = {
  method: string;
  params?: unknown[] | object;
};

type ProviderEventName = "accountsChanged" | "chainChanged" | "connect" | "disconnect" | "message";

type ProviderListener = (...args: unknown[]) => void;

type PendingRequest = {
  resolve: (value: unknown) => void;
  reject: (reason?: unknown) => void;
  timeout: ReturnType<typeof setTimeout>;
};

export const POST_MESSAGE_REQUEST_TIMEOUT_MS = 30_000;

type BridgeRequestMessage = {
  type: "SIDIORA_EIP1193_REQUEST";
  id: string;
  method: string;
  params?: unknown[] | object;
};

type BridgeResponseMessage = {
  type: "SIDIORA_EIP1193_RESPONSE";
  id: string;
  result?: unknown;
  error?: { code: number; message: string; data?: unknown };
};

export type BridgeAccountsChangedEventMessage = {
  type: "SIDIORA_EIP1193_EVENT";
  event: "accountsChanged";
  payload: Hex[];
};

type BridgeEventMessage =
  | BridgeAccountsChangedEventMessage
  | {
      type: "SIDIORA_EIP1193_EVENT";
      event: Exclude<ProviderEventName, "accountsChanged">;
      payload?: unknown;
    };

type BridgeInboundMessage = BridgeResponseMessage | BridgeEventMessage;

type BridgeMessage = BridgeRequestMessage | BridgeInboundMessage;

export function postAccountsChangedEvent(target: Window, targetOrigin: string, accounts: Hex[]): void {
  const message: BridgeAccountsChangedEventMessage = {
    type: "SIDIORA_EIP1193_EVENT",
    event: "accountsChanged",
    payload: accounts,
  };
  target.postMessage(message, targetOrigin);
}

function isBridgeMessage(data: unknown): data is BridgeInboundMessage {
  if (!isRecord(data)) return false;
  if (data.type === "SIDIORA_EIP1193_RESPONSE") {
    if (typeof data.id !== "string" || !data.id) return false;
    const hasResult = Object.hasOwn(data, "result");
    const hasError = Object.hasOwn(data, "error");
    return hasResult !== hasError && (!hasError || isProviderError(data.error));
  }
  if (data.type !== "SIDIORA_EIP1193_EVENT") return false;
  switch (data.event) {
    case "accountsChanged":
      return Array.isArray(data.payload) && data.payload.every((account: unknown) => typeof account === "string" && /^0x[0-9a-f]{40}$/i.test(account));
    case "chainChanged":
      return isChainId(data.payload);
    case "connect":
      return isRecord(data.payload) && isChainId(data.payload.chainId);
    case "disconnect":
      return isProviderError(data.payload);
    case "message":
      return isRecord(data.payload) && typeof data.payload.type === "string" && data.payload.type.length > 0 && Object.hasOwn(data.payload, "data");
    default:
      return false;
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function isChainId(value: unknown): value is string {
  return typeof value === "string" && /^0x[0-9a-f]+$/i.test(value);
}

function isProviderError(value: unknown): value is { code: number; message: string; data?: unknown } {
  return isRecord(value) && typeof value.code === "number" && Number.isSafeInteger(value.code) && typeof value.message === "string";
}

function createProviderError(error?: { code?: number; message?: string; data?: unknown }): Error & { code?: number; data?: unknown } {
  const providerError = new Error(error?.message || "EIP-1193 request failed") as Error & { code?: number; data?: unknown };
  providerError.code = error?.code;
  providerError.data = error?.data;
  return providerError;
}

export class PostMessageEip1193Provider {
  private readonly targetOrigin: string;
  private readonly pending = new Map<string, PendingRequest>();
  private readonly listeners = new Map<ProviderEventName, Set<ProviderListener>>();
  private readonly handleMessageBound: (event: MessageEvent<unknown>) => void;
  private readonly handlePageHideBound: () => void;
  private mounted = false;
  private connected = false;
  private nextRequestId = 0;
  readonly isSidioraPostMessageProvider = true;

  constructor(targetOrigin: string) {
    this.targetOrigin = targetOrigin;
    this.handleMessageBound = this.handleMessage.bind(this);
    this.handlePageHideBound = this.unmount.bind(this);
  }

  mount() {
    if (this.mounted) return;
    this.mounted = true;
    this.connected = true;
    window.addEventListener("message", this.handleMessageBound);
    window.addEventListener("pagehide", this.handlePageHideBound);
  }

  unmount() {
    this.mounted = false;
    this.connected = false;
    window.removeEventListener("message", this.handleMessageBound);
    window.removeEventListener("pagehide", this.handlePageHideBound);
    this.rejectPending("PostMessage EIP-1193 provider unmounted; reconnect and retry");
  }

  request(args: Eip1193RequestArgs): Promise<unknown> {
    if (!this.mounted || !this.connected) {
      return Promise.reject(createProviderError({ code: 4900, message: "PostMessage EIP-1193 wallet disconnected; reconnect and retry" }));
    }
    if (!window.parent || window.parent === window) {
      return Promise.reject(new Error("PostMessage EIP-1193 provider requires iframe context"));
    }

    const id = `${Date.now()}-${this.nextRequestId++}-${Math.random().toString(16).slice(2)}`;
    const message: BridgeRequestMessage = {
      type: "SIDIORA_EIP1193_REQUEST",
      id,
      method: args.method,
      params: args.params,
    };

    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        this.takePending(id)?.reject(
          createProviderError({ code: 4900, message: `PostMessage EIP-1193 request timed out after ${POST_MESSAGE_REQUEST_TIMEOUT_MS}ms; retry the request` }),
        );
      }, POST_MESSAGE_REQUEST_TIMEOUT_MS);
      this.pending.set(id, { resolve, reject, timeout });
      try {
        window.parent.postMessage(message, this.targetOrigin);
      } catch (error) {
        this.takePending(id)?.reject(error);
      }
    });
  }

  private takePending(id: string): PendingRequest | undefined {
    const request = this.pending.get(id);
    if (request) {
      this.pending.delete(id);
      clearTimeout(request.timeout);
    }
    return request;
  }

  private rejectPending(message: string): void {
    for (const id of this.pending.keys()) {
      this.takePending(id)?.reject(createProviderError({ code: 4900, message }));
    }
  }

  on(event: string, listener: ProviderListener): void {
    const listeners = this.listeners.get(event as ProviderEventName) ?? new Set<ProviderListener>();
    listeners.add(listener);
    this.listeners.set(event as ProviderEventName, listeners);
  }

  removeListener(event: string, listener: ProviderListener): void {
    this.listeners.get(event as ProviderEventName)?.delete(listener);
  }

  emit(event: ProviderEventName, payload?: unknown) {
    const listeners = this.listeners.get(event);
    if (!listeners) return;
    for (const listener of listeners) {
      if (event === "accountsChanged") {
        listener(payload);
      } else if (Array.isArray(payload)) {
        listener(...payload);
      } else if (payload === undefined) {
        listener();
      } else {
        listener(payload);
      }
    }
  }

  private handleMessage(event: MessageEvent<unknown>) {
    if (event.source !== window.parent || event.origin !== this.targetOrigin) return;
    if (!isBridgeMessage(event.data)) return;

    if (event.data.type === "SIDIORA_EIP1193_RESPONSE") {
      const pendingRequest = this.takePending(event.data.id);
      if (!pendingRequest) return;
      if (event.data.error) {
        pendingRequest.reject(createProviderError(event.data.error));
        return;
      }
      pendingRequest.resolve(event.data.result);
      return;
    }

    if (event.data.event === "connect" || event.data.event === "disconnect") {
      this.connected = event.data.event === "connect";
      this.rejectPending(`PostMessage EIP-1193 wallet ${this.connected ? "reconnected" : "disconnected"}; retry the request`);
    }
    this.emit(event.data.event, event.data.payload);
  }
}

export function installPostMessageEip1193Provider() {
  if (typeof window === "undefined") return null;
  if (window.parent === window) return null;

  const targetOrigin = process.env.NEXT_PUBLIC_WALLET_IFRAME_ORIGIN;
  if (!targetOrigin) return null;

  const existingProvider = window.ethereum as (Window["ethereum"] & { isSidioraPostMessageProvider?: boolean; unmount?: () => void }) | undefined;
  if (existingProvider?.isSidioraPostMessageProvider) {
    const provider = existingProvider as PostMessageEip1193Provider;
    provider.mount();
    return provider;
  }

  const provider = new PostMessageEip1193Provider(targetOrigin);
  provider.mount();
  window.ethereum = provider as Window["ethereum"];

  window.dispatchEvent(new Event("ethereum#initialized"));
  return provider;
}
