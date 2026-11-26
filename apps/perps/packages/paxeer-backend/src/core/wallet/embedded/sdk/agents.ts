import { PaxeerWalletError } from "./types";

export type AgentMode = "read_only" | "trade_only" | "full";
export interface OwnedAgent {
  did: string;
  label: string;
  is_frozen: boolean;
  mode: AgentMode;
  address: `0x${string}` | null;
  created_at: string;
  last_seen_at: string | null;
}
export interface AgentPolicy {
  mode: AgentMode;
  max_tx_value_wei: string;
  max_daily_value_wei: string;
  max_approve_wei: string;
  rate_limit_per_min: number;
  allow_native_transfer: boolean;
  withdrawal_allowlist_only: boolean;
  daily_reset_utc_hour: number;
}
export interface AgentRuleInput {
  effect: "allow" | "deny";
  subject: "contract" | "selector" | "token" | "address" | "withdrawal";
  value: string;
  max_value_wei?: string | null;
  note?: string | null;
}
export interface AgentRule extends AgentRuleInput {
  id: string;
}
export interface AgentBudgetInput {
  target_contract: `0x${string}` | null;
  token: `0x${string}` | null;
  cap_wei: string;
  expires_in_seconds: number;
}
export interface AgentBudget {
  id: string;
  target_contract: string | null;
  token: string | null;
  cap_wei: string;
  spent_wei: string;
  remaining_wei: string;
  expires_at: string;
}
export interface AgentDetail {
  did: string;
  label: string;
  owner_user_id: string;
  is_frozen: boolean;
  wallet: {
    id: string;
    address: `0x${string}`;
    native_wei: string | null;
  } | null;
  policy: AgentPolicy;
  rules: AgentRule[];
  budgets: AgentBudget[];
}
export interface AgentActivity {
  kind: string;
  to_address: string | null;
  value_wei: string;
  tx_hash: `0x${string}` | null;
  created_at: string;
}
export interface AgentClaim {
  expires: string;
  signature: string;
}
export interface AgentClaimRequest {
  did: string;
  owner_user_id: string;
  expires: string;
  digest_hex: string;
}
export interface AgentTransfer {
  token: `0x${string}` | null;
  amount: string;
}
export interface AgentTransferResult {
  tx_hash: `0x${string}`;
  from: string;
  to: string;
}
export interface AgentKeyDisclosure {
  did: string;
  public_key: string;
  private_key: string;
}
export interface AgentProvision {
  wallet: {
    id: string;
    address: `0x${string}`;
    chain_id: number;
    kind: "agent";
  };
  provision: {
    state:
      | "new"
      | "evm_key"
      | "identity"
      | "bind_signed"
      | "funded"
      | "bind_sent"
      | "active"
      | "refused";
    awaiting: "owner" | "agent_signature" | null;
    bindMessage: `0x${string}` | null;
  };
}

type Method = "GET" | "POST" | "PUT" | "DELETE";
type OwnerSession = { access_token: string; user: { id: string } };
const encoder = new TextEncoder();
const didPattern = /^did:matrix:[A-Za-z0-9_-]{1,128}:[0-9a-f]{16}$/;
const maxUint256 = (1n << 256n) - 1n;

