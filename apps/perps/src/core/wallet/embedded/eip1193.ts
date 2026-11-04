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

type RequestArgs = { method: string; params?: unknown[] | Record<string, unknown> };
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
}

export function createPaxeerEIP1193Provider(
  opts: PaxeerProviderOptions,
): PaxeerEIP1193Provider {
  const { wallet, chainId, rpcUrl } = opts;
  const fetchImpl = opts.fetchImpl ?? globalThis.fetch.bind(globalThis);
  const chainIdHex = `0x${chainId.toString(16)}`;

  const listeners = new Map<EIP1193Event, Set<Listener>>();
  let cachedAddress: `0x${string}` | null = null;
  let rpcId = 0;
  let sessionUserId: string | null = null;
  let sessionToken: string | null = null;
  let authGeneration = 0;
  let destroyed = false;
  let addressLookup: { generation: number; promise: Promise<Hex | null> } | null = null;
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

  function providerError(code: number, message: string): Error & { code: number } {
    return Object.assign(new Error(message), { code });
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

  async function assertSession(session: Session, generation: number): Promise<void> {
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
          throw providerError(4901, "The wallet account is on a different chain");
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
      throw providerError(4100, "The requested signer is not the connected account");
    }
  }

  function requireTransactionConsent(tx: Record<string, unknown>, address: Hex): void {
    if (tx.from !== undefined) requireAccount(tx.from, address);
    if (tx.chainId !== undefined) {
      const value = tx.chainId;
      if (
        !(
          typeof value === "bigint" ||
          (typeof value === "number" && Number.isSafeInteger(value)) ||
          (typeof value === "string" && /^(?:0x[0-9a-fA-F]+|[0-9]+)$/.test(value))
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

  function requireResponse(response: { address: Hex; chain_id: number }, address: Hex): void {
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
  function toBigIntOrUndef(v: unknown): bigint | undefined {
    if (v === undefined || v === null || v === "") return undefined;
    if (typeof v === "bigint") return v;
    if (typeof v === "number") return BigInt(v);
    if (typeof v === "string") {
      // BigInt() accepts both "0x.." and decimal strings.
      try {
        return BigInt(v);
      } catch {
        return undefined;
      }
    }
    return undefined;
  }

  function toIntOrUndef(v: unknown): number | undefined {
    if (v === undefined || v === null || v === "") return undefined;
    if (typeof v === "number") return v;
    if (typeof v === "bigint") return Number(v);
    if (typeof v === "string") {
      try {
        return Number(BigInt(v));
      } catch {
        return undefined;
      }
    }
    return undefined;
  }

  async function rpcCall(method: string, params: unknown): Promise<unknown> {
    const id = ++rpcId;
    const res = await fetchImpl(rpcUrl, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ jsonrpc: "2.0", id, method, params: params ?? [] }),
    });
    const json = (await res.json()) as {
      result?: unknown;
      error?: { code: number; message: string };
    };
    if (json.error) {
      const err = new Error(json.error.message) as Error & { code?: number };
      err.code = json.error.code;
      throw err;
    }
    return json.result;
  }

  function asArray(params: RequestArgs["params"]): unknown[] {
    return Array.isArray(params) ? params : params ? [params] : [];
  }

  function decodeMessage(raw: unknown): string {
    if (typeof raw !== "string")
      throw providerError(-32602, "personal_sign requires a message");
    if (!raw.startsWith("0x")) return raw;
    if (!/^0x(?:[0-9a-fA-F]{2})*$/.test(raw)) {
      throw providerError(-32602, "personal_sign requires complete even-length hex");
    }
    const buf = new Uint8Array((raw.length - 2) / 2);
    for (let i = 0; i < buf.length; i++) buf[i] = parseInt(raw.slice(2 + i * 2, 4 + i * 2), 16);
    try {
      return new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(buf);
    } catch {
      throw providerError(-32602, "personal_sign bytes must be UTF-8 text");
    }
  }

  // Supabase holds its auth lock while calling subscribers; read the session after it releases it.
  const unsubAuth = wallet.onAuthStateChange((_event, session: Session | null) => {
    if (destroyed) return;
    observeSession(session);
    clearTimeout(authTimer);
    if (session)
      authTimer = setTimeout(() => {
        void getAddress();
      }, 0);
  });

  return {
    isPaxeerEmbedded: true as const,

    async request({ method, params }: RequestArgs): Promise<unknown> {
      if (destroyed) throw providerError(4900, "The wallet provider was destroyed");
      const args = [
        "eth_sendTransaction",
        "eth_signTransaction",
        "personal_sign",
        "eth_sign",
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
              value: toBigIntOrUndef(tx.value),
              data: tx.data as `0x${string}` | undefined,
              gas: toBigIntOrUndef(tx.gas ?? tx.gasLimit),
              maxFeePerGas: toBigIntOrUndef(tx.maxFeePerGas),
              maxPriorityFeePerGas: toBigIntOrUndef(tx.maxPriorityFeePerGas),
              nonce: toIntOrUndef(tx.nonce),
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
              value: toBigIntOrUndef(tx.value),
              data: tx.data as `0x${string}` | undefined,
              gas: toBigIntOrUndef(tx.gas ?? tx.gasLimit),
              maxFeePerGas: toBigIntOrUndef(tx.maxFeePerGas),
              maxPriorityFeePerGas: toBigIntOrUndef(tx.maxPriorityFeePerGas),
              nonce: toIntOrUndef(tx.nonce),
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
            await recoverTransactionAddress({ serializedTransaction: serialized }),
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

        case "eth_sign": {
          const { session, generation, address } = await signingSession();
          requireAccount(args[0], address);
          const canonical = wallet.createProvider({ chainId, rpcUrl }, session, () =>
            requireGeneration(generation),
          );
          try {
            const accounts = (await canonical.request({
              method: "eth_requestAccounts",
            })) as Hex[];
            requireAccount(accounts[0], address);
            if ((await canonical.request({ method: "eth_chainId" })) !== chainIdHex) {
              throw providerError(4901, "The digest signing chain changed");
            }
            await assertSession(session, generation);
            const signature = await canonical.request({ method: "eth_sign", params: args });
            await assertSession(session, generation);
            return signature;
          } catch (error) {
            await assertSession(session, generation);
            throw error;
          } finally {
            canonical.disconnect();
          }
        }

        case "eth_signTypedData_v4":
        case "eth_signTypedData_v3":
        case "eth_signTypedData": {
          const err = new Error(
            "eth_signTypedData_v4 is not yet supported by the Paxeer Embedded Wallet API. Use an injected wallet for EIP-712 signing for now.",
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
                  caveats: [{ type: "restrictReturnedAccounts", value: [addr] }],
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
      unsubAuth();
      listeners.clear();
      cachedAddress = null;
    },
  };
}
