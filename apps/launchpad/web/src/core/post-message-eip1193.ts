'use client';

import { checkedWalletParentOrigin, walletParentOrigins } from '@/core/backend-origins';

type JsonRpcRequest = {
  method: string;
  params?: unknown[] | Record<string, unknown>;
};

type JsonRpcError = {
  code: number;
  message: string;
  data?: unknown;
};

type PostMessageRpcRequest = {
  target: 'sidiora:eip1193';
  type: 'request';
  id: string;
  request: JsonRpcRequest;
};

type PostMessageRpcResponse = {
  target: 'sidiora:eip1193';
  type: 'response';
  id: string;
  result?: unknown;
  error?: JsonRpcError;
};

type PostMessageRpcEvent = {
  target: 'sidiora:eip1193';
  type: 'event';
  event: string;
  params?: unknown;
};

type PostMessageRpcReady = {
  target: 'sidiora:eip1193';
  type: 'ready';
};

type ParentMessage = PostMessageRpcResponse | PostMessageRpcEvent | PostMessageRpcReady;

type Handler = (...args: any[]) => void;

function randomId() {
  return crypto.randomUUID();
}

function isObject(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

function isRpcError(value: unknown): value is JsonRpcError {
  return isObject(value) && Number.isSafeInteger(value.code) && typeof value.message === 'string';
}

function isParentMessage(v: unknown): v is ParentMessage {
  if (!isObject(v) || v.target !== 'sidiora:eip1193') return false;
  if (v.type === 'ready') return true;
  if (v.type === 'response') {
    return (
      typeof v.id === 'string' &&
      /^[0-9a-f-]{36}$/i.test(v.id) &&
      Object.hasOwn(v, 'result') !== Object.hasOwn(v, 'error') &&
      (!Object.hasOwn(v, 'error') || isRpcError(v.error))
    );
  }
  if (v.type !== 'event') return false;
  switch (v.event) {
    case 'accountsChanged':
      return (
        Array.isArray(v.params) &&
        v.params.every(
          (account) => typeof account === 'string' && /^0x[0-9a-fA-F]{40}$/.test(account),
        )
      );
    case 'chainChanged':
      return typeof v.params === 'string' && /^0x[0-9a-fA-F]+$/.test(v.params);
    case 'connect':
      return (
        isObject(v.params) &&
        typeof v.params.chainId === 'string' &&
        /^0x[0-9a-fA-F]+$/.test(v.params.chainId)
      );
    case 'disconnect':
      return isRpcError(v.params);
    default:
      return false;
  }
}

function disconnectedError(): Error & { code: number } {
  return Object.assign(new Error('postMessage provider is disconnected'), { code: 4900 });
}

export type InstallPostMessageEip1193Options = {
  allowedParentOrigins?: string[];
  targetOrigin?: string;
  force?: boolean;
};

export type Eip1193Provider = {
  request: (args: JsonRpcRequest) => Promise<unknown>;
  on: (event: string, handler: Handler) => void;
  removeListener: (event: string, handler: Handler) => void;
  isMetaMask?: boolean;
  isCoinbaseWallet?: boolean;
  isBraveWallet?: boolean;
  isRainbow?: boolean;
  isSidioraPostMessage?: boolean;
};

class PostMessageEip1193Provider implements Eip1193Provider {
  public isSidioraPostMessage = true;

  private destroyed = false;
  private parentOrigin: string | undefined;

  private handlers = new Map<string, Set<Handler>>();
  private inflight = new Map<
    string,
    {
      resolve: (v: unknown) => void;
      reject: (e: unknown) => void;
      timeout: ReturnType<typeof setTimeout>;
    }
  >();

  constructor(private readonly opts: { allowedParentOrigins: string[]; targetOrigin?: string }) {
    this.onMessage = this.onMessage.bind(this);
    window.addEventListener('message', this.onMessage);

    for (const origin of opts.targetOrigin ? [opts.targetOrigin] : opts.allowedParentOrigins) {
      window.parent.postMessage(
        { target: 'sidiora:eip1193', type: 'ready' } satisfies PostMessageRpcReady,
        origin,
      );
    }
  }

  destroy() {
    if (this.destroyed) return;
    this.destroyed = true;
    window.removeEventListener('message', this.onMessage);
    for (const item of this.inflight.values()) {
      clearTimeout(item.timeout);
      item.reject(disconnectedError());
    }
    this.inflight.clear();
    this.handlers.clear();
    const host = window as Window & { ethereum?: Eip1193Provider };
    if (host.ethereum === this) delete host.ethereum;
  }

  on(event: string, handler: Handler) {
    const set = this.handlers.get(event) || new Set<Handler>();
    set.add(handler);
    this.handlers.set(event, set);
  }

  removeListener(event: string, handler: Handler) {
    const set = this.handlers.get(event);
    if (!set) return;
    set.delete(handler);
    if (set.size === 0) this.handlers.delete(event);
  }

  assertConfiguration(allowedParentOrigins: string[], targetOrigin?: string) {
    if (
      targetOrigin !== this.opts.targetOrigin ||
      allowedParentOrigins.length !== this.opts.allowedParentOrigins.length ||
      allowedParentOrigins.some((origin) => !this.opts.allowedParentOrigins.includes(origin))
    ) {
      throw new Error('Wallet parent configuration conflicts with the installed provider');
    }
  }

  private emitLocal(event: string, ...args: any[]) {
    const set = this.handlers.get(event);
    if (!set) return;
    for (const fn of set) {
      try {
        fn(...args);
      } catch {
        // ignore listener errors
      }
    }
  }

  async request(args: JsonRpcRequest): Promise<unknown> {
    if (this.destroyed || !this.parentOrigin) throw disconnectedError();
    if (!window.parent || window.parent === window) {
      throw new Error('postMessage provider requires being inside an iframe');
    }

    const id = randomId();
    const msg: PostMessageRpcRequest = {
      target: 'sidiora:eip1193',
      type: 'request',
      id,
      request: args,
    };

    return await new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        this.inflight.delete(id);
        reject(new Error(`postMessage request timeout: ${args.method}`));
      }, 60_000);

      this.inflight.set(id, { resolve, reject, timeout });

      try {
        window.parent.postMessage(msg, this.parentOrigin!);
      } catch (e) {
        clearTimeout(timeout);
        this.inflight.delete(id);
        reject(e);
      }
    });
  }

  private onMessage(event: MessageEvent) {
    if (
      this.destroyed ||
      event.source !== window.parent ||
      !this.opts.allowedParentOrigins.includes(event.origin)
    )
      return;
    if (this.opts.targetOrigin && event.origin !== this.opts.targetOrigin) return;
    if (!isParentMessage(event.data)) return;
    if (event.data.type === 'ready') {
      if (this.parentOrigin) return;
      this.parentOrigin = event.origin;
      this.emitLocal('ready');
      return;
    }
    if (event.origin !== this.parentOrigin) return;

    if (event.data.type === 'response') {
      const pending = this.inflight.get(event.data.id);
      if (!pending) return;

      clearTimeout(pending.timeout);
      this.inflight.delete(event.data.id);

      if (event.data.error) {
        const err = new Error(event.data.error.message);
        (err as any).code = event.data.error.code;
        (err as any).data = event.data.error.data;
        pending.reject(err);
        return;
      }

      pending.resolve(event.data.result);
      return;
    }

    if (event.data.type === 'event') {
      this.emitLocal(event.data.event, event.data.params);
      return;
    }
  }
}