export function agentHex(bytes: Uint8Array): string {
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}
function unhex(hex: string): Uint8Array {
  const value = hex.replace(/^0x/, "");
  if (!/^(?:[0-9a-fA-F]{2})+$/.test(value))
    throw new Error("Invalid agent message hex");
  return Uint8Array.from(value.match(/../g)!, (byte) => parseInt(byte, 16));
}
function concat(...parts: Uint8Array[]): Uint8Array {
  const bytes = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let offset = 0;
  for (const part of parts) {
    bytes.set(part, offset);
    offset += part.length;
  }
  return bytes;
}
function lengthPrefixed(value: string): Uint8Array {
  const bytes = encoder.encode(value);
  const length = new Uint8Array(4);
  new DataView(length.buffer).setUint32(0, bytes.length);
  return concat(length, bytes);
}
function u64(value: bigint): Uint8Array {
  if (value < 0n || value > (1n << 64n) - 1n)
    throw new Error("Agent expiry must fit uint64");
  const bytes = new Uint8Array(8);
  new DataView(bytes.buffer).setBigUint64(0, value);
  return bytes;
}
function subtle(): SubtleCrypto {
  if (!globalThis.crypto?.subtle)
    throw new Error(
      "This browser does not support Web Crypto Ed25519. Use a browser with Ed25519 support or claim an agent that holds its own key.",
    );
  return globalThis.crypto.subtle;
}
export async function browserAgentKeySupported(): Promise<boolean> {
  try {
    await subtle().generateKey("Ed25519", false, ["sign", "verify"]);
    return true;
  } catch {
    return false;
  }
}
export async function browserAgentClaimDigest(
  did: string,
  ownerUserId: string,
  expiry: bigint,
): Promise<Uint8Array> {
  return new Uint8Array(
    await subtle().digest(
      "SHA-256",
      concat(
        encoder.encode("PXW:AGENT-CLAIM:v1"),
        lengthPrefixed(did),
        lengthPrefixed(ownerUserId),
        u64(expiry),
      ),
    ),
  );
}
export async function browserAgentRequestDigest(input: {
  method: string;
  keyId: string;
  nonce: Uint8Array;
  expiry: bigint;
  body: string;
}): Promise<Uint8Array> {
  if (input.nonce.length !== 16)
    throw new Error("Agent request nonce must be 16 bytes");
  const bodyHash = new Uint8Array(
    await subtle().digest("SHA-256", encoder.encode(input.body)),
  );
  return new Uint8Array(
    await subtle().digest(
      "SHA-256",
      concat(
        encoder.encode("PXW:AGENT-REQUEST:v1"),
        lengthPrefixed(input.method),
        lengthPrefixed(input.keyId),
        input.nonce,
        u64(input.expiry),
        bodyHash,
      ),
    ),
  );
}
export async function prepareAgentClaim(
  did: string,
  ownerUserId: string,
): Promise<AgentClaimRequest> {
  if (!didPattern.test(did)) throw new Error("Enter a did:matrix agent DID");
  if (!ownerUserId) throw new Error("Sign in to the embedded wallet first");
  const expires = String(Math.floor(Date.now() / 1000) + 60);
  return {
    did,
    owner_user_id: ownerUserId,
    expires,
    digest_hex: agentHex(
      await browserAgentClaimDigest(did, ownerUserId, BigInt(expires)),
    ),
  };
}
export function parseAgentClaim(
  text: string,
  request: AgentClaimRequest,
): AgentClaim {
  const value: unknown = JSON.parse(text);
  if (
    !value ||
    typeof value !== "object" ||
    Array.isArray(value) ||
    Object.keys(value).sort().join(",") !== "expires,signature" ||
    !("expires" in value) ||
    value.expires !== request.expires ||
    !("signature" in value) ||
    typeof value.signature !== "string" ||
    !/^[0-9a-fA-F]{128}$/.test(value.signature)
  ) {
    throw new Error(
      "Paste the claim JSON with exactly expires and a 128-character hex signature for this request",
    );
  }
  if (BigInt(request.expires) <= BigInt(Math.floor(Date.now() / 1000)))
    throw new Error("Claim expired. Prepare a new claim request.");
  return { expires: request.expires, signature: value.signature };
}
export function checkedAgentAmount(
  input: string,
  decimals: number,
  balance: bigint,
): string {
  if (
    !Number.isInteger(decimals) ||
    decimals < 0 ||
    decimals > 18 ||
    input.length > 80 ||
    !/^\d+(\.\d+)?$/.test(input)
  )
    throw new Error("Enter a positive decimal amount");
  const [whole, fraction = ""] = input.split(".");
  if (fraction.length > decimals)
    throw new Error(`Use at most ${decimals} decimal places`);
  const amount =
    BigInt(whole) * 10n ** BigInt(decimals) +
    BigInt(fraction.padEnd(decimals, "0") || "0");
  if (amount <= 0n || amount > maxUint256)
    throw new Error("Amount must be positive and fit uint256");
  if (amount > balance)
    throw new Error("Amount exceeds the available wallet balance");
  return amount.toString();
}

