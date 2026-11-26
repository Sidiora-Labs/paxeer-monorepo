import { randomBytes } from "node:crypto";
import { ed25519 } from "@noble/curves/ed25519.js";
import { sha256 } from "@noble/hashes/sha2.js";
import { z } from "zod";
import type { Address, Hex } from "viem";
import { AgentKitError, jsonText } from "./result.js";

export const AGENT_REQUEST_DOMAIN = "PXW:AGENT-REQUEST:v1";
export const AGENT_ROUTES = {
  challenge: "/v1/agent/auth/challenge",
  verify: "/v1/agent/auth/verify",
  provision: "/v1/agent/provision",
  me: "/v1/agent/me",
  allowance: "/v1/agent/allowance",
  simulate: "/v1/agent/simulate",
  send: "/v1/agent/send",
  allowanceAndCall: "/v1/agent/actions/allowance-and-call",
} as const;

function u32(value: number): Uint8Array {
  const out = Buffer.alloc(4);
  out.writeUInt32BE(value);
  return out;
}

export interface AgentRequestDigestInput {
  method: string;
  keyId: string;
  nonce: Uint8Array;
  expiry: bigint;
  body: string | Uint8Array;
}

export function agentRequestDigest(input: AgentRequestDigestInput): Uint8Array {
  const method = Buffer.from(input.method);
  const key = Buffer.from(input.keyId);
  if (
    input.nonce.length !== 16 ||
    input.expiry < 0n ||
    input.expiry >= 1n << 64n
  ) {
    throw new AgentKitError(
      "INVALID_SIGNATURE_INPUT",
      "Nonce must be 16 bytes and expiry must be uint64",
    );
  }
  const expiry = Buffer.alloc(8);
  expiry.writeBigUInt64BE(input.expiry);
  return sha256(
    Buffer.concat([
      Buffer.from(AGENT_REQUEST_DOMAIN),
      u32(method.length),
      method,
      u32(key.length),
      key,
      input.nonce,
      expiry,
      sha256(
        typeof input.body === "string" ? Buffer.from(input.body) : input.body,
      ),
    ]),
  );
}

export interface SignAgentRequestInput {
  privateKey: Uint8Array;
  keyId: string;
  method: string;
  url: string;
  body?: string | Uint8Array;
  nonce?: Uint8Array;
  expiresAt?: bigint;
}

