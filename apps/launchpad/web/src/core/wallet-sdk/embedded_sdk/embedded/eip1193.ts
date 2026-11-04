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

import type {
  PaxeerWallet,
  SigningSession,
  TypedDataPayload,
  TxRequest,
} from "./sdk";
import type { Session } from "@supabase/supabase-js";
import {
  getAddress as toChecksum,
  hashTypedData,
  parseTransaction,
  recoverMessageAddress,
  recoverTransactionAddress,
  recoverTypedDataAddress,
  type Hex,
  type TypedDataDefinition,
  type TransactionSerialized,
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
  let principalEpoch = 0;
  let destroyed = false;

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

  function refusal(code: number, message: string): Error & { code: number } {
    return Object.assign(new Error(message), { code });
  }

  function requireAccount(value: unknown, account: Hex): void {
    if (typeof value !== "string" || !/^0x[0-9a-fA-F]{40}$/.test(value)) {
      throw refusal(-32602, "a valid account address is required");
    }
    if (value.toLowerCase() !== account.toLowerCase()) {
      throw refusal(
        4100,
        "the requested signer is not the authenticated wallet",
      );
    }
  }

  function requireChain(value: unknown): void {
    if (
      (typeof value !== "number" &&
        typeof value !== "string" &&
        typeof value !== "bigint") ||
      (typeof value === "number" && !Number.isSafeInteger(value)) ||
      (typeof value === "string" && !/^(?:[0-9]+|0x[0-9a-fA-F]+)$/.test(value))
    ) {
      throw refusal(-32602, "a valid chainId is required");
    }
    if (BigInt(value) !== BigInt(chainId))
      throw refusal(4901, "the requested chain is not connected");
  }

  async function signerContext(): Promise<{
    address: Hex;
    binding: SigningSession;
  }> {
    const epoch = principalEpoch;
    const assertCurrent = (): void => {
      if (destroyed || principalEpoch !== epoch)
        throw refusal(4100, "the wallet session changed");
    };
    const session = await wallet.getSession();
    assertCurrent();
    if (!session?.access_token)
      throw refusal(4100, "the wallet is not authenticated");
    const binding: SigningSession = {
      userId: session.user.id,
      accessToken: session.access_token,
      assertCurrent,
    };
    const result = await wallet.getWallet(binding);
    assertCurrent();
    requireChain(result.wallet.chain_id);
    requireChain(result.chain.id);
    return { address: toChecksum(result.wallet.address), binding };
  }

  async function getAddress(): Promise<Hex | null> {
    try {
      const { address } = await signerContext();
      cachedAddress = address;
      return address;
    } catch {
      return null;
    }
  }

  function checkResponse(
    response: { address: Hex; chain_id?: number },
    account: Hex,
    transaction = false,
  ): void {
    requireAccount(response.address, account);
    if (transaction) requireChain(response.chain_id);
  }

  function signatureField(value: unknown): Hex {
    if (typeof value !== "string" || !/^0x[0-9a-fA-F]{130}$/.test(value)) {
      throw refusal(-32603, "the gateway returned an invalid signature");
    }
    return value as Hex;
  }

  function transactionRequest(args: unknown[], address: Hex): TxRequest {
    const tx = args[0];
    if (!isRecord(tx))
      throw refusal(-32602, "a transaction object is required");
    if (tx.from !== undefined) requireAccount(tx.from, address);
    if (tx.chainId !== undefined) requireChain(tx.chainId);
    return {
      to: tx.to as Hex | undefined,
      value: toBigIntOrUndef(tx.value),
      data: tx.data as Hex | undefined,
      gas: toBigIntOrUndef(tx.gas ?? tx.gasLimit),
      maxFeePerGas: toBigIntOrUndef(tx.maxFeePerGas),
      maxPriorityFeePerGas: toBigIntOrUndef(tx.maxPriorityFeePerGas),
      nonce: toIntOrUndef(tx.nonce),
      chainId,
    };
  }

  function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === "object" && value !== null && !Array.isArray(value);
  }

  function typedDataRequest(raw: unknown): TypedDataPayload {
    let parsed: unknown = raw;
    if (typeof raw === "string") {
      try {
        parsed = JSON.parse(raw);
      } catch {
        throw refusal(-32602, "typed data must be valid JSON");
      }
    }
    if (
      !isRecord(parsed) ||
      !isRecord(parsed.types) ||
      !isRecord(parsed.message) ||
      (parsed.domain !== undefined && !isRecord(parsed.domain)) ||
      typeof parsed.primaryType !== "string" ||
      !parsed.primaryType
    ) {
      throw refusal(
        -32602,
        "typed data requires domain, types, primaryType and message",
      );
    }
    const domain = parsed.domain ?? {};
    if (isRecord(domain) && domain.chainId !== undefined)
      requireChain(domain.chainId);
    for (const fields of Object.values(parsed.types)) {
      if (
        !Array.isArray(fields) ||
        fields.some(
          (field) =>
            !isRecord(field) ||
            typeof field.name !== "string" ||
            !field.name ||
            typeof field.type !== "string" ||
            !field.type,
        )
      ) {
        throw refusal(-32602, "typed data fields require names and types");
      }
    }
    const document = {
      domain,
      types: parsed.types,
      primaryType: parsed.primaryType,
      message: parsed.message,
    } as TypedDataPayload;
    try {
      hashTypedData(document as unknown as TypedDataDefinition);
    } catch {
      throw refusal(-32602, "typed data cannot be encoded");
    }
    return document;
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
      body: JSON.stringify({
        jsonrpc: "2.0",
        id,
        method,
        params: params ?? [],
      }),
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
      throw refusal(-32602, "personal_sign requires a message string");
    if (!raw.startsWith("0x")) {
      if (
        new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(
          new TextEncoder().encode(raw),
        ) !== raw
      ) {
        throw refusal(-32602, "personal_sign text must be valid UTF-8");
      }
      return raw;
    }
    if (!/^0x(?:[0-9a-fA-F]{2})*$/.test(raw))
      throw refusal(-32602, "personal_sign requires complete hex byte pairs");
    const bytes = new Uint8Array((raw.length - 2) / 2);
    for (let i = 0; i < bytes.length; i++)
      bytes[i] = Number.parseInt(raw.slice(2 + i * 2, 4 + i * 2), 16);
    try {
      return new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(
        bytes,
      );
    } catch {
      throw refusal(-32602, "personal_sign bytes must be UTF-8 text");
    }
  }

  // Track auth changes so wagmi gets accountsChanged / connect / disconnect.
  const unsubAuth = wallet.onAuthStateChange(
    (event, session: Session | null) => {
      principalEpoch += 1;
      const before = cachedAddress;
      cachedAddress = null;
      void (async () => {
        if (!session) {
          const had = before;
          cachedAddress = null;
          if (had) {
            emit("accountsChanged", []);
            emit("disconnect", { code: 4900, message: "user signed out" });
          }
          return;
        }
        // SIGNED_IN, TOKEN_REFRESHED, INITIAL_SESSION → make sure we have an address.
        const addr = await getAddress();
        if (addr && addr !== before) {
          emit("accountsChanged", [addr]);
          if (!before) {
            emit("connect", { chainId: chainIdHex });
          }
        }
        void event; // event unused, but kept in signature for clarity
      })();
    },
  );

  return {
    isPaxeerEmbedded: true as const,

    async request({ method, params }: RequestArgs): Promise<unknown> {
      let args = asArray(params);
      if (
        [
          "eth_sendTransaction",
          "eth_signTransaction",
          "personal_sign",
          "eth_signTypedData_v4",
        ].includes(method)
      ) {
        try {
          args = structuredClone(args);
        } catch {
          throw refusal(-32602, "signing parameters must be immutable data");
        }
      }

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
          const { address, binding } = await signerContext();
          const tx = transactionRequest(args, address);
          const response = await wallet.sendTransaction(tx, binding);
          binding.assertCurrent();
          checkResponse(response, address, true);
          if (!/^0x[0-9a-fA-F]{64}$/.test(response.tx_hash))
            throw refusal(
              -32603,
              "the gateway returned an invalid transaction hash",
            );
          return response.tx_hash;
        }

        case "eth_signTransaction": {
          const { address, binding } = await signerContext();
          const tx = transactionRequest(args, address);
          const response = await wallet.signTransaction(tx, binding);
          binding.assertCurrent();
          checkResponse(response, address, true);
          const signed = parseTransaction(response.signed_tx);
          requireChain(signed.chainId);
          requireAccount(
            await recoverTransactionAddress({
              serializedTransaction:
                response.signed_tx as TransactionSerialized,
            }),
            address,
          );
          binding.assertCurrent();
          return response.signed_tx;
        }

        case "personal_sign": {
          const message = decodeMessage(args[0]);
          const { address, binding } = await signerContext();
          requireAccount(args[1], address);
          const response = await wallet.signMessage(message, binding);
          binding.assertCurrent();
          checkResponse(response, address);
          const signature = signatureField(response.signature);
          requireAccount(
            await recoverMessageAddress({ message, signature }),
            address,
          );
          binding.assertCurrent();
          return signature;
        }

        case "eth_sign":
          throw refusal(
            4200,
            "eth_sign requires the maintained SDK construction/digest custody path, which this connector has not admitted",
          );

        case "eth_signTypedData_v4": {
          const typedData = typedDataRequest(args[1]);
          const { address, binding } = await signerContext();
          requireAccount(args[0], address);
          const response = await wallet.signTypedData(typedData, binding);
          binding.assertCurrent();
          checkResponse(response, address);
          const signature = signatureField(response.signature);
          requireAccount(
            await recoverTypedDataAddress({
              ...typedData,
              signature,
            } as unknown as TypedDataDefinition & { signature: Hex }),
            address,
          );
          binding.assertCurrent();
          return signature;
        }

        case "eth_signTypedData_v3":
        case "eth_signTypedData":
          throw refusal(4200, "only eth_signTypedData_v4 is supported");

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
      principalEpoch += 1;
      unsubAuth();
      listeners.clear();
      cachedAddress = null;
    },
  };
}