export class AgentOwnerClient {
  private readonly base: string;
  private readonly fetchImpl: typeof fetch;
  constructor(
    private readonly config: {
      apiUrl: string;
      getSession: () => Promise<OwnerSession | null>;
      fetch?: typeof fetch;
    },
  ) {
    if (!config.apiUrl.trim())
      throw new Error("Agent gateway is not configured");
    this.base = config.apiUrl.replace(/\/$/, "");
    this.fetchImpl = config.fetch ?? globalThis.fetch.bind(globalThis);
  }
  private path(did: string, suffix = ""): string {
    if (!didPattern.test(did)) throw new Error("Invalid agent DID");
    return `/v1/agents/${encodeURIComponent(did)}${suffix}`;
  }
  private async request<T>(
    method: Method,
    path: string,
    body?: unknown,
    headers?: Record<string, string>,
  ): Promise<T> {
    const response = await this.fetchImpl(`${this.base}${path}`, {
      method,
      headers: {
        ...(body === undefined ? {} : { "Content-Type": "application/json" }),
        ...headers,
      },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: AbortSignal.timeout(60_000),
    });
    const payload: unknown = await response.json();
    if (!response.ok) {
      const record =
        payload && typeof payload === "object"
          ? (payload as Record<string, unknown>)
          : {};
      throw new PaxeerWalletError(
        typeof record.message === "string"
          ? record.message
          : `Agent request failed (${response.status})`,
        typeof record.error === "string"
          ? record.error
          : `HTTP_${response.status}`,
        response.status,
        payload,
      );
    }
    return payload as T;
  }
  private async owner<T>(
    method: Method,
    path: string,
    body?: unknown,
  ): Promise<T> {
    const session = await this.config.getSession();
    if (!session?.access_token)
      throw new PaxeerWalletError(
        "Sign in to the embedded wallet first",
        "NO_SESSION",
        401,
      );
    return this.request(method, path, body, {
      Authorization: `Bearer ${session.access_token}`,
    });
  }
  list(): Promise<{ agents: OwnedAgent[] }> {
    return this.owner("GET", "/v1/agents");
  }
  detail(did: string): Promise<AgentDetail> {
    return this.owner("GET", this.path(did));
  }
  claim(
    did: string,
    claim: AgentClaim,
  ): Promise<{ did: string; owner_user_id: string }> {
    return this.owner("POST", this.path(did, "/claim"), {
      expires: claim.expires,
      signature: claim.signature,
    });
  }
  policy(
    did: string,
    policy: AgentPolicy,
  ): Promise<{ did: string; policy: AgentPolicy }> {
    return this.owner("PUT", this.path(did, "/policy"), policy);
  }
  addRule(did: string, rule: AgentRuleInput): Promise<{ rule: AgentRule }> {
    return this.owner("POST", this.path(did, "/rules"), rule);
  }
  deleteRule(did: string, id: string): Promise<{ deleted: boolean }> {
    if (!/^\d+$/.test(id)) throw new Error("Invalid rule id");
    return this.owner("DELETE", this.path(did, `/rules/${id}`));
  }
  addBudget(
    did: string,
    budget: AgentBudgetInput,
  ): Promise<{
    budget: Pick<
      AgentBudget,
      "id" | "cap_wei" | "expires_at" | "target_contract" | "token"
    >;
  }> {
    return this.owner("POST", this.path(did, "/budgets"), budget);
  }
  deleteBudget(did: string, id: string): Promise<{ deactivated: boolean }> {
    if (!/^\d+$/.test(id)) throw new Error("Invalid budget id");
    return this.owner("DELETE", this.path(did, `/budgets/${id}`));
  }
  freeze(
    did: string,
    frozen: boolean,
  ): Promise<{ did: string; is_frozen: boolean }> {
    return this.owner("POST", this.path(did, frozen ? "/freeze" : "/unfreeze"));
  }
  fund(did: string, transfer: AgentTransfer): Promise<AgentTransferResult> {
    return this.owner("POST", this.path(did, "/fund"), transfer);
  }
  sweep(did: string, transfer: AgentTransfer): Promise<AgentTransferResult> {
    return this.owner("POST", this.path(did, "/sweep"), transfer);
  }
  activity(did: string): Promise<{ did: string; events: AgentActivity[] }> {
    return this.owner("GET", this.path(did, "/activity"));
  }