export function signAgentRequest(
  input: SignAgentRequestInput,
): Record<string, string> {
  if (input.privateKey.length !== 32 || !input.keyId) {
    throw new AgentKitError(
      "INVALID_AGENT_KEY",
      "A 32-byte Ed25519 seed and agent DID are required",
    );
  }
  const nonce = input.nonce ?? randomBytes(16);
  const expiry = input.expiresAt ?? BigInt(Math.floor(Date.now() / 1000) + 60);
  const path = /^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(input.url)
    ? new URL(input.url).pathname
    : input.url.split(/[?#]/, 1)[0];
  const digest = agentRequestDigest({
    method: `${input.method.toUpperCase()} ${path}`,
    keyId: input.keyId,
    nonce,
    expiry,
    body: input.body ?? "",
  });
  return {
    "x-agent-key": input.keyId,
    "x-agent-nonce": Buffer.from(nonce).toString("hex"),
    "x-agent-expires": expiry.toString(),
    "x-agent-signature": Buffer.from(
      ed25519.sign(digest, input.privateKey),
    ).toString("hex"),
  };
}

export function gatewayAgentDid(privateKey: Uint8Array, label: string): string {
  if (privateKey.length !== 32 || !/^[A-Za-z0-9_-]{1,128}$/.test(label)) {
    throw new AgentKitError(
      "INVALID_AGENT_KEY",
      "A 32-byte Ed25519 seed and valid agent label are required",
    );
  }
  return `did:matrix:${label}:${Buffer.from(ed25519.getPublicKey(privateKey)).toString("hex").slice(0, 16)}`;
}

export function httpUrl(value: string): string {
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    throw new AgentKitError("INVALID_URL", "An HTTP or HTTPS URL is required");
  }
  if (
    !["http:", "https:"].includes(url.protocol) ||
    url.username ||
    url.password ||
    url.search ||
    url.hash
  ) {
    throw new AgentKitError(
      "INVALID_URL",
      "An HTTP or HTTPS URL without credentials, query or fragment is required",
    );
  }
  return url.href.replace(/\/+$/, "");
}

const tokenSchema = z.object({
  token: z.string().min(1),
  expires_in: z.number().int().positive(),
});
const walletSchema = z.object({
  wallet: z
    .object({ address: z.string().regex(/^0x[0-9a-fA-F]{40}$/) })
    .nullable(),
});

export interface WireTransaction {
  to: Address;
  data: Hex;
  value: "0";
}
export interface AllowanceAndCall {
  token: Address;
  spender: Address;
  amount: string;
  contract: Address;
  method: string;
  args: string[];
  idempotency_key: string;
}
export interface GatewayOptions {
  url: string;
  privateKey: Uint8Array;
  label: string;
  fetch?: typeof fetch;
}

export class AgentGatewayClient {
  readonly did: string;
  readonly #privateKey: Uint8Array;
  readonly #fetch: typeof fetch;
  readonly #url: string;
  #session: { token: string; expires: number } | undefined;
  #authentication: Promise<void> | undefined;

  constructor(options: GatewayOptions) {
    this.#privateKey = Uint8Array.from(options.privateKey);
    this.did = gatewayAgentDid(this.#privateKey, options.label);
    this.#url = httpUrl(options.url);
    this.#fetch = options.fetch ?? globalThis.fetch;
  }

  async #request(
    method: "GET" | "POST",
    route: string,
    body?: unknown,
    signed = false,
    token?: string,
  ): Promise<unknown> {
    const wire = body === undefined ? "" : jsonText(body);
    const url = `${this.#url}${route}`;
    const res = await this.#fetch(url, {
      method,
      headers: {
        ...(body === undefined ? {} : { "content-type": "application/json" }),
        ...(token ? { authorization: `Bearer ${token}` } : {}),
        ...(signed
          ? signAgentRequest({
              privateKey: this.#privateKey,
              keyId: this.did,
              method,
              url,
              body: wire,
            })
          : {}),
      },
      ...(body === undefined ? {} : { body: wire }),
      signal: AbortSignal.timeout(30_000),
    });
    let payload: unknown;
    try {
      payload = await res.json();
    } catch {
      throw new AgentKitError(
        "GATEWAY_RESPONSE_INVALID",
        "Gateway did not return JSON",
        { status: res.status },
      );
    }
    const record = z
      .object({
        error: z.unknown().optional(),
        ok: z.unknown().optional(),
        message: z.unknown().optional(),
      })
      .passthrough()
      .safeParse(payload);
    if (!record.success)
      throw new AgentKitError(
        "GATEWAY_RESPONSE_INVALID",
        "Gateway returned an invalid response",
        { status: res.status },
      );
    const error = record.data.error;
    if (!res.ok || record.data.ok === false || error != null) {
      const envelope = z
        .object({ code: z.string(), message: z.string().optional() })
        .passthrough()
        .safeParse(error);
      const code =
        typeof error === "string"
          ? error
          : envelope.success
            ? envelope.data.code
            : route === AGENT_ROUTES.simulate && record.data.ok === false
              ? "SIMULATION_REVERTED"
              : "GATEWAY_REFUSED";
      const message =
        typeof record.data.message === "string"
          ? record.data.message
          : envelope.success
            ? (envelope.data.message ?? code)
            : code;
      throw new AgentKitError(code, message, {
        status: res.status,
        response: payload,
      });
    }
    return payload;
  }

  async authenticate(): Promise<void> {
    if (this.#session && this.#session.expires > Date.now() + 5_000) return;
    if (this.#authentication) return this.#authentication;
    this.#authentication = (async () => {
      const challenge = z
        .object({
          did: z.string(),
          nonce: z.string().min(8).max(128),
          message: z.string(),
        })
        .parse(
          await this.#request("POST", AGENT_ROUTES.challenge, {
            did: this.did,
          }),
        );
      const message = `paxeer-agent-auth:${this.did}:${challenge.nonce}`;
      if (challenge.did !== this.did || challenge.message !== message) {
        throw new AgentKitError(
          "INVALID_CHALLENGE",
          "Gateway challenge does not match the agent DID and nonce",
        );
      }
      const result = tokenSchema.parse(
        await this.#request("POST", AGENT_ROUTES.verify, {
          did: this.did,
          public_key: Buffer.from(
            ed25519.getPublicKey(this.#privateKey),
          ).toString("hex"),
          nonce: challenge.nonce,
          signature: Buffer.from(
            ed25519.sign(Buffer.from(message), this.#privateKey),
          ).toString("hex"),
        }),
      );
      this.#session = {
        token: result.token,
        expires: Date.now() + result.expires_in * 1000,
      };
    })();
    try {
      await this.#authentication;
    } finally {
      this.#authentication = undefined;
    }
  }

  async read(route: string): Promise<unknown> {
    await this.authenticate();
    return this.#request("GET", route, undefined, false, this.#session!.token);
  }

  async walletAddress(): Promise<Address> {
    const result = walletSchema.parse(await this.read(AGENT_ROUTES.me));
    if (!result.wallet)
      throw new AgentKitError(
        "NO_AGENT_WALLET",
        "Provision the agent wallet before trading",
      );
    return result.wallet.address as Address;
  }

  async provision(bindSignature?: string): Promise<unknown> {
    await this.authenticate();
    return this.#request(
      "POST",
      AGENT_ROUTES.provision,
      bindSignature ? { bind_signature: bindSignature } : {},
      true,
    );
  }

  async allowance(token: Address, spender: Address): Promise<bigint> {
    const value = z
      .object({ allowance: z.string().regex(/^\d+$/) })
      .parse(
        await this.read(
          `${AGENT_ROUTES.allowance}?${new URLSearchParams({ token, spender })}`,
        ),
      );
    return BigInt(value.allowance);
  }

  async simulate(tx: WireTransaction): Promise<unknown> {
    await this.authenticate();
    const payload = await this.#request(
      "POST",
      AGENT_ROUTES.simulate,
      { tx },
      false,
      this.#session!.token,
    );
    const result = z.object({ ok: z.boolean() }).passthrough().parse(payload);
    if (!result.ok)
      throw new AgentKitError(
        "SIMULATION_REVERTED",
        "The call simulation reverted",
        payload,
      );
    return payload;
  }

  async send(tx: WireTransaction): Promise<unknown> {
    await this.authenticate();
    return this.#request("POST", AGENT_ROUTES.send, { tx }, true);
  }

  async allowanceAndCall(input: AllowanceAndCall): Promise<unknown> {
    await this.authenticate();
    return this.#request("POST", AGENT_ROUTES.allowanceAndCall, input, true);
  }

  async action(actionId: string): Promise<unknown> {
    if (!/^act_[0-9a-f]{24}$/.test(actionId))
      throw new AgentKitError(
        "INVALID_ACTION_ID",
        "Expected an act_ action identifier",
      );
    return this.read(`/v1/agent/actions/${actionId}`);
  }

  async receipt(hash: Hex): Promise<unknown> {
    return this.read(`/v1/agent/tx/${hash}`);
  }
}
