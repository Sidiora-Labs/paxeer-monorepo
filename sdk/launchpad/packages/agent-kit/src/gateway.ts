import { randomBytes } from 'node:crypto';
import { AgentKitError, record, refusal, requireThat } from './errors.js';
import {
  agentDid,
  agentPublicKey,
  signAgentBytes,
  signAgentRequest,
  buildClaim,
  bindingMessage,
  agentRequestDigest,
} from './signing.js';
import { DEPLOYMENT } from './contracts.js';

export interface GatewayConfig {
  gatewayUrl: string;
  privateKey: Uint8Array;
  label?: string;
  token?: string;
  fetch?: typeof fetch;
  now?: () => number;
  chainId?: number;
  requestTimeoutMs?: number;
}

export interface GatewayIdentity {
  did: string;
  wallet: { address: `0x${string}`; chain_id: number } | null;
  chain: { id: number };
}

export interface GatewayTransaction {
  to: `0x${string}`;
  data: `0x${string}`;
  value: string;
  chainId: number;
}

export interface Simulation {
  ok: boolean;
  gas: string | null;
  return_data: `0x${string}` | null;
  error: unknown;
}

export function gatewayOrigin(value: string): string {
  const url = new URL(value);
  requireThat(
    ['https:', 'http:'].includes(url.protocol) &&
      !url.username &&
      !url.password &&
      !url.search &&
      !url.hash &&
      url.pathname === '/',
    'invalid_gateway_origin',
  );
  requireThat(
    url.protocol === 'https:' || ['localhost', '127.0.0.1', '[::1]'].includes(url.hostname),
    'insecure_gateway_origin',
  );
  return url.origin;
}

export class AgentGatewayClient {
  readonly did: string;
  readonly publicKey: string;
  readonly origin: string;
  readonly #seed: Uint8Array;
  readonly #fetch: typeof fetch;
  readonly #now: () => number;
  readonly #chainId: number;
  readonly #timeout: number;
  readonly #actions = new Map<string, { body: Record<string, unknown>; authorized: Set<string> }>();
  #token: string | undefined;
  #tokenExpiry = 0;
  #authentication: Promise<void> | undefined;