export function installPostMessageEip1193Provider(options?: InstallPostMessageEip1193Options) {
  if (typeof window === 'undefined') return null;

  if (window.parent === window) return null;

  // Trusted parents are NEXT_PUBLIC_WALLET_IFRAME_ORIGIN through the shared
  // resolver, the same list the CSP frame-ancestors allows (src/middleware.ts).
  // With none configured the app cannot be framed by a wallet, and an empty
  // list here would trust any parent, so the bridge is not installed.
  const allowedParentOrigins = [
    ...new Set(
      (options?.allowedParentOrigins ?? walletParentOrigins()).map(checkedWalletParentOrigin),
    ),
  ];
  if (allowedParentOrigins.length === 0) return null;

  const targetOrigin =
    options?.targetOrigin === undefined
      ? undefined
      : checkedWalletParentOrigin(options.targetOrigin);
  if (targetOrigin && !allowedParentOrigins.includes(targetOrigin))
    throw new Error('Wallet target origin is not an admitted parent');

  const existing = (window as any).ethereum as Eip1193Provider | undefined;
  if (existing && existing.isSidioraPostMessage) {
    const provider = existing as PostMessageEip1193Provider;
    provider.assertConfiguration(allowedParentOrigins, targetOrigin);
    return provider;
  }
  if (existing && !options?.force) return null;

  const provider = new PostMessageEip1193Provider({
    allowedParentOrigins,
    targetOrigin,
  });

  (window as any).ethereum = provider;

  try {
    window.dispatchEvent(new Event('ethereum#initialized'));
  } catch {
    // ignore
  }

  return provider;
}