  async create(
    label: string,
    ownerUserId: string,
    disclose: (key: AgentKeyDisclosure) => void,
  ): Promise<{ did: string; address: `0x${string}` }> {
    if (!/^[A-Za-z0-9_-]{1,128}$/.test(label))
      throw new Error(
        "Use 1–128 letters, numbers, underscores or hyphens for the agent name",
      );
    const session = await this.config.getSession();
    if (!session || session.user.id !== ownerUserId)
      throw new Error("The embedded wallet session changed");
    let generated: CryptoKeyPair;
    try {
      generated = (await subtle().generateKey("Ed25519", true, [
        "sign",
        "verify",
      ])) as CryptoKeyPair;
    } catch {
      throw new Error(
        "This browser does not support Web Crypto Ed25519. Claim an agent that holds its own key or use a browser with Ed25519 support.",
      );
    }
    const publicKey = agentHex(
      new Uint8Array(await subtle().exportKey("raw", generated.publicKey)),
    );
    const did = `did:matrix:${label}:${publicKey.slice(0, 16)}`;
    const exported = new Uint8Array(
      await subtle().exportKey("pkcs8", generated.privateKey),
    );
    const seed = exported.slice(-32);
    const privateKey = await subtle().importKey(
      "pkcs8",
      exported,
      "Ed25519",
      false,
      ["sign"],
    );
    exported.fill(0);
    try {
      disclose({ did, public_key: publicKey, private_key: agentHex(seed) });
    } finally {
      seed.fill(0);
    }
    const sign = async (bytes: Uint8Array) =>
      agentHex(
        new Uint8Array(await subtle().sign("Ed25519", privateKey, bytes)),
      );
    const challenge = await this.request<{
      did: string;
      nonce: string;
      message: string;
    }>("POST", "/v1/agent/auth/challenge", { did });
    if (
      challenge.did !== did ||
      challenge.message !== `paxeer-agent-auth:${did}:${challenge.nonce}`
    )
      throw new Error("Agent challenge does not match this identity");
    await this.request("POST", "/v1/agent/auth/verify", {
      did,
      public_key: publicKey,
      nonce: challenge.nonce,
      signature: await sign(encoder.encode(challenge.message)),
    });
    const expires = String(Math.floor(Date.now() / 1000) + 60);
    await this.claim(did, {
      expires,
      signature: await sign(
        await browserAgentClaimDigest(did, ownerUserId, BigInt(expires)),
      ),
    });
    try {
      await this.freeze(did, false);
      let body: { bind_signature?: string } = {};
      for (let step = 0; step < 4; step++) {
        const nonce = globalThis.crypto.getRandomValues(new Uint8Array(16));
        const expiry = BigInt(Math.floor(Date.now() / 1000) + 60);
        const digest = await browserAgentRequestDigest({
          method: "POST /v1/agent/provision",
          keyId: did,
          nonce,
          expiry,
          body: JSON.stringify(body),
        });
        const result = await this.request<AgentProvision>(
          "POST",
          "/v1/agent/provision",
          body,
          {
            "x-agent-key": did,
            "x-agent-nonce": agentHex(nonce),
            "x-agent-expires": String(expiry),
            "x-agent-signature": await sign(digest),
          },
        );
        if (result.provision.state === "active")
          return { did, address: result.wallet.address };
        if (
          result.provision.awaiting === "agent_signature" &&
          result.provision.bindMessage
        )
          body = {
            bind_signature: await sign(unhex(result.provision.bindMessage)),
          };
        else if (result.provision.state !== "bind_sent")
          throw new Error(
            `Agent setup is ${result.provision.state}. Keep the displayed key to continue setup with your agent.`,
          );
      }
      throw new Error(
        "Agent setup is awaiting confirmation. Keep the displayed key to continue setup with your agent.",
      );
    } finally {
      await this.freeze(did, true);
    }
  }
}
