/**
 * EIP-1193 bridge for the Paxeer Embedded Wallet.
 *
 * The new `@paxeer/wallet` SDK is a REST client over `connect.paxportwallet.com`,
 * not a wallet provider. wagmi v2 connectors expect an EIP-1193 provider, so
 * this module synthesizes one:
 *
 *   wallet methods   (eth_sendTransaction, personal_sign, …)
 *     → forwarded to the SDK (server signs + broadcasts)
 *
 *   read methods     (eth_call, eth_getBalance, eth_blockNumber, …)
 *     → forwarded to the public Paxeer RPC over fetch
 *
 *   chain management (eth_chainId, wallet_switchEthereumChain)
 *     → handled locally; we are single-chain (125) for now
 *
 *   auth state changes
 *     → emitted as `accountsChanged` / `connect` / `disconnect` events
 *
 * Supabase OAuth flows redirect away from the page, so the bridge must
 * survive a full reload — every method re-reads the session from the SDK.
 */

import type { PaxeerWallet } from "./sdk";
import type { Session } from "@supabase/supabase-js";
import {
  getAddress as toChecksum,
  parseTransaction,
  recoverMessageAddress,
  recoverTransactionAddress,
  type Hex,
} from "viem";

type RequestArgs = {
  method: string;
  params?: unknown[] | Record<string, unknown>;
};
type Listener = (...args: unknown[]) => void;

export type EIP1193Event =
  | "connect"
  | "disconnect"
  | "accountsChanged"
  | "chainChanged"
  | "message";

export interface PaxeerEIP1193Provider {
  readonly isPaxeerEmbedded: true;
  request(args: RequestArgs): Promise<unknown>;
  on(event: string, listener: Listener): void;
  removeListener(event: string, listener: Listener): void;
  /** Tear down listeners + auth subscription. Called by the connector on disconnect. */
  destroy(): void;
}

export interface PaxeerProviderOptions {
  wallet: PaxeerWallet;
  chainId: number;
  rpcUrl: string;
  fetchImpl?: typeof fetch;
  /** Read deadline, including the response body (10 seconds by default). */
  rpcTimeoutMs?: number;
}

