import { AgentKitError, record, refusal, requireThat } from './errors.js';
import {
  agentDid,
  agentPublicKey,
  signAgentBytes,
  signAgentRequest,
  buildClaim,
  bindingMessage,
} from './signing.js';
import { DEPLOYMENT } from './contracts.js';

export interface GatewayConfig {
  gatewayUrl: string;
  privateKey: Uint8Array;
  label?: string;
  token?: string;
  fetch?: typeof fetch;
  now?: () => number;
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
    this.#token = config.token;
    this.#tokenExpiry = config.token ? Infinity : 0;
  }

  async #request(
    method: 'GET' | 'POST',
    path: string,
    body?: unknown,
    signed = false,
    authenticated = true,
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
    const fetcher = this.#fetch;
    let response: Response;
    try {
      response = await fetcher(this.origin + path, {
        method,
        headers,
        redirect: 'error',
        signal: AbortSignal.timeout(15000),
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
    if (!response.ok || result.ok === false || result.error)
      throw refusal(response.status, payload);
    return payload;
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
    return this.#request('POST', '/v1/agent/send', { tx }, true);
  }

  allowanceAndCall(body: {
    token: string;
    amount: string;
    spender: string;
    contract: string;
    method: string;
    args: string[];
    idempotency_key: string;
  }): Promise<unknown> {
    return this.#request('POST', '/v1/agent/actions/allowance-and-call', body, true);
  }

  action(actionId: string): Promise<unknown> {
    requireThat(/^act_[0-9a-f]{24}$/.test(actionId), 'invalid_action_id');
    return this.#request('GET', `/v1/agent/actions/${actionId}`);
  }

  async provision(bindSignature?: string): Promise<unknown> {
    if (bindSignature !== undefined)
      requireThat(/^(0x)?[0-9a-fA-F]{128}$/.test(bindSignature), 'invalid_bind_signature');
    const result = record(
      await this.#request(
        'POST',
        '/v1/agent/provision',
        bindSignature === undefined ? {} : { bind_signature: bindSignature },
        true,
      ),
    );
    const provision = record(result.provision);
    if (bindSignature === undefined && provision.awaiting === 'agent_signature') {
      const wallet = record(result.wallet);
      requireThat(
        wallet.chain_id === DEPLOYMENT.chainId &&
          typeof wallet.address === 'string' &&
          wallet.address === provision.address &&
          typeof provision.bindMessage === 'string' &&
          /^0x[0-9a-fA-F]+$/.test(provision.bindMessage),
        'invalid_binding',
      );
      const message = Buffer.from(provision.bindMessage.slice(2), 'hex');
      requireThat(
        message.length === bindingMessage(DEPLOYMENT.chainId, wallet.address, 0n).length,
        'invalid_binding',
      );
      const expected = bindingMessage(
        DEPLOYMENT.chainId,
        wallet.address,
        message.readBigUInt64BE(message.length - 8),
      );
      requireThat(message.equals(expected), 'invalid_binding');
      return this.#request(
        'POST',
        '/v1/agent/provision',
        { bind_signature: signAgentBytes(this.#seed, message) },
        true,
      );
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