  constructor(config: GatewayConfig) {
    this.origin = gatewayOrigin(config.gatewayUrl);
    this.#seed = new Uint8Array(config.privateKey);
    this.did = agentDid(this.#seed, config.label);
    this.publicKey = agentPublicKey(this.#seed).toString('hex');
    this.#fetch = config.fetch ?? globalThis.fetch;
    this.#now = config.now ?? Date.now;
    this.#chainId = config.chainId ?? DEPLOYMENT.chainId;
    this.#timeout = config.requestTimeoutMs ?? 15000;
    requireThat(
      Number.isInteger(this.#timeout) && this.#timeout >= 1000 && this.#timeout <= 120000,
      'invalid_request_timeout',
    );
    this.#token = config.token;
    this.#tokenExpiry = config.token ? Infinity : 0;
  }

  async #request(
    method: 'GET' | 'POST',
    path: string,
    body?: unknown,
    signed = false,
    authenticated = true,
    authorization?: string,
    pendingAction = false,
  ): Promise<unknown> {
    requireThat(path.startsWith('/v1/agent/') && !path.includes('#'), 'invalid_gateway_path');
    if (authenticated && !signed) await this.authenticate();
    const serialized = body === undefined ? undefined : JSON.stringify(body);
    const headers: Record<string, string> = { accept: 'application/json' };
    if (serialized !== undefined) headers['content-type'] = 'application/json';
    if (authenticated && !signed) headers.authorization = `Bearer ${this.#token}`;
    if (signed)
      Object.assign(
        headers,
        signAgentRequest({
          privateKey: this.#seed,
          keyId: this.did,
          method,
          url: path,
          ...(serialized === undefined ? {} : { body: serialized }),
          nowSeconds: Math.floor(this.#now() / 1000),
        }),
      );
    if (authorization) headers['x-agent-attestor-authorization'] = authorization;
    const fetcher = this.#fetch;
    let response: Response;
    try {
      response = await fetcher(this.origin + path, {
        method,
        headers,
        redirect: 'error',
        signal: AbortSignal.timeout(this.#timeout),
        ...(serialized === undefined ? {} : { body: serialized }),
      });
    } catch {
      throw new AgentKitError('gateway_unavailable', 'The wallet gateway could not be reached.');
    }
    const text = await response.text();
    requireThat(Buffer.byteLength(text) <= 1048576, 'gateway_response_too_large');
    let payload: unknown;
    try {
      payload = JSON.parse(text);
    } catch {
      throw new AgentKitError(
        'invalid_gateway_response',
        'Expected a JSON gateway response.',
        response.status,
      );
    }
    const result = record(payload);
    if (!response.ok || result.ok === false || (result.error && !pendingAction))
      throw refusal(response.status, payload);
    return payload;
  }

  #authorization(request: string, path: string, body: unknown): string {
    requireThat(Buffer.byteLength(request) <= 65536, 'invalid_signing_request');
    const wire = record(JSON.parse(request));
    const origin = record(wire.origin);
    requireThat(
      origin.did === this.did &&
        origin.method === `POST ${path}` &&
        origin.body === Buffer.from(JSON.stringify(body)).toString('base64') &&
        typeof wire.key_id === 'string',
      'signing_request_mismatch',
    );
    const nonce = randomBytes(16);
    const expiry = Math.floor(this.#now() / 1000) + 60;
    return JSON.stringify({
      body: request,
      nonce: nonce.toString('hex'),
      expiry,
      signature: signAgentBytes(
        this.#seed,
        agentRequestDigest({
          method: '/v1/sign',
          keyId: wire.key_id,
          nonce,
          expiry: BigInt(expiry),
          body: request,
        }),
      ),
    });
  }

  async #signed(path: string, body: unknown): Promise<unknown> {
    let authorization: string | undefined;
    for (let leg = 0; ; leg++) {
      try {
        return await this.#request('POST', path, body, true, true, authorization);
      } catch (error) {
        if (
          !(error instanceof AgentKitError) ||
          error.status !== 409 ||
          error.code !== 'agent_reauthorization_required' ||
          leg >= 2
        )
          throw error;
        const details = record(error.details);
        requireThat(typeof details.signing_request === 'string', 'invalid_signing_request');
        authorization = this.#authorization(details.signing_request, path, body);
      }
    }
  }

  async authenticate(): Promise<void> {
    if (this.#token && this.#now() < this.#tokenExpiry) return;
    if (this.#authentication) return this.#authentication;
    this.#authentication = this.#authenticate();
    try {
      await this.#authentication;
    } finally {
      this.#authentication = undefined;
    }
  }

  async #authenticate(): Promise<void> {
    const challenge = record(
      await this.#request('POST', '/v1/agent/auth/challenge', { did: this.did }, false, false),
    );
    requireThat(
      typeof challenge.nonce === 'string' &&
        challenge.message === `paxeer-agent-auth:${this.did}:${challenge.nonce}`,
      'invalid_agent_challenge',
    );
    const result = record(
      await this.#request(
        'POST',
        '/v1/agent/auth/verify',
        {
          did: this.did,
          public_key: this.publicKey,
          nonce: challenge.nonce,
          signature: signAgentBytes(this.#seed, Buffer.from(challenge.message)),
        },
        false,
        false,
      ),
    );
    requireThat(
      typeof result.token === 'string' &&
        result.token.length > 0 &&
        typeof result.expires_in === 'number' &&
        result.expires_in > 0,
      'invalid_agent_token',
    );
    this.#token = result.token;
    this.#tokenExpiry = this.#now() + result.expires_in * 1000 - 5000;
  }

  async identity(): Promise<GatewayIdentity> {
    const result = record(await this.#request('GET', '/v1/agent/me'));
    const chain = record(result.chain);
    requireThat(
      result.did === this.did && Number.isSafeInteger(chain.id),
      'invalid_agent_identity',
    );
    if (result.wallet !== null) {
      const wallet = record(result.wallet);
      requireThat(
        typeof wallet.address === 'string' &&
          /^0x[0-9a-fA-F]{40}$/.test(wallet.address) &&
          wallet.chain_id === chain.id,
        'invalid_agent_wallet',
      );
    }
    return result as unknown as GatewayIdentity;
  }

  balances(tokens: readonly string[] = []): Promise<unknown> {
    requireThat(
      tokens.length <= 100 && tokens.every((token) => /^0x[0-9a-fA-F]{40}$/.test(token)),
      'invalid_tokens',
    );
    return this.#request(
      'GET',
      '/v1/agent/balances' + (tokens.length ? `?tokens=${tokens.join(',')}` : ''),
    );
  }

  async allowance(token: string, spender: string): Promise<bigint> {
    requireThat(
      [token, spender].every((value) => /^0x[0-9a-fA-F]{40}$/.test(value)),
      'invalid_address',
    );
    const result = record(
      await this.#request('GET', `/v1/agent/allowance?token=${token}&spender=${spender}`),
    );
    requireThat(
      typeof result.allowance === 'string' && /^\d{1,78}$/.test(result.allowance),
      'invalid_allowance',
    );
    return BigInt(result.allowance);
  }

  async simulate(tx: GatewayTransaction): Promise<Simulation> {
    const result = record(await this.#request('POST', '/v1/agent/simulate', { tx }));
    requireThat(
      result.ok === true && (result.return_data === null || typeof result.return_data === 'string'),
      'invalid_simulation',
    );
    return result as unknown as Simulation;
  }

  send(tx: GatewayTransaction): Promise<unknown> {
    return this.#signed('/v1/agent/send', { tx });
  }

  async allowanceAndCall(body: {
    token: string;
    amount: string;
    spender: string;
    contract: string;
    method: string;
    args: string[];
    idempotency_key: string;
  }): Promise<unknown> {
    const result = record(await this.#signed('/v1/agent/actions/allowance-and-call', body));
    requireThat(typeof result.action_id === 'string', 'invalid_action_id');
    requireThat(this.#actions.size < 10000, 'action_capacity_reached');
    this.#actions.set(result.action_id, {
      body: { ...body, args: [...body.args] },
      authorized: new Set(),
    });
    return result;
  }

  async action(actionId: string): Promise<unknown> {
    requireThat(/^act_[0-9a-f]{24}$/.test(actionId), 'invalid_action_id');
    const result = record(
      await this.#request(
        'GET',
        `/v1/agent/actions/${actionId}`,
        undefined,
        false,
        true,
        undefined,
        true,
      ),
    );
    if (result.error && result.terminal !== true) {
      const error = record(result.error);
      const saved = this.#actions.get(actionId);
      if (error.code === 'AGENT_REAUTHORIZATION_REQUIRED' && saved) {
        const cause = record(error.cause);
        requireThat(typeof cause.signing_request === 'string', 'invalid_signing_request');
        const request = cause.signing_request;
        if (!saved.authorized.has(request)) {
          const path = '/v1/agent/actions/allowance-and-call';
          const authorization = this.#authorization(request, path, saved.body);
          const resumed = record(
            await this.#request('POST', path, saved.body, true, true, authorization, true),
          );
          requireThat(resumed.action_id === actionId, 'action_identity_mismatch');
          saved.authorized.add(request);
          return resumed;
        }
      } else {
        const retry = record(error.retry);
        if (!['POLL_ACTION', 'RETRY_SAME_ACTION'].includes(String(retry.strategy)))
          throw refusal(200, result);
      }
    }
    if (result.terminal === true) this.#actions.delete(actionId);
    return result;
  }

  async provision(bindSignature?: string, stage?: 'wallet' | 'identity'): Promise<unknown> {
    if (bindSignature !== undefined)
      requireThat(/^(0x)?[0-9a-fA-F]{128}$/.test(bindSignature), 'invalid_bind_signature');
    const result = record(
      await this.#signed('/v1/agent/provision', {
        ...(bindSignature === undefined ? {} : { bind_signature: bindSignature }),
        ...(stage === undefined ? {} : { stage }),
      }),
    );
    const provision = record(result.provision);
    if (bindSignature === undefined && provision.awaiting === 'agent_signature') {
      const wallet = record(result.wallet);
      requireThat(
        wallet.chain_id === this.#chainId &&
          typeof wallet.address === 'string' &&
          wallet.address === provision.address &&
          typeof provision.bindMessage === 'string' &&
          /^0x[0-9a-fA-F]+$/.test(provision.bindMessage),
        'invalid_binding',
      );
      const message = Buffer.from(provision.bindMessage.slice(2), 'hex');
      requireThat(
        message.length === bindingMessage(this.#chainId, wallet.address, 0n).length,
        'invalid_binding',
      );
      const expected = bindingMessage(
        this.#chainId,
        wallet.address,
        message.readBigUInt64BE(message.length - 8),
      );
      requireThat(message.equals(expected), 'invalid_binding');
      return this.#signed('/v1/agent/provision', {
        bind_signature: signAgentBytes(this.#seed, message),
        ...(stage === undefined ? {} : { stage }),
      });
    }
    return result;
  }

  claim(ownerUserId: string): { did: string; expires: string; signature: string } {
    return {
      did: this.did,
      ...buildClaim(this.#seed, this.did, ownerUserId, BigInt(Math.floor(this.#now() / 1000) + 60)),
    };
  }
}