export function createPaxeerEIP1193Provider(
  opts: PaxeerProviderOptions,
): PaxeerEIP1193Provider {
  const { wallet, chainId, rpcUrl } = opts;
  const fetchImpl = opts.fetchImpl ?? globalThis.fetch.bind(globalThis);
  const chainIdHex = `0x${chainId.toString(16)}`;
  const rpcTimeoutMs = opts.rpcTimeoutMs ?? 10_000;
  if (
    !Number.isSafeInteger(rpcTimeoutMs) ||
    rpcTimeoutMs <= 0 ||
    rpcTimeoutMs > 10_000
  )
    throw new Error("rpcTimeoutMs must be an integer between 1 and 10000");
  const rpcRequests = new Set<AbortController>();

  const listeners = new Map<EIP1193Event, Set<Listener>>();
  let cachedAddress: `0x${string}` | null = null;
  let rpcId = 0;
  let sessionUserId: string | null = null;
  let sessionToken: string | null = null;
  let authGeneration = 0;
  let destroyed = false;
  let addressLookup: {
    generation: number;
    promise: Promise<Hex | null>;
  } | null = null;
  let authTimer: ReturnType<typeof setTimeout> | undefined;

  function emit(event: EIP1193Event, ...args: unknown[]): void {
    const set = listeners.get(event);
    if (!set) return;
    for (const listener of set) {
      try {
        listener(...args);
      } catch {
        /* swallow — wagmi listeners must not break our auth flow */
      }
    }
  }

  function providerError(
    code: number,
    message: string,
    data?: unknown,
  ): Error & { code: number; data?: unknown } {
    return Object.assign(new Error(message), {
      code,
      ...(data === undefined ? {} : { data }),
    });
  }

  function observeSession(session: Session | null): void {
    const userId = session?.user.id ?? null;
    const token = session?.access_token ?? null;
    if (userId === sessionUserId && token === sessionToken) return;
    authGeneration += 1;
    addressLookup = null;
    if (userId !== sessionUserId) {
      const hadAddress = cachedAddress !== null;
      cachedAddress = null;
      if (hadAddress) emit("accountsChanged", []);
      if (hadAddress && !session)
        emit("disconnect", { code: 4900, message: "user signed out" });
    }
    sessionUserId = userId;
    sessionToken = token;
  }

  function requireGeneration(generation: number): void {
    if (destroyed || generation !== authGeneration)
      throw providerError(4100, "The wallet session changed");
  }

  async function assertSession(
    session: Session,
    generation: number,
  ): Promise<void> {
    const current = await wallet.getSession();
    if (
      destroyed ||
      generation !== authGeneration ||
      current?.user.id !== session.user.id ||
      current?.access_token !== session.access_token
    ) {
      if (!destroyed && generation === authGeneration) observeSession(current);
      throw providerError(4100, "The wallet session changed");
    }
  }

  async function getAddress(): Promise<Hex | null> {
    const beforeRead = authGeneration;
    const session = await wallet.getSession();
    if (destroyed || beforeRead !== authGeneration) return null;
    observeSession(session);
    if (!session) return null;
    if (cachedAddress) return cachedAddress;
    const generation = authGeneration;
    if (addressLookup?.generation === generation) return addressLookup.promise;
    const promise = (async (): Promise<Hex | null> => {
      try {
        const r = await wallet.getWallet(session);
        await assertSession(session, generation);
        if (r.wallet.chain_id !== chainId || r.chain.id !== chainId) {
          throw providerError(
            4901,
            "The wallet account is on a different chain",
          );
        }
        const address = toChecksum(r.wallet.address);
        if (cachedAddress !== address) {
          const hadAddress = cachedAddress !== null;
          cachedAddress = address;
          emit("accountsChanged", [address]);
          if (!hadAddress) emit("connect", { chainId: chainIdHex });
        }
        return address;
      } catch {
        return null;
      } finally {
        if (addressLookup?.generation === generation) addressLookup = null;
      }
    })();
    addressLookup = { generation, promise };
    return promise;
  }

  async function signingSession(): Promise<{
    session: Session;
    generation: number;
    address: Hex;
  }> {
    const beforeRead = authGeneration;
    const session = await wallet.getSession();
    if (destroyed || beforeRead !== authGeneration || !session) {
      throw providerError(4100, "No stable signed-in wallet session");
    }
    observeSession(session);
    const generation = authGeneration;
    const address = await getAddress();
    await assertSession(session, generation);
    if (!address) throw providerError(4100, "No connected wallet account");
    return { session, generation, address };
  }

  function requireAccount(value: unknown, address: Hex): void {
    if (typeof value !== "string" || !/^0x[0-9a-fA-F]{40}$/.test(value)) {
      throw providerError(-32602, "A valid signing address is required");
    }
    if (value.toLowerCase() !== address.toLowerCase()) {
      throw providerError(
        4100,
        "The requested signer is not the connected account",
      );
    }
  }

  function requireTransactionConsent(
    tx: Record<string, unknown>,
    address: Hex,
  ): void {
    if (tx.from !== undefined) requireAccount(tx.from, address);
    if (tx.chainId !== undefined) {
      const value = tx.chainId;
      if (
        !(
          typeof value === "bigint" ||
          (typeof value === "number" && Number.isSafeInteger(value)) ||
          (typeof value === "string" &&
            /^(?:0x[0-9a-fA-F]+|[0-9]+)$/.test(value))
        )
      ) {
        throw providerError(-32602, "Invalid transaction chainId");
      }
      if (BigInt(value) !== BigInt(chainId)) {
        throw providerError(
          4901,
          "The requested transaction chain differs from the connected chain",
        );
      }
    }
  }

  function requireResponse(
    response: { address: Hex; chain_id: number },
    address: Hex,
  ): void {
    requireAccount(response.address, address);
    if (response.chain_id !== chainId)
      throw providerError(4901, "Signing response chain mismatch");
  }

  /**
   * viem/wagmi serialize transaction quantity fields as hex (`0x2dc6c0`).
   * The SDK's `TxRequest` expects either a decimal string or a bigint
   * ("Decimal string in wei"). Sending hex straight through gets rejected
   * by `connect.paxportwallet.com` with a 400. Normalize every quantity
   * field to bigint before forwarding.
   */
  function toBigIntOrUndef(v: unknown, field: string): bigint | undefined {
    if (v === undefined) return undefined;
    if (
      !(
        typeof v === "bigint" ||
        (typeof v === "number" && Number.isSafeInteger(v)) ||
        (typeof v === "string" && /^(?:0x[0-9a-fA-F]+|0|[1-9][0-9]*)$/.test(v))
      )
    )
      throw providerError(
        -32602,
        `Invalid transaction ${field}: expected an unsigned integer`,
        { field },
      );
    const value = BigInt(v);
    if (value < 0n || value > (1n << 256n) - 1n)
      throw providerError(
        -32602,
        `Invalid transaction ${field}: out of range`,
        { field },
      );
    return value;
  }

  function toIntOrUndef(v: unknown, field: string): number | undefined {
    const value = toBigIntOrUndef(v, field);
    if (value === undefined) return undefined;
    if (value > BigInt(Number.MAX_SAFE_INTEGER))
      throw providerError(
        -32602,
        `Invalid transaction ${field}: exceeds the safe integer range`,
        { field },
      );
    return Number(value);
  }

  function transactionQuantities(tx: Record<string, unknown>) {
    const gas = toBigIntOrUndef(tx.gas, "gas");
    const gasLimit = toBigIntOrUndef(tx.gasLimit, "gasLimit");
    return {
      value: toBigIntOrUndef(tx.value, "value"),
      gas: gas ?? gasLimit,
      maxFeePerGas: toBigIntOrUndef(tx.maxFeePerGas, "maxFeePerGas"),
      maxPriorityFeePerGas: toBigIntOrUndef(
        tx.maxPriorityFeePerGas,
        "maxPriorityFeePerGas",
      ),
      nonce: toIntOrUndef(tx.nonce, "nonce"),
    };
  }

  async function rpcCall(method: string, params: unknown): Promise<unknown> {
    const id = ++rpcId;
    const controller = new AbortController();
    rpcRequests.add(controller);
    const timer = setTimeout(() => {
      controller.abort(
        providerError(
          4900,
          `Wallet RPC request timed out after ${rpcTimeoutMs}ms`,
        ),
      );
    }, rpcTimeoutMs);
    try {
      let res: Response;
      try {
        res = await fetchImpl(rpcUrl, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            jsonrpc: "2.0",
            id,
            method,
            params: params ?? [],
          }),
          signal: controller.signal,
        });
      } catch (cause) {
        throw providerError(4900, "Wallet RPC request failed", { cause });
      }
      if (!res.ok) {
        await res.body?.cancel();
        throw Object.assign(
          providerError(4900, `Wallet RPC HTTP ${res.status}`),
          { status: res.status },
        );
      }
      let json: unknown;
      try {
        json = await res.json();
      } catch {
        throw providerError(-32603, "Invalid JSON in wallet RPC response");
      }
      if (
        !isRecord(json) ||
        json.jsonrpc !== "2.0" ||
        json.id !== id ||
        Object.hasOwn(json, "result") === Object.hasOwn(json, "error")
      )
        throw providerError(-32603, "Invalid wallet RPC response envelope");
      if (Object.hasOwn(json, "error")) {
        const error = json.error;
        if (
          !isRecord(error) ||
          typeof error.code !== "number" ||
          !Number.isSafeInteger(error.code) ||
          typeof error.message !== "string"
        )
          throw providerError(-32603, "Invalid wallet RPC error envelope");
        throw providerError(error.code, error.message, error.data);
      }
      return json.result;
    } catch (error) {
      if (controller.signal.aborted) throw controller.signal.reason;
      throw error;
    } finally {
      clearTimeout(timer);
      rpcRequests.delete(controller);
    }
  }

  function isRecord(value: unknown): value is Record<string, unknown> {
    return value !== null && typeof value === "object" && !Array.isArray(value);
  }

  function asArray(params: RequestArgs["params"]): unknown[] {
    return Array.isArray(params) ? params : params ? [params] : [];
  }

  function decodeMessage(raw: unknown): string {
    if (typeof raw !== "string")
      throw providerError(-32602, "personal_sign requires a message");
    if (!raw.startsWith("0x")) return raw;
    if (!/^0x(?:[0-9a-fA-F]{2})*$/.test(raw)) {
      throw providerError(
        -32602,
        "personal_sign requires complete even-length hex",
      );
    }
    const buf = new Uint8Array((raw.length - 2) / 2);
    for (let i = 0; i < buf.length; i++)
      buf[i] = parseInt(raw.slice(2 + i * 2, 4 + i * 2), 16);
    try {
      return new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(
        buf,
      );
    } catch {
      throw providerError(-32602, "personal_sign bytes must be UTF-8 text");
    }
  }

  // Supabase holds its auth lock while calling subscribers; read the session after it releases it.
  const unsubAuth = wallet.onAuthStateChange(
    (_event, session: Session | null) => {
      if (destroyed) return;
      observeSession(session);
      clearTimeout(authTimer);
      if (session)
        authTimer = setTimeout(() => {
          void getAddress();
        }, 0);
    },
  );

  return {
    isPaxeerEmbedded: true as const,

    async request({ method, params }: RequestArgs): Promise<unknown> {
      if (destroyed)
        throw providerError(4900, "The wallet provider was destroyed");
      const args = [
        "eth_sendTransaction",
        "eth_signTransaction",
        "personal_sign",
        "eth_sign",
        "eth_signTypedData_v4",
      ].includes(method)
        ? structuredClone(asArray(params))
        : asArray(params);

      switch (method) {
        case "eth_chainId":
        case "net_version":
          return method === "net_version" ? String(chainId) : chainIdHex;

        case "eth_accounts": {
          const addr = await getAddress();
          return addr ? [addr] : [];
        }

        case "eth_requestAccounts": {
          const addr = await getAddress();
          if (!addr) {
            const err = new Error(
              "User is not signed in to the Paxeer Embedded Wallet. Open the connect modal first.",
            ) as Error & { code?: number };
            err.code = 4100;
            throw err;
          }
          return [addr];
        }

        case "eth_sendTransaction": {
          const tx = (args[0] ?? {}) as Record<string, unknown>;
          const { session, generation, address } = await signingSession();
          requireTransactionConsent(tx, address);
          await assertSession(session, generation);
          const r = await wallet.sendTransaction(
            {
              to: tx.to as `0x${string}` | undefined,
              data: tx.data as `0x${string}` | undefined,
              ...transactionQuantities(tx),
              chainId,
            },
            session,
            () => requireGeneration(generation),
          );
          await assertSession(session, generation);
          requireResponse(r, address);
          if (!/^0x[0-9a-fA-F]{64}$/.test(r.tx_hash))
            throw providerError(-32603, "Invalid transaction hash");
          return r.tx_hash;
        }

        case "eth_signTransaction": {
          const tx = (args[0] ?? {}) as Record<string, unknown>;
          const { session, generation, address } = await signingSession();
          requireTransactionConsent(tx, address);
          await assertSession(session, generation);
          const r = await wallet.signTransaction(
            {
              to: tx.to as `0x${string}` | undefined,
              data: tx.data as `0x${string}` | undefined,
              ...transactionQuantities(tx),
              chainId,
            },
            session,
            () => requireGeneration(generation),
          );
          await assertSession(session, generation);
          requireResponse(r, address);
          if (!/^0x02(?:[0-9a-fA-F]{2})+$/.test(r.signed_tx)) {
            throw providerError(
              -32603,
              "Signing response must contain an EIP-1559 transaction",
            );
          }
          const serialized = r.signed_tx as `0x02${string}`;
          const signed = parseTransaction(serialized);
          if (signed.chainId !== chainId)
            throw providerError(4901, "Signed transaction chain mismatch");
          requireAccount(
            await recoverTransactionAddress({
              serializedTransaction: serialized,
            }),
            address,
          );
          await assertSession(session, generation);
          return r.signed_tx;
        }

        case "personal_sign": {
          const message = decodeMessage(args[0]);
          const { session, generation, address } = await signingSession();
          requireAccount(args[1], address);
          await assertSession(session, generation);
          const r = await wallet.signMessage(message, session, () =>
            requireGeneration(generation),
          );
          await assertSession(session, generation);
          requireAccount(r.address, address);
          requireAccount(
            await recoverMessageAddress({ message, signature: r.signature }),
            address,
          );
          await assertSession(session, generation);
          return r.signature;
        }

        case "eth_sign":
        case "eth_signTypedData_v4": {
          const { session, generation, address } = await signingSession();
          requireAccount(args[0], address);
          const canonical = wallet.createProvider(
            { chainId, rpcUrl },
            session,
            () => requireGeneration(generation),
          );
          try {
            const accounts = (await canonical.request({
              method: "eth_requestAccounts",
            })) as Hex[];
            requireAccount(accounts[0], address);
            if (
              (await canonical.request({ method: "eth_chainId" })) !==
              chainIdHex
            ) {
              throw providerError(4901, "The signing chain changed");
            }
            await assertSession(session, generation);
            const signature = await canonical.request({ method, params: args });
            await assertSession(session, generation);
            return signature;
          } catch (error) {
            await assertSession(session, generation);
            if (
              method === "eth_signTypedData_v4" &&
              error instanceof Error &&
              "status" in error &&
              [404, 405, 501].includes(Number(error.status))
            ) {
              throw providerError(
                4200,
                "EIP-712 signing is unavailable on this wallet gateway deployment. Use a gateway with the canonical EIP-712 v4 signing route or an injected wallet.",
                { cause: error },
              );
            }
            throw error;
          } finally {
            canonical.disconnect();
          }
        }

        case "eth_signTypedData_v3":
        case "eth_signTypedData_v1":
        case "eth_signTypedData": {
          const err = new Error(
            `${method} is not supported. Use eth_signTypedData_v4 through the canonical wallet provider for EIP-712 signing.`,
          ) as Error & { code?: number };
          err.code = 4200; // Unsupported Method
          throw err;
        }

        case "wallet_switchEthereumChain": {
          const target = (args[0] as { chainId?: string } | undefined)?.chainId;
          if (target && target.toLowerCase() === chainIdHex) return null;
          const err = new Error(
            `Chain ${target ?? "?"} is not supported by the Paxeer Embedded Wallet (only ${chainIdHex} / 125).`,
          ) as Error & { code?: number };
          err.code = 4902; // Chain Not Added
          throw err;
        }

        case "wallet_addEthereumChain":
          // Single-chain wallet: nothing to add, succeed silently.
          return null;

        case "wallet_getPermissions":
        case "wallet_requestPermissions": {
          const addr = await getAddress();
          return addr
            ? [
                {
                  parentCapability: "eth_accounts",
                  caveats: [
                    { type: "restrictReturnedAccounts", value: [addr] },
                  ],
                },
              ]
            : [];
        }

        // Read methods → forward to public RPC.
        default:
          return rpcCall(method, args);
      }
    },

    on(event: string, listener: Listener): void {
      const key = event as EIP1193Event;
      const set = listeners.get(key) ?? new Set<Listener>();
      set.add(listener);
      listeners.set(key, set);
    },

    removeListener(event: string, listener: Listener): void {
      listeners.get(event as EIP1193Event)?.delete(listener);
    },

    destroy(): void {
      destroyed = true;
      authGeneration += 1;
      clearTimeout(authTimer);
      for (const controller of rpcRequests)
        controller.abort(
          providerError(4900, "The wallet provider was destroyed"),
        );
      rpcRequests.clear();
      unsubAuth();
      listeners.clear();
      cachedAddress = null;
    },
  };
}
